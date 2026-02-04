package izanami

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	errmsg "github.com/webskin/izanami-go-cli/internal/errors"
)

const (
	apiAdminTenants = "/api/admin/tenants/"
)

// AdminClient represents an Izanami HTTP client for admin API operations (/api/admin/*)
// Use this for administrative operations like managing features, projects, tenants, etc.
// For client operations (feature checks, events), use FeatureCheckClient instead.
type AdminClient struct {
	http             *resty.Client
	config           *ResolvedConfig
	beforeRequest    []func(*resty.Request) error
	afterResponse    []func(*resty.Response) error
	structuredLogger func(level, message string, fields map[string]interface{})
}

// APIError represents a structured API error with status code and message
// This allows callers to inspect the status code without parsing error strings
type APIError struct {
	StatusCode int    // HTTP status code
	Message    string // Error message from the API
	RawBody    string // Raw response body for debugging
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (%d): %s", e.StatusCode, e.Message)
}

// NewAdminClient creates a new Izanami admin client with the given configuration.
// This validates that admin authentication (PAT or JWT) is configured.
// For client operations (feature checks, events), use NewFeatureCheckClient instead.
func NewAdminClient(config *ResolvedConfig) (*AdminClient, error) {
	if err := config.ValidateAdminAuth(); err != nil {
		return nil, err
	}

	return newAdminClientInternal(config)
}

// NewAdminClientNoAuth creates an admin client without authentication validation.
// Use this for operations that don't require authentication (e.g., health checks).
func NewAdminClientNoAuth(config *ResolvedConfig) (*AdminClient, error) {
	if config.LeaderURL == "" {
		return nil, fmt.Errorf(errmsg.MsgLeaderURLRequired)
	}

	return newAdminClientInternal(config)
}

// copyConfig makes a defensive copy of the config to prevent external mutations
func copyConfig(config *ResolvedConfig) *ResolvedConfig {
	cp := &ResolvedConfig{
		LeaderURL:                   config.LeaderURL,
		ClientID:                    config.ClientID,
		ClientSecret:                config.ClientSecret,
		PersonalAccessTokenUsername: config.PersonalAccessTokenUsername,
		JwtToken:                    config.JwtToken,
		PersonalAccessToken:         config.PersonalAccessToken,
		Tenant:                      config.Tenant,
		Project:                     config.Project,
		Context:                     config.Context,
		Timeout:                     config.Timeout,
		Verbose:                     config.Verbose,
		OutputFormat:                config.OutputFormat,
		Color:                       config.Color,
		Username:                    config.Username,
		AuthMethod:                  config.AuthMethod,
		InsecureSkipVerify:          config.InsecureSkipVerify,
		WorkerURL:                   config.WorkerURL,
		WorkerName:                  config.WorkerName,
		WorkerSource:                config.WorkerSource,
	}
	if config.ClientKeys != nil {
		cp.ClientKeys = make(map[string]TenantClientKeysConfig, len(config.ClientKeys))
		for k, v := range config.ClientKeys {
			cp.ClientKeys[k] = v
		}
	}
	if config.WorkerClientKeys != nil {
		cp.WorkerClientKeys = make(map[string]TenantClientKeysConfig, len(config.WorkerClientKeys))
		for k, v := range config.WorkerClientKeys {
			cp.WorkerClientKeys[k] = v
		}
	}
	return cp
}

// newHTTPClient creates a configured resty HTTP client.
// This is shared between AdminClient and FeatureCheckClient.
func newHTTPClient(baseURL string, timeout int, insecureSkipVerify bool) *resty.Client {
	client := resty.New().
		SetBaseURL(baseURL).
		SetTimeout(time.Duration(timeout) * time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(5 * time.Second).
		AddRetryCondition(func(r *resty.Response, err error) bool {
			// IMPORTANT: Only retry on network errors or 5xx for idempotent methods (GET, HEAD)
			// This prevents duplicate operations from POST/PUT/DELETE retries.
			// Non-idempotent methods (POST, PUT, DELETE, PATCH) are NOT retried to avoid
			// creating duplicate resources or applying the same modification multiple times.
			if r == nil {
				// Network error, safe to retry
				return err != nil
			}
			method := r.Request.Method
			isIdempotent := method == http.MethodGet || method == http.MethodHead
			return isIdempotent && (err != nil || r.StatusCode() >= 500)
		})

	// Configure TLS to skip certificate verification if requested
	if insecureSkipVerify {
		client.SetTLSClientConfig(&tls.Config{
			InsecureSkipVerify: true,
		})
	}

	return client
}

// newAdminClientInternal creates the actual admin client (shared logic)
func newAdminClientInternal(config *ResolvedConfig) (*AdminClient, error) {
	configCopy := copyConfig(config)

	httpClient := newHTTPClient(configCopy.LeaderURL, configCopy.Timeout, configCopy.InsecureSkipVerify)

	izClient := &AdminClient{
		http:             httpClient,
		config:           configCopy,
		beforeRequest:    []func(*resty.Request) error{},
		afterResponse:    []func(*resty.Response) error{},
		structuredLogger: nil,
	}

	if configCopy.Verbose {
		enableAdminSecureDebugMode(httpClient, izClient)
	}

	return izClient, nil
}

const maxBodyLogLength = 2048 // Maximum length for logged request/response bodies

// AddBeforeRequestHook adds a middleware function that runs before each request
func (c *AdminClient) AddBeforeRequestHook(hook func(*resty.Request) error) {
	c.beforeRequest = append(c.beforeRequest, hook)
}

// AddAfterResponseHook adds a middleware function that runs after each response
func (c *AdminClient) AddAfterResponseHook(hook func(*resty.Response) error) {
	c.afterResponse = append(c.afterResponse, hook)
}

// SetStructuredLogger sets a structured logging function
// The logger receives: level ("info", "warn", "error"), message, and optional fields
func (c *AdminClient) SetStructuredLogger(logger func(level, message string, fields map[string]interface{})) {
	c.structuredLogger = logger
}

// executeBeforeRequestHooks runs all before-request middleware hooks
func (c *AdminClient) executeBeforeRequestHooks(req *resty.Request) error {
	for _, hook := range c.beforeRequest {
		if err := hook(req); err != nil {
			return err
		}
	}
	return nil
}

// executeAfterResponseHooks runs all after-response middleware hooks
func (c *AdminClient) executeAfterResponseHooks(resp *resty.Response) error {
	for _, hook := range c.afterResponse {
		if err := hook(resp); err != nil {
			return err
		}
	}
	return nil
}

// log writes a log message using structured logger if available, otherwise falls back to stderr
func (c *AdminClient) log(level, message string, fields map[string]interface{}) {
	if c.structuredLogger != nil {
		c.structuredLogger(level, message, fields)
	} else if c.config.Verbose {
		// Fallback to stderr for verbose mode
		fmt.Fprintf(os.Stderr, "[%s] %s", level, message)
		if len(fields) > 0 {
			fmt.Fprintf(os.Stderr, " %v", fields)
		}
		fmt.Fprintf(os.Stderr, "\n")
	}
}

// normalizeContextPath ensures context path has a leading slash if not empty
func normalizeContextPath(contextPath string) string {
	if contextPath != "" && !strings.HasPrefix(contextPath, "/") {
		return "/" + contextPath
	}
	return contextPath
}

// truncateString truncates a string to maxLength and adds a truncation notice
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return fmt.Sprintf("%s... [TRUNCATED: %d more bytes]", s[:maxLength], len(s)-maxLength)
}

// setAdminAuth sets authentication for admin API requests (PAT or JWT)
func (c *AdminClient) setAdminAuth(req *resty.Request) {
	// Priority: PAT token > JWT cookie
	if c.config.PersonalAccessToken != "" {
		// Personal Access Token authentication - Uses Basic Auth with username:token
		// Username is required and sent to server for PAT authentication
		req.SetBasicAuth(c.config.PersonalAccessTokenUsername, c.config.PersonalAccessToken)
	} else if c.config.JwtToken != "" {
		// JWT cookie authentication - ONLY sends JWT token cookie
		// Username is NOT sent to server (JWT is self-contained)
		req.SetHeader("Cookie", "token="+c.config.JwtToken)
	}
}

// enableAdminSecureDebugMode enables verbose logging with sensitive data redaction for AdminClient
func enableAdminSecureDebugMode(httpClient *resty.Client, izClient *AdminClient) {
	sensitiveHeaders := sensitiveHeadersMap()

	// Log response details (after receiving)
	// We log both request and response here because the request details
	// are only fully available after the request is sent
	httpClient.OnAfterResponse(func(c *resty.Client, resp *resty.Response) error {
		logAdminRequest(resp, sensitiveHeaders, izClient)
		logAdminResponse(resp, sensitiveHeaders, izClient)
		return nil
	})
}

// logAdminRequest logs HTTP request details with sensitive data redaction for AdminClient
func logAdminRequest(resp *resty.Response, sensitiveHeaders map[string]bool, izClient *AdminClient) {
	req := resp.Request.RawRequest

	// Use structured logging if available
	if izClient != nil && izClient.structuredLogger != nil {
		url := req.URL.Path
		if req.URL.RawQuery != "" {
			url += "?" + req.URL.RawQuery
		}
		izClient.structuredLogger("info", "HTTP Request", map[string]interface{}{
			"method": req.Method,
			"url":    url,
			"host":   req.Host,
		})
	} else {
		logRequestToStderr(resp, sensitiveHeaders)
	}
}

// logAdminResponse logs HTTP response details with sensitive data redaction for AdminClient
func logAdminResponse(resp *resty.Response, sensitiveHeaders map[string]bool, izClient *AdminClient) {
	// Use structured logging if available
	if izClient != nil && izClient.structuredLogger != nil {
		fields := map[string]interface{}{
			"status":   resp.Status(),
			"duration": resp.Time().String(),
		}
		if len(resp.Body()) > 0 {
			fields["body_size"] = len(resp.Body())
		}
		izClient.structuredLogger("info", "HTTP Response", fields)
	} else {
		logResponseToStderr(resp, sensitiveHeaders)
	}
}

// logRequestToStderr writes HTTP request details to stderr with sensitive header redaction.
// Used by both AdminClient and FeatureCheckClient verbose logging.
func logRequestToStderr(resp *resty.Response, sensitiveHeaders map[string]bool) {
	req := resp.Request.RawRequest

	fmt.Fprintf(os.Stderr, "==============================================================================\n")
	fmt.Fprintf(os.Stderr, "~~~ REQUEST ~~~\n")
	url := req.URL.Path
	if req.URL.RawQuery != "" {
		url += "?" + req.URL.RawQuery
	}
	fmt.Fprintf(os.Stderr, "%s  %s  %s\n", req.Method, url, req.Proto)
	fmt.Fprintf(os.Stderr, "HOST   : %s\n", req.Host)
	fmt.Fprintf(os.Stderr, "HEADERS:\n")
	for key, values := range req.Header {
		keyLower := strings.ToLower(key)
		if sensitiveHeaders[keyLower] {
			fmt.Fprintf(os.Stderr, "\t%s: [REDACTED]\n", key)
		} else {
			for _, value := range values {
				fmt.Fprintf(os.Stderr, "\t%s: %s\n", key, value)
			}
		}
	}
	fmt.Fprintf(os.Stderr, "BODY   :\n")
	logBody(os.Stderr, resp.Request.Body)
	fmt.Fprintf(os.Stderr, "------------------------------------------------------------------------------\n")
}

// logResponseToStderr writes HTTP response details to stderr with sensitive header redaction.
// Used by both AdminClient and FeatureCheckClient verbose logging.
func logResponseToStderr(resp *resty.Response, sensitiveHeaders map[string]bool) {
	fmt.Fprintf(os.Stderr, "~~~ RESPONSE ~~~\n")
	fmt.Fprintf(os.Stderr, "STATUS       : %s\n", resp.Status())
	fmt.Fprintf(os.Stderr, "PROTO        : %s\n", resp.Proto())
	fmt.Fprintf(os.Stderr, "RECEIVED AT  : %v\n", time.Now().Format(time.RFC3339Nano))
	fmt.Fprintf(os.Stderr, "TIME DURATION: %v\n", resp.Time())
	fmt.Fprintf(os.Stderr, "HEADERS      :\n")
	for key, values := range resp.Header() {
		keyLower := strings.ToLower(key)
		if sensitiveHeaders[keyLower] {
			fmt.Fprintf(os.Stderr, "\t%s: [REDACTED]\n", key)
		} else {
			for _, value := range values {
				fmt.Fprintf(os.Stderr, "\t%s: %s\n", key, value)
			}
		}
	}
	fmt.Fprintf(os.Stderr, "BODY         :\n")
	if len(resp.Body()) > 0 {
		body := string(resp.Body())
		fmt.Fprintf(os.Stderr, "%s\n", truncateString(body, maxBodyLogLength))
	} else {
		fmt.Fprintf(os.Stderr, "***** NO CONTENT *****\n")
	}
	fmt.Fprintf(os.Stderr, "==============================================================================\n")
}

// sensitiveHeadersMap returns the map of sensitive headers that should be redacted
func sensitiveHeadersMap() map[string]bool {
	return map[string]bool{
		"cookie":                true,
		"set-cookie":            true,
		"authorization":         true,
		"izanami-client-secret": true,
		"izanami-client-id":     true,
		"x-api-key":             true,
		"authentication":        true,
		"www-authenticate":      true,
	}
}

// logBody safely logs a request body with truncation and type handling
func logBody(w io.Writer, body interface{}) {
	if body == nil {
		fmt.Fprintf(w, "***** NO CONTENT *****\n")
		return
	}

	var bodyStr string
	switch v := body.(type) {
	case string:
		bodyStr = v
	case []byte:
		bodyStr = string(v)
	default:
		// For other types, marshal as JSON if possible
		if jsonBytes, err := json.Marshal(v); err == nil {
			bodyStr = string(jsonBytes)
		} else {
			bodyStr = fmt.Sprintf("%v", v)
		}
	}

	fmt.Fprintf(w, "%s\n", truncateString(bodyStr, maxBodyLogLength))
}

// handleError parses error responses from the API and returns a structured APIError
func (c *AdminClient) handleError(resp *resty.Response) error {
	return parseAPIError(resp)
}

// parseAPIError parses error responses and returns a structured APIError.
// This is shared between AdminClient and FeatureCheckClient.
func parseAPIError(resp *resty.Response) error {
	rawBody := string(resp.Body())

	var errResp ErrorResponse
	if err := json.Unmarshal(resp.Body(), &errResp); err == nil && errResp.Message != "" {
		return &APIError{
			StatusCode: resp.StatusCode(),
			Message:    errResp.Message,
			RawBody:    rawBody,
		}
	}

	return &APIError{
		StatusCode: resp.StatusCode(),
		Message:    rawBody,
		RawBody:    rawBody,
	}
}

// buildPath constructs a URL path with properly escaped segments
func buildPath(segments ...string) string {
	var escaped []string
	for _, seg := range segments {
		if seg != "" {
			escaped = append(escaped, url.PathEscape(seg))
		}
	}
	return strings.Join(escaped, "/")
}

// ============================================================================
// AUTHENTICATION
// ============================================================================

// Login performs login with username and password, returning the JWT token
func (c *AdminClient) Login(ctx context.Context, username, password string) (string, error) {
	resp, err := c.http.R().
		SetContext(ctx).
		SetBasicAuth(username, password).
		Post("/api/admin/login")

	if err != nil {
		return "", fmt.Errorf("%s: %w", errmsg.MsgLoginRequestFailed, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf(errmsg.MsgLoginFailed, resp.StatusCode())
	}

	// Extract JWT token from Set-Cookie header
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "token" {
			return cookie.Value, nil
		}
	}

	return "", fmt.Errorf(errmsg.MsgNoJWTTokenInResponse)
}
