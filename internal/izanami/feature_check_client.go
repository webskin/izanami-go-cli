package izanami

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	errmsg "github.com/webskin/izanami-go-cli/internal/errors"
)

// FeatureCheckClient represents an Izanami HTTP client for client API operations (/api/v2/*)
// Use this for client operations like feature checks and event streaming.
// For admin operations (feature management, projects, etc.), use AdminClient instead.
type FeatureCheckClient struct {
	http   *resty.Client
	config *ResolvedConfig
}

// NewFeatureCheckClient creates a new Izanami feature check client with the given configuration.
// This validates that client authentication (client-id/secret) is configured.
// The client uses WorkerURL if resolved, otherwise falls back to LeaderURL.
// The caller is responsible for applying credential precedence before calling this.
// For admin operations, use NewAdminClient instead.
func NewFeatureCheckClient(config *ResolvedConfig) (*FeatureCheckClient, error) {
	if err := config.ValidateClientAuth(); err != nil {
		return nil, err
	}

	configCopy := copyConfig(config)

	// Use WorkerURL if set, otherwise use LeaderURL
	baseURL := configCopy.GetWorkerURL()

	httpClient := newHTTPClient(baseURL, configCopy.Timeout, configCopy.InsecureSkipVerify)

	client := &FeatureCheckClient{
		http:   httpClient,
		config: configCopy,
	}

	if configCopy.Verbose {
		enableFeatureCheckSecureDebugMode(httpClient, client)
	}

	return client, nil
}

// setClientAuth sets authentication for client API requests (client-id/secret headers)
func (c *FeatureCheckClient) setClientAuth(req *resty.Request) {
	req.SetHeader("Izanami-Client-Id", c.config.ClientID)
	req.SetHeader("Izanami-Client-Secret", c.config.ClientSecret)
}

// handleError parses error responses from the API and returns a structured APIError
func (c *FeatureCheckClient) handleError(resp *resty.Response) error {
	return parseAPIError(resp)
}

// enableFeatureCheckSecureDebugMode enables verbose logging with sensitive data redaction for FeatureCheckClient
func enableFeatureCheckSecureDebugMode(httpClient *resty.Client, client *FeatureCheckClient) {
	sensitiveHeaders := sensitiveHeadersMap()

	httpClient.OnAfterResponse(func(c *resty.Client, resp *resty.Response) error {
		logFeatureCheckRequest(resp, sensitiveHeaders, client)
		logFeatureCheckResponse(resp, sensitiveHeaders)
		return nil
	})
}

// logFeatureCheckRequest logs HTTP request details with sensitive data redaction for FeatureCheckClient
func logFeatureCheckRequest(resp *resty.Response, sensitiveHeaders map[string]bool, client *FeatureCheckClient) {
	logRequestToStderr(resp, sensitiveHeaders)
}

// logFeatureCheckResponse logs HTTP response details with sensitive data redaction
func logFeatureCheckResponse(resp *resty.Response, sensitiveHeaders map[string]bool) {
	logResponseToStderr(resp, sensitiveHeaders)
}

// LogSSERequest logs SSE request details when verbose mode is enabled.
// This is needed because SetDoNotParseResponse(true) bypasses OnAfterResponse hooks.
func (c *FeatureCheckClient) LogSSERequest(method, path string, queryParams map[string]string, headers map[string]string, body interface{}) {
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
	fmt.Fprintf(os.Stderr, "HOST   : %s\n", c.config.GetWorkerURL())
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
func (c *FeatureCheckClient) LogSSEResponse(statusCode int, status string) {
	if !c.config.Verbose {
		return
	}

	fmt.Fprintf(os.Stderr, "~~~ RESPONSE (SSE) ~~~\n")
	fmt.Fprintf(os.Stderr, "STATUS       : %d %s\n", statusCode, status)
	fmt.Fprintf(os.Stderr, "BODY         : [streaming...]\n")
	fmt.Fprintf(os.Stderr, "==============================================================================\n")
}

// ============================================================================
// FEATURE CHECK OPERATIONS (Client API)
// ============================================================================

// CheckFeature checks if a feature is active (client API) and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseFeatureCheckResult for typed struct.
// Note: Feature check uses client-id/client-secret authentication.
// If payload is provided, uses POST method for script features, otherwise uses GET.
func CheckFeature[T any](c *FeatureCheckClient, ctx context.Context, featureID, user, contextPath, payload string, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.checkFeatureRaw(ctx, featureID, user, contextPath, payload)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// checkFeatureRaw fetches feature check result and returns raw JSON bytes
func (c *FeatureCheckClient) checkFeatureRaw(ctx context.Context, featureID, user, contextPath, payload string) ([]byte, error) {
	path := "/api/v2/features/" + buildPath(featureID)

	req := c.http.R().SetContext(ctx)
	c.setClientAuth(req)

	if user != "" {
		req.SetQueryParam("user", user)
	}
	if contextPath != "" {
		req.SetQueryParam("context", normalizeContextPath(contextPath))
	}

	var resp *resty.Response
	var err error

	// Use POST if payload is provided, GET otherwise
	if payload != "" {
		req.SetHeader("Content-Type", "application/json").SetBody(payload)
		resp, err = req.Post(path)
	} else {
		resp, err = req.Get(path)
	}

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToCheckFeature, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}

// CheckFeatures checks activation for multiple features (bulk operation) and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseActivationsWithConditions for typed map.
// Supports filtering by feature IDs, projects, tags, and returns conditions if requested.
func CheckFeatures[T any](c *FeatureCheckClient, ctx context.Context, request CheckFeaturesRequest, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.checkFeaturesRaw(ctx, request)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// checkFeaturesRaw fetches feature activations and returns raw JSON bytes
func (c *FeatureCheckClient) checkFeaturesRaw(ctx context.Context, request CheckFeaturesRequest) ([]byte, error) {
	path := "/api/v2/features"

	req := c.http.R().SetContext(ctx)
	c.setClientAuth(req)

	// Set query parameters
	if request.User != "" {
		req.SetQueryParam("user", request.User)
	}
	if request.Context != "" {
		req.SetQueryParam("context", normalizeContextPath(request.Context))
	}
	if len(request.Features) > 0 {
		req.SetQueryParam("features", strings.Join(request.Features, ","))
	}
	if len(request.Projects) > 0 {
		req.SetQueryParam("projects", strings.Join(request.Projects, ","))
	}
	if request.Conditions {
		req.SetQueryParam("conditions", "true")
	}
	if request.Date != "" {
		req.SetQueryParam("date", request.Date)
	}
	if len(request.OneTagIn) > 0 {
		req.SetQueryParam("oneTagIn", strings.Join(request.OneTagIn, ","))
	}
	if len(request.AllTagsIn) > 0 {
		req.SetQueryParam("allTagsIn", strings.Join(request.AllTagsIn, ","))
	}
	if len(request.NoTagIn) > 0 {
		req.SetQueryParam("noTagIn", strings.Join(request.NoTagIn, ","))
	}

	var resp *resty.Response
	var err error

	// Use POST if payload is provided, GET otherwise
	if request.Payload != "" {
		req.SetHeader("Content-Type", "application/json").SetBody(request.Payload)
		resp, err = req.Post(path)
	} else {
		resp, err = req.Get(path)
	}

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToCheckFeatures, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}

// ============================================================================
// EVENT STREAMING (SSE)
// ============================================================================

// WatchEvents opens a Server-Sent Events stream to watch for feature flag changes.
// The callback is called for each event received. Return an error to stop watching.
// Implements automatic reconnection with exponential backoff on connection failures.
// The request parameter allows filtering which events to receive and setting refresh intervals.
func (c *FeatureCheckClient) WatchEvents(ctx context.Context, request EventsWatchRequest, callback EventCallback) error {
	lastEventID := ""
	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		retryDelay, err := c.streamEvents(ctx, request, lastEventID, func(event Event) error {
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
func (c *FeatureCheckClient) streamEvents(ctx context.Context, request EventsWatchRequest, lastEventID string, callback EventCallback) (time.Duration, error) {
	req := c.http.R().SetContext(ctx).SetDoNotParseResponse(true)

	// Collect headers for logging
	headers := make(map[string]string)

	// Set client authentication
	c.setClientAuth(req)
	headers["Izanami-Client-Id"] = c.config.ClientID
	headers["Izanami-Client-Secret"] = c.config.ClientSecret

	if lastEventID != "" {
		req.SetHeader("Last-Event-Id", lastEventID)
		headers["Last-Event-Id"] = lastEventID
	}

	// Collect query parameters for logging
	queryParams := make(map[string]string)

	// Set query parameters
	if request.User != "" {
		req.SetQueryParam("user", request.User)
		queryParams["user"] = request.User
	}
	if request.Context != "" {
		ctxPath := normalizeContextPath(request.Context)
		req.SetQueryParam("context", ctxPath)
		queryParams["context"] = ctxPath
	}
	if len(request.Features) > 0 {
		features := strings.Join(request.Features, ",")
		req.SetQueryParam("features", features)
		queryParams["features"] = features
	}
	if len(request.Projects) > 0 {
		projects := strings.Join(request.Projects, ",")
		req.SetQueryParam("projects", projects)
		queryParams["projects"] = projects
	}
	if request.Conditions {
		req.SetQueryParam("conditions", "true")
		queryParams["conditions"] = "true"
	}
	if request.Date != "" {
		req.SetQueryParam("date", request.Date)
		queryParams["date"] = request.Date
	}
	if len(request.OneTagIn) > 0 {
		oneTagIn := strings.Join(request.OneTagIn, ",")
		req.SetQueryParam("oneTagIn", oneTagIn)
		queryParams["oneTagIn"] = oneTagIn
	}
	if len(request.AllTagsIn) > 0 {
		allTagIn := strings.Join(request.AllTagsIn, ",")
		req.SetQueryParam("allTagIn", allTagIn)
		queryParams["allTagIn"] = allTagIn
	}
	if len(request.NoTagIn) > 0 {
		noTagIn := strings.Join(request.NoTagIn, ",")
		req.SetQueryParam("noTagIn", noTagIn)
		queryParams["noTagIn"] = noTagIn
	}
	if request.RefreshInterval > 0 {
		interval := strconv.Itoa(request.RefreshInterval)
		req.SetQueryParam("refreshInterval", interval)
		queryParams["refreshInterval"] = interval
	}
	if request.KeepAliveInterval > 0 {
		interval := strconv.Itoa(request.KeepAliveInterval)
		req.SetQueryParam("keepAliveInterval", interval)
		queryParams["keepAliveInterval"] = interval
	}

	var resp *resty.Response
	var err error
	var method string

	// Use POST if payload is provided, GET otherwise
	if request.Payload != "" {
		method = "POST"
		req.SetHeader("Content-Type", "application/json").SetBody(request.Payload)
		headers["Content-Type"] = "application/json"
		c.LogSSERequest(method, "/api/v2/events", queryParams, headers, request.Payload)
		resp, err = req.Post("/api/v2/events")
	} else {
		method = "GET"
		c.LogSSERequest(method, "/api/v2/events", queryParams, headers, nil)
		resp, err = req.Get("/api/v2/events")
	}

	if err != nil {
		// Check if this is a context cancellation
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		return 0, fmt.Errorf("%s: %w", errmsg.MsgFailedToConnectToEventStream, err)
	}
	defer resp.RawBody().Close()

	// Log SSE response
	c.LogSSEResponse(resp.StatusCode(), resp.Status())

	if resp.StatusCode() != http.StatusOK {
		return 0, fmt.Errorf(errmsg.MsgEventStreamReturnedStatus, resp.StatusCode())
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
func (c *FeatureCheckClient) parseSSE(ctx context.Context, body io.ReadCloser, callback EventCallback) (time.Duration, error) {
	reader := bufio.NewReader(body)
	var event Event
	var retryDelay time.Duration

	for {
		// Check for context cancellation
		if err := checkContextCancellation(ctx); err != nil {
			return retryDelay, err
		}

		// Read next line
		line, err := readSSELine(ctx, reader)
		if err != nil {
			return retryDelay, err
		}

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
		field, value, ok := parseSSEField(line)
		if !ok {
			continue
		}

		// Update event and capture retry delay if set
		if delay := updateEventField(&event, field, value); delay > 0 {
			retryDelay = delay
		}
	}
}
