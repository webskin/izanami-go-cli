package izanami

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

// Client represents an Izanami HTTP client
type Client struct {
	http   *resty.Client
	config *Config
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
			// Retry on network errors or 5xx status codes
			return err != nil || r.StatusCode() >= 500
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

// enableSecureDebugMode enables verbose logging with sensitive data redaction
func enableSecureDebugMode(client *resty.Client) {
	// Sensitive headers that should be redacted
	sensitiveHeaders := map[string]bool{
		"cookie":                 true,
		"authorization":          true,
		"izanami-client-secret":  true,
		"izanami-client-id":      true,
		"x-api-key":              true,
		"authentication":         true,
	}

	// Log response details (after receiving)
	// We log both request and response here because the request details
	// are only fully available after the request is sent
	client.OnAfterResponse(func(c *resty.Client, resp *resty.Response) error {
		req := resp.Request.RawRequest

		// Log request details
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
		if resp.Request.Body != nil {
			fmt.Fprintf(os.Stderr, "%v\n", resp.Request.Body)
		} else {
			fmt.Fprintf(os.Stderr, "***** NO CONTENT *****\n")
		}
		fmt.Fprintf(os.Stderr, "------------------------------------------------------------------------------\n")

		// Log response details
		fmt.Fprintf(os.Stderr, "~~~ RESPONSE ~~~\n")
		fmt.Fprintf(os.Stderr, "STATUS       : %s\n", resp.Status())
		fmt.Fprintf(os.Stderr, "PROTO        : %s\n", resp.Proto())
		fmt.Fprintf(os.Stderr, "RECEIVED AT  : %v\n", time.Now().Format(time.RFC3339Nano))
		fmt.Fprintf(os.Stderr, "TIME DURATION: %v\n", resp.Time())
		fmt.Fprintf(os.Stderr, "HEADERS      :\n")
		for key, values := range resp.Header() {
			for _, value := range values {
				fmt.Fprintf(os.Stderr, "\t%s: %s\n", key, value)
			}
		}
		fmt.Fprintf(os.Stderr, "BODY         :\n")
		if len(resp.Body()) > 0 {
			fmt.Fprintf(os.Stderr, "%s\n", string(resp.Body()))
		} else {
			fmt.Fprintf(os.Stderr, "***** NO CONTENT *****\n")
		}
		fmt.Fprintf(os.Stderr, "==============================================================================\n")
		return nil
	})
}

// handleError parses error responses from the API
func (c *Client) handleError(resp *resty.Response) error {
	var errResp ErrorResponse
	if err := json.Unmarshal(resp.Body(), &errResp); err == nil && errResp.Message != "" {
		return fmt.Errorf("API error (%d): %s", resp.StatusCode(), errResp.Message)
	}
	return fmt.Errorf("API error (%d): %s", resp.StatusCode(), string(resp.Body()))
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
	path := fmt.Sprintf("/api/admin/tenants/%s/features", tenant)

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
	path := fmt.Sprintf("/api/admin/tenants/%s/features/%s", tenant, featureID)

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
func (c *Client) CreateFeature(ctx context.Context, tenant, project string, feature interface{}) (*Feature, error) {
	path := fmt.Sprintf("/api/admin/tenants/%s/projects/%s/features", tenant, project)

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
func (c *Client) UpdateFeature(ctx context.Context, tenant, featureID string, feature interface{}, preserveProtectedContexts bool) error {
	path := fmt.Sprintf("/api/admin/tenants/%s/features/%s", tenant, featureID)

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
	path := fmt.Sprintf("/api/admin/tenants/%s/features/%s", tenant, featureID)

	resp, err := c.http.R().
		SetContext(ctx).
		Delete(path)

	if err != nil {
		return fmt.Errorf("failed to delete feature: %w", err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}

// CheckFeature checks if a feature is active (client API)
func (c *Client) CheckFeature(ctx context.Context, featureID, user, contextPath string) (*FeatureCheckResult, error) {
	path := fmt.Sprintf("/api/v2/features/%s", featureID)

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
		path = fmt.Sprintf("/api/admin/tenants/%s/projects/%s/contexts", tenant, project)
	} else {
		path = fmt.Sprintf("/api/admin/tenants/%s/contexts", tenant)
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
func (c *Client) CreateContext(ctx context.Context, tenant, project, name, parentPath string, contextData interface{}) error {
	var path string
	if project != "" {
		if parentPath != "" {
			path = fmt.Sprintf("/api/admin/tenants/%s/projects/%s/contexts/%s", tenant, project, parentPath)
		} else {
			path = fmt.Sprintf("/api/admin/tenants/%s/projects/%s/contexts", tenant, project)
		}
	} else {
		if parentPath != "" {
			path = fmt.Sprintf("/api/admin/tenants/%s/contexts/%s", tenant, parentPath)
		} else {
			path = fmt.Sprintf("/api/admin/tenants/%s/contexts", tenant)
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
		path = fmt.Sprintf("/api/admin/tenants/%s/projects/%s/contexts/%s", tenant, project, contextPath)
	} else {
		path = fmt.Sprintf("/api/admin/tenants/%s/contexts/%s", tenant, contextPath)
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
func (c *Client) ListTenants(ctx context.Context) ([]Tenant, error) {
	var tenants []Tenant
	resp, err := c.http.R().
		SetContext(ctx).
		SetResult(&tenants).
		Get("/api/admin/tenants")

	if err != nil {
		return nil, fmt.Errorf("failed to list tenants: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return tenants, nil
}

// GetTenant retrieves a specific tenant
func (c *Client) GetTenant(ctx context.Context, name string) (*Tenant, error) {
	var tenant Tenant
	resp, err := c.http.R().
		SetContext(ctx).
		SetResult(&tenant).
		Get(fmt.Sprintf("/api/admin/tenants/%s", name))

	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return &tenant, nil
}

// CreateTenant creates a new tenant
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
	resp, err := c.http.R().
		SetContext(ctx).
		Delete(fmt.Sprintf("/api/admin/tenants/%s", name))

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
	var projects []Project
	resp, err := c.http.R().
		SetContext(ctx).
		SetResult(&projects).
		Get(fmt.Sprintf("/api/admin/tenants/%s/projects", tenant))

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
	var proj Project
	resp, err := c.http.R().
		SetContext(ctx).
		SetResult(&proj).
		Get(fmt.Sprintf("/api/admin/tenants/%s/projects/%s", tenant, project))

	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return &proj, nil
}

// CreateProject creates a new project
func (c *Client) CreateProject(ctx context.Context, tenant string, project interface{}) error {
	resp, err := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(project).
		Post(fmt.Sprintf("/api/admin/tenants/%s/projects", tenant))

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
	resp, err := c.http.R().
		SetContext(ctx).
		Delete(fmt.Sprintf("/api/admin/tenants/%s/projects/%s", tenant, project))

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
	var keys []APIKey
	resp, err := c.http.R().
		SetContext(ctx).
		SetResult(&keys).
		Get(fmt.Sprintf("/api/admin/tenants/%s/keys", tenant))

	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return keys, nil
}

// GetAPIKey retrieves a specific API key
func (c *Client) GetAPIKey(ctx context.Context, tenant, clientID string) (*APIKey, error) {
	var key APIKey
	resp, err := c.http.R().
		SetContext(ctx).
		SetResult(&key).
		Get(fmt.Sprintf("/api/admin/tenants/%s/keys/%s", tenant, clientID))

	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return &key, nil
}

// CreateAPIKey creates a new API key
func (c *Client) CreateAPIKey(ctx context.Context, tenant string, key interface{}) (*APIKey, error) {
	var result APIKey
	resp, err := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(key).
		SetResult(&result).
		Post(fmt.Sprintf("/api/admin/tenants/%s/keys", tenant))

	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	if resp.StatusCode() != http.StatusCreated && resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return &result, nil
}

// UpdateAPIKey updates an existing API key
func (c *Client) UpdateAPIKey(ctx context.Context, tenant, clientID string, key interface{}) error {
	resp, err := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(key).
		Put(fmt.Sprintf("/api/admin/tenants/%s/keys/%s", tenant, clientID))

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
	resp, err := c.http.R().
		SetContext(ctx).
		Delete(fmt.Sprintf("/api/admin/tenants/%s/keys/%s", tenant, clientID))

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
	var tags []Tag
	resp, err := c.http.R().
		SetContext(ctx).
		SetResult(&tags).
		Get(fmt.Sprintf("/api/admin/tenants/%s/tags", tenant))

	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return tags, nil
}

// CreateTag creates a new tag
func (c *Client) CreateTag(ctx context.Context, tenant string, tag interface{}) error {
	resp, err := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(tag).
		Post(fmt.Sprintf("/api/admin/tenants/%s/tags", tenant))

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
	resp, err := c.http.R().
		SetContext(ctx).
		Delete(fmt.Sprintf("/api/admin/tenants/%s/tags/%s", tenant, tagName))

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

// WatchEvents opens a Server-Sent Events stream to watch for feature flag changes
// The callback is called for each event received. Return an error to stop watching.
func (c *Client) WatchEvents(ctx context.Context, callback EventCallback) error {
	lastEventID := ""

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := c.streamEvents(ctx, lastEventID, func(event Event) error {
			lastEventID = event.ID
			return callback(event)
		})

		if err != nil {
			// If context was cancelled, return
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// Otherwise, wait a bit and reconnect
			time.Sleep(2 * time.Second)
		}
	}
}

// streamEvents opens a single SSE connection and processes events
func (c *Client) streamEvents(ctx context.Context, lastEventID string, callback EventCallback) error {
	req := c.http.R().SetContext(ctx).SetDoNotParseResponse(true)

	if lastEventID != "" {
		req.SetHeader("Last-Event-Id", lastEventID)
	}

	resp, err := req.Get("/api/v2/events")
	if err != nil {
		return fmt.Errorf("failed to connect to event stream: %w", err)
	}
	defer resp.RawBody().Close()

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("event stream returned status %d", resp.StatusCode())
	}

	return c.parseSSE(resp.RawBody(), callback)
}

// parseSSE parses Server-Sent Events from the response body
func (c *Client) parseSSE(body io.ReadCloser, callback EventCallback) error {
	reader := bufio.NewReader(body)
	var event Event

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("error reading event stream: %w", err)
		}

		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")

		// Empty line means end of event
		if line == "" {
			if event.Data != "" {
				if err := callback(event); err != nil {
					return err
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
		path = fmt.Sprintf("/api/admin/tenants/%s/search", tenant)
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
	resp, err := c.http.R().
		SetContext(ctx).
		SetHeader("Accept", "application/x-ndjson").
		Post(fmt.Sprintf("/api/admin/tenants/%s/_export", tenant))

	if err != nil {
		return "", fmt.Errorf("failed to export: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return "", c.handleError(resp)
	}

	return string(resp.Body()), nil
}

// Import imports tenant data
func (c *Client) Import(ctx context.Context, tenant, filePath string, req ImportRequest) (*ImportStatus, error) {
	var status ImportStatus

	resp, err := c.http.R().
		SetContext(ctx).
		SetFile("export", filePath).
		SetQueryParam("conflict", req.Conflict).
		SetQueryParam("timezone", req.Timezone).
		SetResult(&status).
		Post(fmt.Sprintf("/api/admin/tenants/%s/_import", tenant))

	if err != nil {
		return nil, fmt.Errorf("failed to import: %w", err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusCreated {
		return nil, c.handleError(resp)
	}

	return &status, nil
}
