package izanami

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_WatchEvents(t *testing.T) {
	// SSE response with multiple events
	sseData := `id: 1
event: feature-created
data: {"name":"feature-1","active":true}

id: 2
event: feature-updated
data: {"name":"feature-2","active":false}

`

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/events", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		// Check query params
		user := r.URL.Query().Get("user")
		context := r.URL.Query().Get("context")
		assert.Equal(t, "user123", user)
		assert.Equal(t, "/prod", context) // context paths are normalized to have leading slash

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		// Write SSE data
		w.Write([]byte(sseData))
	})
	defer server.Close()

	config := &ResolvedConfig{
		LeaderURL:    server.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Timeout:      30,
	}

	client, err := NewFeatureCheckClient(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	request := EventsWatchRequest{
		User:    "user123",
		Context: "prod",
	}

	receivedEvents := []Event{}
	err = client.WatchEvents(ctx, request, func(event Event) error {
		receivedEvents = append(receivedEvents, event)
		// Stop after receiving 2 events
		if len(receivedEvents) >= 2 {
			cancel()
		}
		return nil
	})

	// Expect context cancellation error
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)

	// Verify we received the events
	assert.Len(t, receivedEvents, 2)
	assert.Equal(t, "1", receivedEvents[0].ID)
	assert.Equal(t, "feature-created", receivedEvents[0].Type)
	assert.Contains(t, receivedEvents[0].Data, "feature-1")

	assert.Equal(t, "2", receivedEvents[1].ID)
	assert.Equal(t, "feature-updated", receivedEvents[1].Type)
	assert.Contains(t, receivedEvents[1].Data, "feature-2")
}

func TestClient_WatchEvents_WithPayload(t *testing.T) {
	sseData := `id: 1
data: {"result":true}

`

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/events", r.URL.Path)
		assert.Equal(t, "POST", r.Method) // POST when payload is provided
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sseData))
	})
	defer server.Close()

	config := &ResolvedConfig{
		LeaderURL:    server.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Timeout:      30,
	}

	client, err := NewFeatureCheckClient(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	request := EventsWatchRequest{
		Payload: `{"key":"value"}`,
	}

	receivedEvents := []Event{}
	err = client.WatchEvents(ctx, request, func(event Event) error {
		receivedEvents = append(receivedEvents, event)
		cancel()
		return nil
	})

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.Len(t, receivedEvents, 1)
}

func Test_parseSSEField(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantField string
		wantValue string
		wantOK    bool
	}{
		{
			name:      "id field",
			line:      "id: 123",
			wantField: "id",
			wantValue: "123",
			wantOK:    true,
		},
		{
			name:      "event field",
			line:      "event: feature-created",
			wantField: "event",
			wantValue: "feature-created",
			wantOK:    true,
		},
		{
			name:      "data field",
			line:      "data: {\"key\":\"value\"}",
			wantField: "data",
			wantValue: "{\"key\":\"value\"}",
			wantOK:    true,
		},
		{
			name:      "retry field",
			line:      "retry: 5000",
			wantField: "retry",
			wantValue: "5000",
			wantOK:    true,
		},
		{
			name:      "comment line",
			line:      ": this is a comment",
			wantField: "",
			wantValue: "",
			wantOK:    false,
		},
		{
			name:      "invalid line",
			line:      "invalid",
			wantField: "",
			wantValue: "",
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, value, ok := parseSSEField(tt.line)
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.Equal(t, tt.wantField, field)
				assert.Equal(t, tt.wantValue, value)
			}
		})
	}
}

func Test_updateEventField(t *testing.T) {
	tests := []struct {
		name      string
		event     Event
		field     string
		value     string
		wantEvent Event
		wantDelay time.Duration
	}{
		{
			name:      "set id",
			event:     Event{},
			field:     "id",
			value:     "123",
			wantEvent: Event{ID: "123"},
			wantDelay: 0,
		},
		{
			name:      "set event type",
			event:     Event{},
			field:     "event",
			value:     "feature-created",
			wantEvent: Event{Type: "feature-created"},
			wantDelay: 0,
		},
		{
			name:      "set data",
			event:     Event{},
			field:     "data",
			value:     "test data",
			wantEvent: Event{Data: "test data"},
			wantDelay: 0,
		},
		{
			name:      "append data",
			event:     Event{Data: "line1"},
			field:     "data",
			value:     "line2",
			wantEvent: Event{Data: "line1\nline2"},
			wantDelay: 0,
		},
		{
			name:      "set retry",
			event:     Event{},
			field:     "retry",
			value:     "5000",
			wantEvent: Event{},
			wantDelay: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := tt.event
			delay := updateEventField(&event, tt.field, tt.value)
			assert.Equal(t, tt.wantEvent, event)
			assert.Equal(t, tt.wantDelay, delay)
		})
	}
}

func Test_normalizeContextPath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "already has slash",
			input: "/prod",
			want:  "/prod",
		},
		{
			name:  "missing slash",
			input: "prod",
			want:  "/prod",
		},
		{
			name:  "path with multiple segments",
			input: "prod/us-east",
			want:  "/prod/us-east",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeContextPath(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
