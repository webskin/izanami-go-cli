package izanami

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFeatureCheckClient(t *testing.T) {
	t.Run("valid config with client credentials", func(t *testing.T) {
		config := &Config{
			BaseURL:      "http://localhost:9000",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)

		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("missing client-id returns error", func(t *testing.T) {
		config := &Config{
			BaseURL:      "http://localhost:9000",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)

		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "client credentials required")
	})

	t.Run("missing client-secret returns error", func(t *testing.T) {
		config := &Config{
			BaseURL:  "http://localhost:9000",
			ClientID: "test-client-id",
			Timeout:  30,
		}

		client, err := NewFeatureCheckClient(config)

		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "client credentials required")
	})

	t.Run("missing base URL returns error", func(t *testing.T) {
		config := &Config{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)

		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "base URL")
	})

	t.Run("uses ClientBaseURL when set", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(FeatureCheckResult{Active: true, Name: "feature1"})
		}))
		defer server.Close()

		config := &Config{
			BaseURL:       "http://admin-server:9000",
			ClientBaseURL: server.URL, // Client operations should use this
			ClientID:      "test-client-id",
			ClientSecret:  "test-client-secret",
			Timeout:       30,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		// The client should use ClientBaseURL for feature checks
		ctx := context.Background()
		result, err := CheckFeature(client, ctx, "feature1", "", "", "", ParseFeatureCheckResult)

		require.NoError(t, err)
		assert.Equal(t, true, result.Active)
	})

	t.Run("falls back to BaseURL when ClientBaseURL not set", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(FeatureCheckResult{Active: true, Name: "feature1"})
		}))
		defer server.Close()

		config := &Config{
			BaseURL:      server.URL,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		ctx := context.Background()
		result, err := CheckFeature(client, ctx, "feature1", "", "", "", ParseFeatureCheckResult)

		require.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestCheckFeature(t *testing.T) {
	t.Run("GET request without payload", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/api/v2/features/feature1", r.URL.Path)
			assert.Equal(t, "test-client-id", r.Header.Get("Izanami-Client-Id"))
			assert.Equal(t, "test-client-secret", r.Header.Get("Izanami-Client-Secret"))

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(FeatureCheckResult{
				Active:  true,
				Name:    "Feature 1",
				Project: "project1",
			})
		}))
		defer server.Close()

		config := &Config{
			BaseURL:      server.URL,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		ctx := context.Background()
		result, err := CheckFeature(client, ctx, "feature1", "", "", "", ParseFeatureCheckResult)

		require.NoError(t, err)
		assert.Equal(t, true, result.Active)
		assert.Equal(t, "Feature 1", result.Name)
		assert.Equal(t, "project1", result.Project)
	})

	t.Run("POST request with payload", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			// Verify payload is sent
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			assert.Contains(t, body, "user")

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(FeatureCheckResult{Active: "custom-value", Name: "script-feature"})
		}))
		defer server.Close()

		config := &Config{
			BaseURL:      server.URL,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		ctx := context.Background()
		payload := `{"user": "testuser"}`
		result, err := CheckFeature(client, ctx, "script-feature", "", "", payload, ParseFeatureCheckResult)

		require.NoError(t, err)
		assert.Equal(t, "custom-value", result.Active)
	})

	t.Run("with user query parameter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "testuser", r.URL.Query().Get("user"))

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(FeatureCheckResult{Active: true})
		}))
		defer server.Close()

		config := &Config{
			BaseURL:      server.URL,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		ctx := context.Background()
		_, err = CheckFeature(client, ctx, "feature1", "testuser", "", "", ParseFeatureCheckResult)

		require.NoError(t, err)
	})

	t.Run("with context query parameter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Context should be normalized to have leading slash
			assert.Equal(t, "/production/us-east", r.URL.Query().Get("context"))

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(FeatureCheckResult{Active: true})
		}))
		defer server.Close()

		config := &Config{
			BaseURL:      server.URL,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		ctx := context.Background()
		_, err = CheckFeature(client, ctx, "feature1", "", "production/us-east", "", ParseFeatureCheckResult)

		require.NoError(t, err)
	})

	t.Run("handles error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(ErrorResponse{Message: "Feature not found"})
		}))
		defer server.Close()

		config := &Config{
			BaseURL:      server.URL,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		ctx := context.Background()
		_, err = CheckFeature(client, ctx, "nonexistent", "", "", "", ParseFeatureCheckResult)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "404")
	})

	t.Run("with Identity mapper returns raw JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"active":true,"name":"feature1"}`))
		}))
		defer server.Close()

		config := &Config{
			BaseURL:      server.URL,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		ctx := context.Background()
		result, err := CheckFeature(client, ctx, "feature1", "", "", "", Identity)

		require.NoError(t, err)
		assert.Equal(t, `{"active":true,"name":"feature1"}`, string(result))
	})
}

func TestCheckFeatures(t *testing.T) {
	t.Run("GET request without payload", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/api/v2/features", r.URL.Path)

			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"feature1":{"name":"Feature 1","active":true,"project":"proj1"}}`))
		}))
		defer server.Close()

		config := &Config{
			BaseURL:      server.URL,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		ctx := context.Background()
		result, err := CheckFeatures(client, ctx, CheckFeaturesRequest{}, ParseActivationsWithConditions)

		require.NoError(t, err)
		assert.Contains(t, result, "feature1")
		assert.Equal(t, "Feature 1", result["feature1"].Name)
	})

	t.Run("POST request with payload", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)

			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{}`))
		}))
		defer server.Close()

		config := &Config{
			BaseURL:      server.URL,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		ctx := context.Background()
		_, err = CheckFeatures(client, ctx, CheckFeaturesRequest{
			Payload: `{"scriptData": "test"}`,
		}, ParseActivationsWithConditions)

		require.NoError(t, err)
	})

	t.Run("with feature filter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "feature1,feature2", r.URL.Query().Get("features"))

			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{}`))
		}))
		defer server.Close()

		config := &Config{
			BaseURL:      server.URL,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		ctx := context.Background()
		_, err = CheckFeatures(client, ctx, CheckFeaturesRequest{
			Features: []string{"feature1", "feature2"},
		}, ParseActivationsWithConditions)

		require.NoError(t, err)
	})

	t.Run("with project filter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "proj1,proj2", r.URL.Query().Get("projects"))

			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{}`))
		}))
		defer server.Close()

		config := &Config{
			BaseURL:      server.URL,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		ctx := context.Background()
		_, err = CheckFeatures(client, ctx, CheckFeaturesRequest{
			Projects: []string{"proj1", "proj2"},
		}, ParseActivationsWithConditions)

		require.NoError(t, err)
	})

	t.Run("with tag filters", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "tag1", r.URL.Query().Get("oneTagIn"))
			assert.Equal(t, "tag2,tag3", r.URL.Query().Get("allTagsIn"))
			assert.Equal(t, "tag4", r.URL.Query().Get("noTagIn"))

			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{}`))
		}))
		defer server.Close()

		config := &Config{
			BaseURL:      server.URL,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		ctx := context.Background()
		_, err = CheckFeatures(client, ctx, CheckFeaturesRequest{
			OneTagIn:  []string{"tag1"},
			AllTagsIn: []string{"tag2", "tag3"},
			NoTagIn:   []string{"tag4"},
		}, ParseActivationsWithConditions)

		require.NoError(t, err)
	})

	t.Run("with conditions flag", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "true", r.URL.Query().Get("conditions"))

			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{}`))
		}))
		defer server.Close()

		config := &Config{
			BaseURL:      server.URL,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		ctx := context.Background()
		_, err = CheckFeatures(client, ctx, CheckFeaturesRequest{
			Conditions: true,
		}, ParseActivationsWithConditions)

		require.NoError(t, err)
	})

	t.Run("with date parameter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "2024-01-15T10:00:00Z", r.URL.Query().Get("date"))

			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{}`))
		}))
		defer server.Close()

		config := &Config{
			BaseURL:      server.URL,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		ctx := context.Background()
		_, err = CheckFeatures(client, ctx, CheckFeaturesRequest{
			Date: "2024-01-15T10:00:00Z",
		}, ParseActivationsWithConditions)

		require.NoError(t, err)
	})

	t.Run("with user and context", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "testuser", r.URL.Query().Get("user"))
			assert.Equal(t, "/production", r.URL.Query().Get("context"))

			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{}`))
		}))
		defer server.Close()

		config := &Config{
			BaseURL:      server.URL,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		ctx := context.Background()
		_, err = CheckFeatures(client, ctx, CheckFeaturesRequest{
			User:    "testuser",
			Context: "production",
		}, ParseActivationsWithConditions)

		require.NoError(t, err)
	})

	t.Run("handles error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(ErrorResponse{Message: "Invalid credentials"})
		}))
		defer server.Close()

		config := &Config{
			BaseURL:      server.URL,
			ClientID:     "wrong-id",
			ClientSecret: "wrong-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		ctx := context.Background()
		_, err = CheckFeatures(client, ctx, CheckFeaturesRequest{}, ParseActivationsWithConditions)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "401")
	})
}

func TestLogSSERequest(t *testing.T) {
	t.Run("does nothing when verbose is false", func(t *testing.T) {
		config := &Config{
			BaseURL:      "http://localhost:9000",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
			Verbose:      false,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		// Should not panic or output anything
		client.LogSSERequest("GET", "/api/v2/events", nil, nil, nil)
	})

	t.Run("logs when verbose is true", func(t *testing.T) {
		config := &Config{
			BaseURL:      "http://localhost:9000",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
			Verbose:      true,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		// Should not panic - verbose logging goes to stderr
		client.LogSSERequest("GET", "/api/v2/events",
			map[string]string{"user": "testuser"},
			map[string]string{"Accept": "text/event-stream"},
			nil)
	})
}

func TestLogSSEResponse(t *testing.T) {
	t.Run("does nothing when verbose is false", func(t *testing.T) {
		config := &Config{
			BaseURL:      "http://localhost:9000",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
			Verbose:      false,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		// Should not panic or output anything
		client.LogSSEResponse(200, "OK")
	})

	t.Run("logs when verbose is true", func(t *testing.T) {
		config := &Config{
			BaseURL:      "http://localhost:9000",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
			Verbose:      true,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		// Should not panic - verbose logging goes to stderr
		client.LogSSEResponse(200, "OK")
	})
}

func TestWatchEvents(t *testing.T) {
	t.Run("returns context error when cancelled before connection", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond) // Simulate slow response
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := &Config{
			BaseURL:      server.URL,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err = client.WatchEvents(ctx, EventsWatchRequest{}, func(event Event) error {
			return nil
		})

		assert.Equal(t, context.Canceled, err)
	})

	t.Run("handles context cancellation during streaming", func(t *testing.T) {
		eventsSent := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "SSE not supported", http.StatusInternalServerError)
				return
			}

			// Send events until client disconnects
			for i := 0; i < 10; i++ {
				select {
				case <-r.Context().Done():
					return
				default:
					w.Write([]byte("event: update\n"))
					w.Write([]byte("data: {\"active\":true}\n\n"))
					flusher.Flush()
					eventsSent++
					time.Sleep(50 * time.Millisecond)
				}
			}
		}))
		defer server.Close()

		config := &Config{
			BaseURL:      server.URL,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		eventsReceived := 0
		err = client.WatchEvents(ctx, EventsWatchRequest{}, func(event Event) error {
			eventsReceived++
			return nil
		})

		assert.Equal(t, context.DeadlineExceeded, err)
		assert.Greater(t, eventsReceived, 0)
	})

	t.Run("passes query parameters correctly", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "testuser", r.URL.Query().Get("user"))
			assert.Equal(t, "/production", r.URL.Query().Get("context"))
			assert.Equal(t, "feature1,feature2", r.URL.Query().Get("features"))
			assert.Equal(t, "proj1", r.URL.Query().Get("projects"))
			assert.Equal(t, "true", r.URL.Query().Get("conditions"))
			assert.Equal(t, "10", r.URL.Query().Get("refreshInterval"))
			assert.Equal(t, "30", r.URL.Query().Get("keepAliveInterval"))

			w.Header().Set("Content-Type", "text/event-stream")
			w.Write([]byte("event: init\n"))
			w.Write([]byte("data: {}\n\n"))
		}))
		defer server.Close()

		config := &Config{
			BaseURL:      server.URL,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		client.WatchEvents(ctx, EventsWatchRequest{
			User:              "testuser",
			Context:           "production",
			Features:          []string{"feature1", "feature2"},
			Projects:          []string{"proj1"},
			Conditions:        true,
			RefreshInterval:   10,
			KeepAliveInterval: 30,
		}, func(event Event) error {
			return nil
		})
	})
}

func TestFeatureCheckClient_handleError(t *testing.T) {
	t.Run("parses JSON error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Message: "Invalid feature ID"})
		}))
		defer server.Close()

		config := &Config{
			BaseURL:      server.URL,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		ctx := context.Background()
		_, err = CheckFeature(client, ctx, "invalid!", "", "", "", ParseFeatureCheckResult)

		assert.Error(t, err)
		apiErr, ok := err.(*APIError)
		require.True(t, ok)
		assert.Equal(t, 400, apiErr.StatusCode)
		assert.Equal(t, "Invalid feature ID", apiErr.Message)
	})

	t.Run("handles non-JSON error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		}))
		defer server.Close()

		config := &Config{
			BaseURL:      server.URL,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Timeout:      30,
		}

		client, err := NewFeatureCheckClient(config)
		require.NoError(t, err)

		ctx := context.Background()
		_, err = CheckFeature(client, ctx, "feature1", "", "", "", ParseFeatureCheckResult)

		assert.Error(t, err)
		apiErr, ok := err.(*APIError)
		require.True(t, ok)
		assert.Equal(t, 500, apiErr.StatusCode)
		assert.Contains(t, apiErr.Message, "Internal Server Error")
	})
}

func TestSSEParsing(t *testing.T) {
	t.Run("parseSSEField with valid field", func(t *testing.T) {
		field, value, ok := parseSSEField("data: {\"active\":true}")

		assert.True(t, ok)
		assert.Equal(t, "data", field)
		assert.Equal(t, "{\"active\":true}", value)
	})

	t.Run("parseSSEField with comment", func(t *testing.T) {
		_, _, ok := parseSSEField(": keep-alive")

		assert.False(t, ok)
	})

	t.Run("parseSSEField with no colon", func(t *testing.T) {
		_, _, ok := parseSSEField("invalid line")

		assert.False(t, ok)
	})

	t.Run("updateEventField sets id", func(t *testing.T) {
		event := &Event{}
		updateEventField(event, "id", "123")

		assert.Equal(t, "123", event.ID)
	})

	t.Run("updateEventField sets event type", func(t *testing.T) {
		event := &Event{}
		updateEventField(event, "event", "update")

		assert.Equal(t, "update", event.Type)
	})

	t.Run("updateEventField appends data with newline", func(t *testing.T) {
		event := &Event{Data: "line1"}
		updateEventField(event, "data", "line2")

		assert.Equal(t, "line1\nline2", event.Data)
	})

	t.Run("updateEventField returns retry duration", func(t *testing.T) {
		event := &Event{}
		duration := updateEventField(event, "retry", "5000")

		assert.Equal(t, 5*time.Second, duration)
	})

	t.Run("readSSELine with context", func(t *testing.T) {
		reader := bytes.NewReader([]byte("data: test\n"))
		bufReader := bufio.NewReader(reader)
		ctx := context.Background()

		line, err := readSSELine(ctx, bufReader)

		require.NoError(t, err)
		assert.Equal(t, "data: test", line)
	})
}
