package izanami

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockServer creates a test HTTP server with predefined responses
func mockServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid client auth",
			config: &Config{
				BaseURL:      "http://localhost:9000",
				ClientID:     "test-client",
				ClientSecret: "test-secret",
				Timeout:      30,
			},
			wantErr: false,
		},
		{
			name: "valid user auth",
			config: &Config{
				BaseURL:  "http://localhost:9000",
				Username: "test-user",
				JwtToken: "test-token",
				Timeout:  30,
			},
			wantErr: false,
		},
		{
			name: "missing base URL",
			config: &Config{
				ClientID:     "test-client",
				ClientSecret: "test-secret",
			},
			wantErr: true,
		},
		{
			name: "missing auth",
			config: &Config{
				BaseURL: "http://localhost:9000",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config)
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

func TestClient_Health(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/_health", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(HealthStatus{
			Status:  "UP",
			Version: "1.0.0",
		})
	})
	defer server.Close()

	config := &Config{
		BaseURL:      server.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Timeout:      30,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	health, err := client.Health(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, health)
	assert.Equal(t, "UP", health.Status)
	assert.Equal(t, "1.0.0", health.Version)
}

func TestClient_ListFeatures(t *testing.T) {
	expectedFeatures := []Feature{
		{
			ID:          "feature-1",
			Name:        "feature-1",
			Description: "Test feature 1",
			Project:     "test-project",
			Enabled:     true,
			Tags:        []string{"test"},
		},
		{
			ID:          "feature-2",
			Name:        "feature-2",
			Description: "Test feature 2",
			Project:     "test-project",
			Enabled:     false,
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/features", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		// Note: project filtering is not supported by Izanami API
		// Only tag filtering is supported via query params

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedFeatures)
	})
	defer server.Close()

	config := &Config{
		BaseURL:      server.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Timeout:      30,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	features, err := client.ListFeatures(ctx, "test-tenant", "")

	assert.NoError(t, err)
	assert.Len(t, features, 2)
	assert.Equal(t, "feature-1", features[0].ID)
	assert.Equal(t, "feature-2", features[1].ID)
}

func TestClient_GetFeature(t *testing.T) {
	expectedFeature := &FeatureWithOverloads{
		ID:      "feature-1",
		Name:    "feature-1",
		Project: "test-project",
		Enabled: true,
		Tags:    []string{"test"},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/features/feature-1", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedFeature)
	})
	defer server.Close()

	config := &Config{
		BaseURL:      server.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Timeout:      30,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	feature, err := client.GetFeature(ctx, "test-tenant", "feature-1")

	assert.NoError(t, err)
	assert.NotNil(t, feature)
	assert.Equal(t, "feature-1", feature.ID)
	assert.Equal(t, "test-project", feature.Project)
	assert.True(t, feature.Enabled)
}

func TestClient_CreateFeature(t *testing.T) {
	featureData := map[string]interface{}{
		"name":        "new-feature",
		"description": "New test feature",
		"enabled":     true,
	}

	expectedFeature := &Feature{
		ID:          "new-feature",
		Name:        "new-feature",
		Description: "New test feature",
		Project:     "test-project",
		Enabled:     true,
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/projects/test-project/features", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Decode and verify request body
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, "new-feature", body["name"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(expectedFeature)
	})
	defer server.Close()

	config := &Config{
		BaseURL:      server.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Timeout:      30,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	feature, err := client.CreateFeature(ctx, "test-tenant", "test-project", featureData)

	assert.NoError(t, err)
	assert.NotNil(t, feature)
	assert.Equal(t, "new-feature", feature.ID)
}

func TestClient_CheckFeature(t *testing.T) {
	expectedResult := &FeatureCheckResult{
		Active:  true,
		Name:    "my-feature",
		Project: "test-project",
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/features/my-feature", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		// Check query params
		user := r.URL.Query().Get("user")
		context := r.URL.Query().Get("context")
		assert.Equal(t, "user123", user)
		assert.Equal(t, "prod", context)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResult)
	})
	defer server.Close()

	config := &Config{
		BaseURL:      server.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Timeout:      30,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	result, err := client.CheckFeature(ctx, "my-feature", "user123", "prod")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, true, result.Active)
	assert.Equal(t, "my-feature", result.Name)
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

	config := &Config{
		BaseURL:      server.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Timeout:      30,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.GetFeature(ctx, "test-tenant", "nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
	assert.Contains(t, err.Error(), "Feature not found")
}
