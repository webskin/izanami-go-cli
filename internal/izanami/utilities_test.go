package izanami

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		Username: "test-user",
		JwtToken: "test-jwt-token",
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

func TestClient_Search(t *testing.T) {
	expectedResults := []SearchResult{
		{
			ID:   "result-1",
			Name: "Feature 1",
			Type: "feature",
		},
		{
			ID:   "result-2",
			Name: "Project 2",
			Type: "project",
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/search", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		// Check query params
		query := r.URL.Query().Get("query")
		filter := r.URL.Query().Get("filter")
		assert.Equal(t, "test", query)
		assert.Equal(t, "feature,project", filter)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResults)
	})
	defer server.Close()

	config := &Config{
		BaseURL:      server.URL,
		Username: "test-user",
		JwtToken: "test-jwt-token",
		Timeout:      30,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	results, err := client.Search(ctx, "test-tenant", "test", []string{"feature", "project"})

	assert.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "result-1", results[0].ID)
	assert.Equal(t, "result-2", results[1].ID)
}

func TestClient_Search_Global(t *testing.T) {
	expectedResults := []SearchResult{
		{
			ID:   "result-1",
			Name: "Global Feature",
			Type: "feature",
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/search", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResults)
	})
	defer server.Close()

	config := &Config{
		BaseURL:      server.URL,
		Username: "test-user",
		JwtToken: "test-jwt-token",
		Timeout:      30,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	results, err := client.Search(ctx, "", "test", nil)

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "result-1", results[0].ID)
}

func TestClient_Export(t *testing.T) {
	// NDJSON export format
	exportData := `{"type":"feature","data":{"id":"feature-1","name":"feature-1","enabled":true}}
{"type":"project","data":{"id":"project-1","name":"project-1"}}
`

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/_export", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "application/x-ndjson", r.Header.Get("Accept"))

		// Verify request body has export options
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, true, body["allProjects"])
		assert.Equal(t, true, body["allKeys"])

		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(exportData))
	})
	defer server.Close()

	config := &Config{
		BaseURL:      server.URL,
		Username: "test-user",
		JwtToken: "test-jwt-token",
		Timeout:      30,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	result, err := client.Export(ctx, "test-tenant")

	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "feature-1")
	assert.Contains(t, result, "project-1")
}

// Note: Import test requires file handling and is complex to mock properly
// Skipping for now as it's covered by integration tests
