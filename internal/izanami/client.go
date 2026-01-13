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

// Client represents an Izanami HTTP client
type Client struct {
	http             *resty.Client
	config           *Config
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

// NewClient creates a new Izanami client with the given configuration
func NewClient(config *Config) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return newClientInternal(config)
}

// NewClientNoAuth creates a client without authentication validation (for health checks)
func NewClientNoAuth(config *Config) (*Client, error) {
	if config.BaseURL == "" {
		return nil, fmt.Errorf(errmsg.MsgBaseURLRequired)
	}

	return newClientInternal(config)
}

// newClientInternal creates the actual client (shared logic)
func newClientInternal(config *Config) (*Client, error) {
	// Make a defensive copy of the config to prevent external mutations
	configCopy := &Config{
		BaseURL:                     config.BaseURL,
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
		InsecureSkipVerify:          config.InsecureSkipVerify,
	}

	client := resty.New().
		SetBaseURL(configCopy.BaseURL).
		SetTimeout(time.Duration(configCopy.Timeout) * time.Second).
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
	if configCopy.InsecureSkipVerify {
		client.SetTLSClientConfig(&tls.Config{
			InsecureSkipVerify: true,
		})
	}

	izClient := &Client{
		http:             client,
		config:           configCopy,
		beforeRequest:    []func(*resty.Request) error{},
		afterResponse:    []func(*resty.Response) error{},
		structuredLogger: nil,
	}

	if configCopy.Verbose {
		enableSecureDebugMode(client, izClient)
	}

	// Note: Authentication is NOT set at client level
	// Each API method will set appropriate authentication at the request level:
	// - Admin APIs: Use setAdminAuth() to set PAT or JWT
	// - Client APIs: Use setClientAuth() to set client-id/secret headers

	return izClient, nil
}

const maxBodyLogLength = 2048 // Maximum length for logged request/response bodies

// AddBeforeRequestHook adds a middleware function that runs before each request
func (c *Client) AddBeforeRequestHook(hook func(*resty.Request) error) {
	c.beforeRequest = append(c.beforeRequest, hook)
}

// AddAfterResponseHook adds a middleware function that runs after each response
func (c *Client) AddAfterResponseHook(hook func(*resty.Response) error) {
	c.afterResponse = append(c.afterResponse, hook)
}

// SetStructuredLogger sets a structured logging function
// The logger receives: level ("info", "warn", "error"), message, and optional fields
func (c *Client) SetStructuredLogger(logger func(level, message string, fields map[string]interface{})) {
	c.structuredLogger = logger
}

// executeBeforeRequestHooks runs all before-request middleware hooks
func (c *Client) executeBeforeRequestHooks(req *resty.Request) error {
	for _, hook := range c.beforeRequest {
		if err := hook(req); err != nil {
			return err
		}
	}
	return nil
}

// executeAfterResponseHooks runs all after-response middleware hooks
func (c *Client) executeAfterResponseHooks(resp *resty.Response) error {
	for _, hook := range c.afterResponse {
		if err := hook(resp); err != nil {
			return err
		}
	}
	return nil
}

// log writes a log message using structured logger if available, otherwise falls back to stderr
func (c *Client) log(level, message string, fields map[string]interface{}) {
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
func (c *Client) setAdminAuth(req *resty.Request) {
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

// setClientAuth sets authentication for client API requests (client-id/secret)
// Returns error if client credentials are not configured
func (c *Client) setClientAuth(req *resty.Request) error {
	if c.config.ClientID == "" || c.config.ClientSecret == "" {
		return fmt.Errorf("client credentials required (client-id and client-secret)")
	}
	req.SetHeader("Izanami-Client-Id", c.config.ClientID)
	req.SetHeader("Izanami-Client-Secret", c.config.ClientSecret)
	return nil
}

// enableSecureDebugMode enables verbose logging with sensitive data redaction
func enableSecureDebugMode(httpClient *resty.Client, izClient *Client) {
	// Sensitive headers that should be redacted in both requests and responses
	sensitiveHeaders := map[string]bool{
		"cookie":                true,
		"set-cookie":            true,
		"authorization":         true,
		"izanami-client-secret": true,
		"izanami-client-id":     true,
		"x-api-key":             true,
		"authentication":        true,
		"www-authenticate":      true,
	}

	// Log response details (after receiving)
	// We log both request and response here because the request details
	// are only fully available after the request is sent
	httpClient.OnAfterResponse(func(c *resty.Client, resp *resty.Response) error {
		logRequest(resp, sensitiveHeaders, izClient)
		logResponse(resp, sensitiveHeaders, izClient)
		return nil
	})
}

// logRequest logs HTTP request details with sensitive data redaction
func logRequest(resp *resty.Response, sensitiveHeaders map[string]bool, izClient *Client) {
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
		fmt.Fprintf(os.Stderr, "==============================================================================\n")
		fmt.Fprintf(os.Stderr, "~~~ REQUEST ~~~\n")
		// Log full URL including query parameters
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
}

// logResponse logs HTTP response details with sensitive data redaction
func logResponse(resp *resty.Response, sensitiveHeaders map[string]bool, izClient *Client) {
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

// LogSSERequest logs SSE request details when verbose mode is enabled.
// This is needed because SetDoNotParseResponse(true) bypasses OnAfterResponse hooks.
func (c *Client) LogSSERequest(method, path string, queryParams map[string]string, headers map[string]string, body interface{}) {
	if !c.config.Verbose {
		return
	}

	sensitiveHeaders := sensitiveHeadersMap()

	fmt.Fprintf(os.Stderr, "==============================================================================\n")
	fmt.Fprintf(os.Stderr, "~~~ REQUEST (SSE) ~~~\n")

	// Build URL with query params
	url := path
	if len(queryParams) > 0 {
		params := make([]string, 0, len(queryParams))
		for k, v := range queryParams {
			params = append(params, k+"="+v)
		}
		url += "?" + strings.Join(params, "&")
	}

	fmt.Fprintf(os.Stderr, "%s  %s\n", method, url)
	fmt.Fprintf(os.Stderr, "HOST   : %s\n", c.config.BaseURL)
	fmt.Fprintf(os.Stderr, "HEADERS:\n")
	for key, value := range headers {
		keyLower := strings.ToLower(key)
		if sensitiveHeaders[keyLower] {
			fmt.Fprintf(os.Stderr, "\t%s: [REDACTED]\n", key)
		} else {
			fmt.Fprintf(os.Stderr, "\t%s: %s\n", key, value)
		}
	}
	fmt.Fprintf(os.Stderr, "BODY   :\n")
	logBody(os.Stderr, body)
	fmt.Fprintf(os.Stderr, "------------------------------------------------------------------------------\n")
}

// LogSSEResponse logs SSE response status when verbose mode is enabled.
func (c *Client) LogSSEResponse(statusCode int, status string) {
	if !c.config.Verbose {
		return
	}

	fmt.Fprintf(os.Stderr, "~~~ RESPONSE (SSE) ~~~\n")
	fmt.Fprintf(os.Stderr, "STATUS       : %d %s\n", statusCode, status)
	fmt.Fprintf(os.Stderr, "BODY         : [streaming...]\n")
	fmt.Fprintf(os.Stderr, "==============================================================================\n")
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
func (c *Client) handleError(resp *resty.Response) error {
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
func (c *Client) Login(ctx context.Context, username, password string) (string, error) {
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
