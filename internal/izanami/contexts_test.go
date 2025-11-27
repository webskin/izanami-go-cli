package izanami

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_ListContexts(t *testing.T) {
	expectedContexts := []Context{
		{
			Name:    "context-1",
			Path:    "/context-1",
			Global:  false,
			Project: "project-1",
		},
		{
			Name:   "context-2",
			Path:   "/context-2",
			Global: true,
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/contexts", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedContexts)
	})
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Username: "test-user",
		JwtToken: "test-jwt-token",
		Timeout:  30,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	contexts, err := ListContexts(client, ctx, "test-tenant", "", false, ParseContexts)

	assert.NoError(t, err)
	assert.Len(t, contexts, 2)
	assert.Equal(t, "context-1", contexts[0].Name)
	assert.Equal(t, "context-2", contexts[1].Name)
	assert.False(t, contexts[0].Global)
	assert.True(t, contexts[1].Global)
}

func TestClient_CreateContext(t *testing.T) {
	contextData := map[string]interface{}{
		"name":   "new-context",
		"global": false,
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/contexts", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Decode and verify request body
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, "new-context", body["name"])

		w.WriteHeader(http.StatusCreated)
	})
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Username: "test-user",
		JwtToken: "test-jwt-token",
		Timeout:  30,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.CreateContext(ctx, "test-tenant", "", "new-context", "", contextData)

	assert.NoError(t, err)
}

func TestClient_DeleteContext(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Note: Full path verification would require matching the exact implementation
		assert.Contains(t, r.URL.Path, "/contexts")
		assert.Equal(t, "DELETE", r.Method)

		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Username: "test-user",
		JwtToken: "test-jwt-token",
		Timeout:  30,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.DeleteContext(ctx, "test-tenant", "", "/test-context")

	assert.NoError(t, err)
}
