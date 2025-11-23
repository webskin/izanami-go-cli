package izanami

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	errmsg "github.com/webskin/izanami-go-cli/internal/errors"
)

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
// The request parameter allows filtering which events to receive and setting refresh intervals.
func (c *Client) WatchEvents(ctx context.Context, request EventsWatchRequest, callback EventCallback) error {
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
func (c *Client) streamEvents(ctx context.Context, request EventsWatchRequest, lastEventID string, callback EventCallback) (time.Duration, error) {
	req := c.http.R().SetContext(ctx).SetDoNotParseResponse(true)

	// Set client authentication
	if err := c.setClientAuth(req); err != nil {
		return 0, err
	}

	if lastEventID != "" {
		req.SetHeader("Last-Event-Id", lastEventID)
	}

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
		req.SetQueryParam("allTagIn", strings.Join(request.AllTagsIn, ","))
	}
	if len(request.NoTagIn) > 0 {
		req.SetQueryParam("noTagIn", strings.Join(request.NoTagIn, ","))
	}
	if request.RefreshInterval > 0 {
		req.SetQueryParam("refreshInterval", strconv.Itoa(request.RefreshInterval))
	}
	if request.KeepAliveInterval > 0 {
		req.SetQueryParam("keepAliveInterval", strconv.Itoa(request.KeepAliveInterval))
	}

	var resp *resty.Response
	var err error

	// Use POST if payload is provided, GET otherwise
	if request.Payload != "" {
		req.SetHeader("Content-Type", "application/json").SetBody(request.Payload)
		resp, err = req.Post("/api/v2/events")
	} else {
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

// checkContextCancellation checks if context is cancelled
func checkContextCancellation(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// readSSELine reads and normalizes a line from the SSE stream
func readSSELine(ctx context.Context, reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		// Don't wrap EOF when context is cancelled
		if err == io.EOF && ctx.Err() != nil {
			return "", ctx.Err()
		}
		if err == io.EOF {
			return "", err
		}
		return "", fmt.Errorf("%s: %w", errmsg.MsgErrorReadingEventStream, err)
	}

	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	return line, nil
}

// parseSSEField parses a field:value pair and returns field and value
func parseSSEField(line string) (field, value string, ok bool) {
	// Comment, ignore
	if strings.HasPrefix(line, ":") {
		return "", "", false
	}

	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	field = parts[0]
	value = strings.TrimPrefix(parts[1], " ")
	return field, value, true
}

// updateEventField updates the event based on field type and returns retry delay if set
func updateEventField(event *Event, field, value string) time.Duration {
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
			return time.Duration(ms) * time.Millisecond
		}
	}
	return 0
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
