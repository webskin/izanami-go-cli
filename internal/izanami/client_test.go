package izanami

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockServer creates a test HTTP server with predefined responses
func mockServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

func TestNewAdminClient(t *testing.T) {
	tests := []struct {
		name    string
		config  *ResolvedConfig
		wantErr bool
	}{
		{
			name: "valid client auth",
			config: &ResolvedConfig{
				LeaderURL: "http://localhost:9000",
				Username:  "test-user",
				JwtToken:  "test-jwt-token",
				Timeout:   30,
			},
			wantErr: false,
		},
		{
			name: "valid user auth",
			config: &ResolvedConfig{
				LeaderURL: "http://localhost:9000",
				Username:  "test-user",
				JwtToken:  "test-token",
				Timeout:   30,
			},
			wantErr: false,
		},
		{
			name: "missing base URL",
			config: &ResolvedConfig{
				Username: "test-user",
				JwtToken: "test-jwt-token",
			},
			wantErr: true,
		},
		{
			name: "missing auth",
			config: &ResolvedConfig{
				LeaderURL: "http://localhost:9000",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewAdminClient(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

func TestClient_ErrorHandling(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{
			Message: "Feature not found",
		})
	})
	defer server.Close()

	config := &ResolvedConfig{
		LeaderURL: server.URL,
		Username:  "test-user",
		JwtToken:  "test-jwt-token",
		Timeout:   30,
	}

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = GetFeature(client, ctx, "test-tenant", "nonexistent", ParseFeature)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
	assert.Contains(t, err.Error(), "Feature not found")
}

func TestClient_Login(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/login", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		// Check basic auth header
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "testuser", user)
		assert.Equal(t, "testpass", pass)

		// Set JWT token in cookie
		http.SetCookie(w, &http.Cookie{ // NOSONAR go:S2092 - Test cookie, no security concern
			Name:  "token",
			Value: "jwt-token-12345",
		})
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	config := &ResolvedConfig{
		LeaderURL: server.URL,
		Username:  "test-user",
		JwtToken:  "test-jwt-token",
		Timeout:   30,
	}

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	token, err := client.Login(ctx, "testuser", "testpass")

	assert.NoError(t, err)
	assert.Equal(t, "jwt-token-12345", token)
}

func Test_buildPath(t *testing.T) {
	tests := []struct {
		name     string
		segments []string
		want     string
	}{
		{
			name:     "simple path",
			segments: []string{"tenant", "features"},
			want:     "tenant/features",
		},
		{
			name:     "path with special chars",
			segments: []string{"tenant", "feature@1"},
			want:     "tenant/feature@1",
		},
		{
			name:     "path with empty segment",
			segments: []string{"tenant", "", "features"},
			want:     "tenant/features",
		},
		{
			name:     "path with spaces",
			segments: []string{"tenant", "my feature"},
			want:     "tenant/my%20feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPath(tt.segments...)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_truncateString(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		maxLength int
		want      string
	}{
		{
			name:      "short string",
			input:     "hello",
			maxLength: 10,
			want:      "hello",
		},
		{
			name:      "exact length",
			input:     "hello",
			maxLength: 5,
			want:      "hello",
		},
		{
			name:      "truncated string",
			input:     "hello world this is a long string",
			maxLength: 10,
			want:      "hello worl... [TRUNCATED: 23 more bytes]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateString(tt.input, tt.maxLength)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_Hooks(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(HealthStatus{
			Status:  "UP",
			Version: "1.0.0",
		})
	})
	defer server.Close()

	config := &ResolvedConfig{
		LeaderURL: server.URL,
		Username:  "test-user",
		JwtToken:  "test-jwt-token",
		Timeout:   30,
	}

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	// Test before request hook registration
	client.AddBeforeRequestHook(func(req *resty.Request) error {
		return nil
	})

	// Test after response hook registration
	client.AddAfterResponseHook(func(resp *resty.Response) error {
		return nil
	})

	ctx := context.Background()
	_, err = Health(client, ctx, ParseHealthStatus)
	require.NoError(t, err)

	// Note: Hooks are defined but not yet integrated into API methods
	// This test just verifies hooks can be registered without errors
}

func TestClient_StructuredLogger(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(HealthStatus{
			Status:  "UP",
			Version: "1.0.0",
		})
	})
	defer server.Close()

	config := &ResolvedConfig{
		LeaderURL: server.URL,
		Username:  "test-user",
		JwtToken:  "test-jwt-token",
		Timeout:   30,
		Verbose:   true,
	}

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	logCalled := false
	client.SetStructuredLogger(func(level, message string, fields map[string]interface{}) {
		logCalled = true
		assert.Contains(t, []string{"info", "warn", "error"}, level)
	})

	ctx := context.Background()
	_, err = Health(client, ctx, ParseHealthStatus)
	require.NoError(t, err)

	// With verbose mode enabled, structured logger should be called
	assert.True(t, logCalled)
}

func TestNewAdminClientNoAuth(t *testing.T) {
	t.Run("valid config without auth", func(t *testing.T) {
		config := &ResolvedConfig{
			LeaderURL: "http://localhost:9000",
			Timeout:   30,
		}

		client, err := NewAdminClientNoAuth(config)

		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("missing leader URL returns error", func(t *testing.T) {
		config := &ResolvedConfig{
			Timeout: 30,
		}

		client, err := NewAdminClientNoAuth(config)

		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "leader URL")
	})

	t.Run("can make health check without auth", func(t *testing.T) {
		server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
			// Verify no auth headers are required
			assert.Empty(t, r.Header.Get("Authorization"))
			assert.Empty(t, r.Header.Get("Cookie"))

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(HealthStatus{
				Database: true,
				Status:   "UP",
				Version:  "1.0.0",
			})
		})
		defer server.Close()

		config := &ResolvedConfig{
			LeaderURL: server.URL,
			Timeout:   30,
		}

		client, err := NewAdminClientNoAuth(config)
		require.NoError(t, err)

		ctx := context.Background()
		health, err := Health(client, ctx, ParseHealthStatus)

		require.NoError(t, err)
		assert.True(t, health.Database)
		assert.Equal(t, "UP", health.Status)
	})
}

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name       string
		apiError   APIError
		wantString string
	}{
		{
			name: "formats with status code and message",
			apiError: APIError{
				StatusCode: 404,
				Message:    "Feature not found",
				RawBody:    `{"message":"Feature not found"}`,
			},
			wantString: "API error (404): Feature not found",
		},
		{
			name: "handles 401 unauthorized",
			apiError: APIError{
				StatusCode: 401,
				Message:    "Invalid credentials",
				RawBody:    "",
			},
			wantString: "API error (401): Invalid credentials",
		},
		{
			name: "handles 500 server error",
			apiError: APIError{
				StatusCode: 500,
				Message:    "Internal server error",
				RawBody:    "Internal server error",
			},
			wantString: "API error (500): Internal server error",
		},
		{
			name: "handles empty message",
			apiError: APIError{
				StatusCode: 400,
				Message:    "",
				RawBody:    "",
			},
			wantString: "API error (400): ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.apiError.Error()
			assert.Equal(t, tt.wantString, got)
		})
	}
}

func TestAPIError_Interface(t *testing.T) {
	// Verify APIError implements error interface
	var err error = &APIError{
		StatusCode: 404,
		Message:    "Not found",
	}

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}
