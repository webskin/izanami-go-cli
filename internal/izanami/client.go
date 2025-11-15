package izanami

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	apiAdminTenants = "/api/admin/tenants/"
)

// Client represents an Izanami HTTP client
type Client struct {
	http   *resty.Client
	config *Config
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
		return nil, fmt.Errorf("base URL is required")
	}

	return newClientInternal(config)
}

// newClientInternal creates the actual client (shared logic)
func newClientInternal(config *Config) (*Client, error) {
	// Make a defensive copy of the config to prevent external mutations
	configCopy := &Config{
		BaseURL:      config.BaseURL,
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Username:     config.Username,
		Token:        config.Token,
		Tenant:       config.Tenant,
		Project:      config.Project,
		Context:      config.Context,
		Timeout:      config.Timeout,
		Verbose:      config.Verbose,
	}

	client := resty.New().
		SetBaseURL(configCopy.BaseURL).
		SetTimeout(time.Duration(configCopy.Timeout) * time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(5 * time.Second).
		AddRetryCondition(func(r *resty.Response, err error) bool {
			// Only retry on network errors or 5xx for idempotent methods (GET, HEAD)
			// This prevents duplicate operations from POST/PUT/DELETE retries
			if r == nil {
				// Network error, safe to retry
				return err != nil
			}
			method := r.Request.Method
			isIdempotent := method == http.MethodGet || method == http.MethodHead
			return isIdempotent && (err != nil || r.StatusCode() >= 500)
		})

	if configCopy.Verbose {
		enableSecureDebugMode(client)
	}

	// Set authentication
	if configCopy.ClientID != "" && configCopy.ClientSecret != "" {
		// Client API key authentication
		client.SetHeader("Izanami-Client-Id", configCopy.ClientID)
		client.SetHeader("Izanami-Client-Secret", configCopy.ClientSecret)
	} else if configCopy.Username != "" && configCopy.Token != "" {
		// Admin JWT cookie authentication
		// Izanami expects the JWT token in a cookie named "token"
		cookie := &http.Cookie{
			Name:  "token",
			Value: configCopy.Token,
			Path:  "/",
		}
		client.SetCookie(cookie)
	}

	return &Client{
		http:   client,
		config: configCopy,
	}, nil
}

const maxBodyLogLength = 2048 // Maximum length for logged request/response bodies

// enableSecureDebugMode enables verbose logging with sensitive data redaction
func enableSecureDebugMode(client *resty.Client) {
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
	client.OnAfterResponse(func(c *resty.Client, resp *resty.Response) error {
		logRequest(resp, sensitiveHeaders)
		logResponse(resp, sensitiveHeaders)
		return nil
	})
}

// logRequest logs HTTP request details with sensitive data redaction
func logRequest(resp *resty.Response, sensitiveHeaders map[string]bool) {
	req := resp.Request.RawRequest

	fmt.Fprintf(os.Stderr, "==============================================================================\n")
	fmt.Fprintf(os.Stderr, "~~~ REQUEST ~~~\n")
	fmt.Fprintf(os.Stderr, "%s  %s  %s\n", req.Method, req.URL.Path, req.Proto)
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

// logResponse logs HTTP response details with sensitive data redaction
func logResponse(resp *resty.Response, sensitiveHeaders map[string]bool) {
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
		if len(body) > maxBodyLogLength {
			fmt.Fprintf(os.Stderr, "%s... [TRUNCATED: %d more bytes]\n", body[:maxBodyLogLength], len(body)-maxBodyLogLength)
		} else {
			fmt.Fprintf(os.Stderr, "%s\n", body)
		}
	} else {
		fmt.Fprintf(os.Stderr, "***** NO CONTENT *****\n")
	}
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

	if len(bodyStr) > maxBodyLogLength {
		fmt.Fprintf(w, "%s... [TRUNCATED: %d more bytes]\n", bodyStr[:maxBodyLogLength], len(bodyStr)-maxBodyLogLength)
	} else {
		fmt.Fprintf(w, "%s\n", bodyStr)
	}
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
		return "", fmt.Errorf("login request failed: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("login failed (status %d): invalid credentials", resp.StatusCode())
	}

	// Extract JWT token from Set-Cookie header
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "token" {
			return cookie.Value, nil
		}
	}

	return "", fmt.Errorf("no JWT token in login response")
}

// ============================================================================
// FEATURE OPERATIONS
// ============================================================================

// ListFeatures lists all features in a tenant
func (c *Client) ListFeatures(ctx context.Context, tenant string, tag string) ([]Feature, error) {
	path := apiAdminTenants + buildPath(tenant, "features")

	req := c.http.R().SetContext(ctx).SetResult(&[]Feature{})

	// Add tag filter if specified (server-side filtering)
	if tag != "" {
		req.SetQueryParam("tag", tag)
	}

	resp, err := req.Get(path)
	if err != nil {
		return nil, fmt.Errorf("failed to list features: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	features := resp.Result().(*[]Feature)
	return *features, nil
}

// GetFeature retrieves a specific feature
func (c *Client) GetFeature(ctx context.Context, tenant, featureID string) (*FeatureWithOverloads, error) {
	path := apiAdminTenants + buildPath(tenant, "features", featureID)

	var feature FeatureWithOverloads
	resp, err := c.http.R().
		SetContext(ctx).
		SetResult(&feature).
		Get(path)

	if err != nil {
		return nil, fmt.Errorf("failed to get feature: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return &feature, nil
}

// CreateFeature creates a new feature
// The feature parameter accepts either a *Feature or any compatible struct
func (c *Client) CreateFeature(ctx context.Context, tenant, project string, feature interface{}) (*Feature, error) {
	path := apiAdminTenants + buildPath(tenant, "projects", project, "features")

	var result Feature
	resp, err := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(feature).
		SetResult(&result).
		Post(path)

	if err != nil {
		return nil, fmt.Errorf("failed to create feature: %w", err)
	}

	if resp.StatusCode() != http.StatusCreated && resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return &result, nil
}

// UpdateFeature updates an existing feature
// The feature parameter accepts either a *Feature, *FeatureWithOverloads, or any compatible struct
func (c *Client) UpdateFeature(ctx context.Context, tenant, featureID string, feature interface{}, preserveProtectedContexts bool) error {
	path := apiAdminTenants + buildPath(tenant, "features", featureID)

	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(feature)

	if preserveProtectedContexts {
		req.SetQueryParam("preserveProtectedContexts", "true")
	}

	resp, err := req.Put(path)
	if err != nil {
		return fmt.Errorf("failed to update feature: %w", err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}

// DeleteFeature deletes a feature
func (c *Client) DeleteFeature(ctx context.Context, tenant, featureID string) error {
	path := apiAdminTenants + buildPath(tenant, "features", featureID)

	resp, err := c.http.R().
		SetContext(ctx).
		Delete(path)

	if err != nil {
		return fmt.Errorf("failed to delete feature: %w", err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent && resp.StatusCode() != http.StatusNotFound {
		return c.handleError(resp)
	}

	return nil
}

// CheckFeature checks if a feature is active (client API)
func (c *Client) CheckFeature(ctx context.Context, featureID, user, contextPath string) (*FeatureCheckResult, error) {
	path := "/api/v2/features/" + buildPath(featureID)

	req := c.http.R().SetContext(ctx).SetResult(&FeatureCheckResult{})

	if user != "" {
		req.SetQueryParam("user", user)
	}
	if contextPath != "" {
		req.SetQueryParam("context", contextPath)
	}

	resp, err := req.Get(path)
	if err != nil {
		return nil, fmt.Errorf("failed to check feature: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	result := resp.Result().(*FeatureCheckResult)
	return result, nil
}

// ============================================================================
// CONTEXT OPERATIONS
// ============================================================================

// ListContexts lists all contexts for a tenant or project
func (c *Client) ListContexts(ctx context.Context, tenant, project string, all bool) ([]Context, error) {
	var path string
	if project != "" {
		path = apiAdminTenants + buildPath(tenant, "projects", project, "contexts")
	} else {
		path = apiAdminTenants + buildPath(tenant, "contexts")
	}

	req := c.http.R().SetContext(ctx).SetResult(&[]Context{})

	if all {
		req.SetQueryParam("all", "true")
	}

	resp, err := req.Get(path)
	if err != nil {
		return nil, fmt.Errorf("failed to list contexts: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	contexts := resp.Result().(*[]Context)
	return *contexts, nil
}

// CreateContext creates a new context
// The contextData parameter accepts a map or any compatible struct with context fields
func (c *Client) CreateContext(ctx context.Context, tenant, project, name, parentPath string, contextData interface{}) error {
	var path string
	if project != "" {
		if parentPath != "" {
			path = apiAdminTenants + buildPath(tenant, "projects", project, "contexts", parentPath)
		} else {
			path = apiAdminTenants + buildPath(tenant, "projects", project, "contexts")
		}
	} else {
		if parentPath != "" {
			path = apiAdminTenants + buildPath(tenant, "contexts", parentPath)
		} else {
			path = apiAdminTenants + buildPath(tenant, "contexts")
		}
	}

	resp, err := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(contextData).
		Post(path)

	if err != nil {
		return fmt.Errorf("failed to create context: %w", err)
	}

	if resp.StatusCode() != http.StatusCreated && resp.StatusCode() != http.StatusOK {
		return c.handleError(resp)
	}

	return nil
}

// DeleteContext deletes a context
func (c *Client) DeleteContext(ctx context.Context, tenant, project, contextPath string) error {
	var path string
	if project != "" {
		path = apiAdminTenants + buildPath(tenant, "projects", project, "contexts", contextPath)
	} else {
		path = apiAdminTenants + buildPath(tenant, "contexts", contextPath)
	}

	resp, err := c.http.R().
		SetContext(ctx).
		Delete(path)

	if err != nil {
		return fmt.Errorf("failed to delete context: %w", err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}

// ============================================================================
// TENANT OPERATIONS
// ============================================================================

// ListTenants lists all tenants
// The right parameter filters tenants by minimum required permission level (Read, Write, or Admin)
func (c *Client) ListTenants(ctx context.Context, right *RightLevel) ([]Tenant, error) {
	req := c.http.R().
		SetContext(ctx).
		SetResult(&[]Tenant{})

	// Add right filter if specified
	if right != nil {
		req.SetQueryParam("right", right.String())
	}

	resp, err := req.Get("/api/admin/tenants")

	if err != nil {
		return nil, fmt.Errorf("failed to list tenants: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	tenants := resp.Result().(*[]Tenant)
	return *tenants, nil
}

// GetTenant retrieves a specific tenant
func (c *Client) GetTenant(ctx context.Context, name string) (*Tenant, error) {
	path := apiAdminTenants + buildPath(name)

	var tenant Tenant
	resp, err := c.http.R().
		SetContext(ctx).
		SetResult(&tenant).
		Get(path)

	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return &tenant, nil
}

// CreateTenant creates a new tenant
// The tenant parameter accepts either a *Tenant or any compatible struct
func (c *Client) CreateTenant(ctx context.Context, tenant interface{}) error {
	resp, err := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(tenant).
		Post("/api/admin/tenants")

	if err != nil {
		return fmt.Errorf("failed to create tenant: %w", err)
	}

	if resp.StatusCode() != http.StatusCreated && resp.StatusCode() != http.StatusOK {
		return c.handleError(resp)
	}

	return nil
}

// DeleteTenant deletes a tenant
func (c *Client) DeleteTenant(ctx context.Context, name string) error {
	path := apiAdminTenants + buildPath(name)

	resp, err := c.http.R().
		SetContext(ctx).
		Delete(path)

	if err != nil {
		return fmt.Errorf("failed to delete tenant: %w", err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}

// ============================================================================
// PROJECT OPERATIONS
// ============================================================================

// ListProjects lists all projects in a tenant
func (c *Client) ListProjects(ctx context.Context, tenant string) ([]Project, error) {
	path := apiAdminTenants + buildPath(tenant, "projects")

	var projects []Project
	resp, err := c.http.R().
		SetContext(ctx).
		SetResult(&projects).
		Get(path)

	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return projects, nil
}

// GetProject retrieves a specific project
func (c *Client) GetProject(ctx context.Context, tenant, project string) (*Project, error) {
	path := apiAdminTenants + buildPath(tenant, "projects", project)

	var proj Project
	resp, err := c.http.R().
		SetContext(ctx).
		SetResult(&proj).
		Get(path)

	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return &proj, nil
}

// CreateProject creates a new project
// The project parameter accepts either a *Project or any compatible struct
func (c *Client) CreateProject(ctx context.Context, tenant string, project interface{}) error {
	path := apiAdminTenants + buildPath(tenant, "projects")

	resp, err := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(project).
		Post(path)

	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}

	if resp.StatusCode() != http.StatusCreated && resp.StatusCode() != http.StatusOK {
		return c.handleError(resp)
	}

	return nil
}

// DeleteProject deletes a project
func (c *Client) DeleteProject(ctx context.Context, tenant, project string) error {
	path := apiAdminTenants + buildPath(tenant, "projects", project)

	resp, err := c.http.R().
		SetContext(ctx).
		Delete(path)

	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}

// ============================================================================
// API KEY OPERATIONS
// ============================================================================

// ListAPIKeys lists all API keys for a tenant
func (c *Client) ListAPIKeys(ctx context.Context, tenant string) ([]APIKey, error) {
	path := apiAdminTenants + buildPath(tenant, "keys")

	var keys []APIKey
	resp, err := c.http.R().
		SetContext(ctx).
		SetResult(&keys).
		Get(path)

	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return keys, nil
}

// GetAPIKey retrieves a specific API key by clientID
// Note: The Izanami API doesn't have a dedicated endpoint for getting a single key,
// so this method lists all keys and filters by clientID
func (c *Client) GetAPIKey(ctx context.Context, tenant, clientID string) (*APIKey, error) {
	keys, err := c.ListAPIKeys(ctx, tenant)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	// Find the key with matching clientID
	for i := range keys {
		if keys[i].ClientID == clientID {
			return &keys[i], nil
		}
	}

	return nil, &APIError{
		StatusCode: http.StatusNotFound,
		Message:    fmt.Sprintf("API key with clientID '%s' not found", clientID),
		RawBody:    "",
	}
}

// CreateAPIKey creates a new API key
// The key parameter accepts either an *APIKey or any compatible struct
func (c *Client) CreateAPIKey(ctx context.Context, tenant string, key interface{}) (*APIKey, error) {
	path := apiAdminTenants + buildPath(tenant, "keys")

	var result APIKey
	resp, err := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(key).
		SetResult(&result).
		Post(path)

	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	if resp.StatusCode() != http.StatusCreated && resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return &result, nil
}

// UpdateAPIKey updates an existing API key
// The key parameter accepts either an *APIKey or any compatible struct
func (c *Client) UpdateAPIKey(ctx context.Context, tenant, clientID string, key interface{}) error {
	path := apiAdminTenants + buildPath(tenant, "keys", clientID)

	resp, err := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(key).
		Put(path)

	if err != nil {
		return fmt.Errorf("failed to update API key: %w", err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}

// DeleteAPIKey deletes an API key
func (c *Client) DeleteAPIKey(ctx context.Context, tenant, clientID string) error {
	path := apiAdminTenants + buildPath(tenant, "keys", clientID)

	resp, err := c.http.R().
		SetContext(ctx).
		Delete(path)

	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}

// ============================================================================
// TAG OPERATIONS
// ============================================================================

// ListTags lists all tags in a tenant
func (c *Client) ListTags(ctx context.Context, tenant string) ([]Tag, error) {
	path := apiAdminTenants + buildPath(tenant, "tags")

	var tags []Tag
	resp, err := c.http.R().
		SetContext(ctx).
		SetResult(&tags).
		Get(path)

	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return tags, nil
}

// CreateTag creates a new tag
// The tag parameter accepts either a *Tag or any compatible struct
func (c *Client) CreateTag(ctx context.Context, tenant string, tag interface{}) error {
	path := apiAdminTenants + buildPath(tenant, "tags")

	resp, err := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(tag).
		Post(path)

	if err != nil {
		return fmt.Errorf("failed to create tag: %w", err)
	}

	if resp.StatusCode() != http.StatusCreated && resp.StatusCode() != http.StatusOK {
		return c.handleError(resp)
	}

	return nil
}

// DeleteTag deletes a tag
func (c *Client) DeleteTag(ctx context.Context, tenant, tagName string) error {
	path := apiAdminTenants + buildPath(tenant, "tags", tagName)

	resp, err := c.http.R().
		SetContext(ctx).
		Delete(path)

	if err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}

// ============================================================================
// EVENT STREAMING (SSE)
// ============================================================================

// Event represents a Server-Sent Event from Izanami
type Event struct {
	ID   string
	Type string
	Data string
}

// EventCallback is called for each received event
type EventCallback func(event Event) error

// WatchEvents opens a Server-Sent Events stream to watch for feature flag changes.
// The callback is called for each event received. Return an error to stop watching.
// Implements automatic reconnection with exponential backoff on connection failures.
func (c *Client) WatchEvents(ctx context.Context, callback EventCallback) error {
	lastEventID := ""
	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		retryDelay, err := c.streamEvents(ctx, lastEventID, func(event Event) error {
			lastEventID = event.ID
			return callback(event)
		})

		if err != nil {
			// If context was cancelled, return immediately without logging
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// For EOF without context cancellation, it's a normal stream end
			if errors.Is(err, io.EOF) {
				// Use minimal backoff for clean disconnects
				backoff = 2 * time.Second
			} else {
				// For other errors, use exponential backoff
				backoff = time.Duration(math.Min(float64(backoff*2), float64(maxBackoff)))
			}
		} else {
			// Stream ended normally, reset backoff
			backoff = 1 * time.Second
		}

		// If server sent a retry delay via SSE retry: field, use it
		if retryDelay > 0 {
			backoff = retryDelay
		}

		// Wait before reconnecting
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
}

// streamEvents opens a single SSE connection and processes events.
// Returns the retry delay suggested by the server (if any) and any error encountered.
func (c *Client) streamEvents(ctx context.Context, lastEventID string, callback EventCallback) (time.Duration, error) {
	req := c.http.R().SetContext(ctx).SetDoNotParseResponse(true)

	if lastEventID != "" {
		req.SetHeader("Last-Event-Id", lastEventID)
	}

	resp, err := req.Get("/api/v2/events")
	if err != nil {
		// Check if this is a context cancellation
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		return 0, fmt.Errorf("failed to connect to event stream: %w", err)
	}
	defer resp.RawBody().Close()

	if resp.StatusCode() != http.StatusOK {
		return 0, fmt.Errorf("event stream returned status %d", resp.StatusCode())
	}

	retryDelay, err := c.parseSSE(ctx, resp.RawBody(), callback)

	// If context was cancelled, return context error instead of parse error
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}

	return retryDelay, err
}

// parseSSE parses Server-Sent Events from the response body.
// Returns the retry delay from the server (if specified) and any error encountered.
// Supports the SSE retry: field for dynamic reconnection timing.
func (c *Client) parseSSE(ctx context.Context, body io.ReadCloser, callback EventCallback) (time.Duration, error) {
	reader := bufio.NewReader(body)
	var event Event
	var retryDelay time.Duration

	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return retryDelay, ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			// Don't wrap EOF when context is cancelled
			if err == io.EOF && ctx.Err() != nil {
				return retryDelay, ctx.Err()
			}
			if err == io.EOF {
				return retryDelay, err
			}
			return retryDelay, fmt.Errorf("error reading event stream: %w", err)
		}

		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")

		// Empty line means end of event
		if line == "" {
			if event.Data != "" {
				if err := callback(event); err != nil {
					return retryDelay, err
				}
				event = Event{}
			}
			continue
		}

		// Parse field
		if strings.HasPrefix(line, ":") {
			// Comment, ignore
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		field := parts[0]
		value := strings.TrimPrefix(parts[1], " ")

		switch field {
		case "id":
			event.ID = value
		case "event":
			event.Type = value
		case "data":
			if event.Data != "" {
				event.Data += "\n"
			}
			event.Data += value
		case "retry":
			// SSE retry field specifies reconnection delay in milliseconds
			if ms, err := strconv.ParseInt(value, 10, 64); err == nil && ms > 0 {
				retryDelay = time.Duration(ms) * time.Millisecond
			}
		}
	}
}

// ============================================================================
// UTILITY OPERATIONS
// ============================================================================

// Health checks the health status of Izanami
func (c *Client) Health(ctx context.Context) (*HealthStatus, error) {
	var health HealthStatus
	resp, err := c.http.R().
		SetContext(ctx).
		SetResult(&health).
		Get("/api/_health")

	if err != nil {
		return nil, fmt.Errorf("failed to check health: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return &health, nil
}

// Search performs a global search
func (c *Client) Search(ctx context.Context, tenant, query string, filters []string) ([]SearchResult, error) {
	var path string
	if tenant != "" {
		path = apiAdminTenants + buildPath(tenant, "search")
	} else {
		path = "/api/admin/search"
	}

	req := c.http.R().
		SetContext(ctx).
		SetResult(&[]SearchResult{}).
		SetQueryParam("query", query)

	if len(filters) > 0 {
		req.SetQueryParam("filter", strings.Join(filters, ","))
	}

	resp, err := req.Get(path)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	results := resp.Result().(*[]SearchResult)
	return *results, nil
}

// Export exports tenant data
func (c *Client) Export(ctx context.Context, tenant string) (string, error) {
	path := apiAdminTenants + buildPath(tenant, "_export")

	resp, err := c.http.R().
		SetContext(ctx).
		SetHeader("Accept", "application/x-ndjson").
		Post(path)

	if err != nil {
		return "", fmt.Errorf("failed to export: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return "", c.handleError(resp)
	}

	return string(resp.Body()), nil
}

// Import imports tenant data from a file
// All fields from ImportRequest are passed as query parameters to the API
func (c *Client) Import(ctx context.Context, tenant, filePath string, req ImportRequest) (*ImportStatus, error) {
	path := apiAdminTenants + buildPath(tenant, "_import")

	var status ImportStatus

	httpReq := c.http.R().
		SetContext(ctx).
		SetFile("export", filePath).
		SetResult(&status)

	// Set all query parameters from ImportRequest
	if req.Version > 0 {
		httpReq.SetQueryParam("version", strconv.Itoa(req.Version))
	}
	if req.Conflict != "" {
		httpReq.SetQueryParam("conflict", req.Conflict)
	}
	if req.Timezone != "" {
		httpReq.SetQueryParam("timezone", req.Timezone)
	}
	if req.DeduceProject {
		httpReq.SetQueryParam("deduceProject", "true")
	}
	if req.CreateProjects {
		httpReq.SetQueryParam("create", "true")
	}
	if req.Project != "" {
		httpReq.SetQueryParam("project", req.Project)
	}
	if req.ProjectPartSize > 0 {
		httpReq.SetQueryParam("projectPartSize", strconv.Itoa(req.ProjectPartSize))
	}
	if req.InlineScript {
		httpReq.SetQueryParam("inlineScript", "true")
	}

	resp, err := httpReq.Post(path)

	if err != nil {
		return nil, fmt.Errorf("failed to import: %w", err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusCreated {
		return nil, c.handleError(resp)
	}

	return &status, nil
}
