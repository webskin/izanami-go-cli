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

func TestClient_CreateContext_WithParentPath(t *testing.T) {
	contextData := map[string]interface{}{
		"name": "sub-context",
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify slashes in parent path are NOT escaped
		assert.Equal(t, "/api/admin/tenants/test-tenant/contexts/parent/child", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

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
	err = client.CreateContext(ctx, "test-tenant", "", "sub-context", "parent/child", contextData)

	assert.NoError(t, err)
}

func TestClient_CreateContext_WithProject(t *testing.T) {
	contextData := map[string]interface{}{
		"name": "project-context",
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/projects/my-project/contexts", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

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
	err = client.CreateContext(ctx, "test-tenant", "my-project", "project-context", "", contextData)

	assert.NoError(t, err)
}

func TestClient_CreateContext_WithProjectAndParentPath(t *testing.T) {
	contextData := map[string]interface{}{
		"name": "nested-context",
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify slashes in parent path are NOT escaped for project contexts
		assert.Equal(t, "/api/admin/tenants/test-tenant/projects/my-project/contexts/prod/eu", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

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
	err = client.CreateContext(ctx, "test-tenant", "my-project", "nested-context", "prod/eu", contextData)

	assert.NoError(t, err)
}

func TestClient_UpdateContext(t *testing.T) {
	contextData := map[string]interface{}{
		"protected": true,
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/contexts/prod", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, true, body["protected"])

		w.WriteHeader(http.StatusNoContent)
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
	err = client.UpdateContext(ctx, "test-tenant", "prod", contextData)

	assert.NoError(t, err)
}

func TestClient_UpdateContext_WithNestedPath(t *testing.T) {
	contextData := map[string]interface{}{
		"protected": false,
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify slashes in context path are NOT escaped
		assert.Equal(t, "/api/admin/tenants/test-tenant/contexts/prod/eu/france", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)

		w.WriteHeader(http.StatusNoContent)
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
	err = client.UpdateContext(ctx, "test-tenant", "prod/eu/france", contextData)

	assert.NoError(t, err)
}

func TestClient_DeleteContext(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/contexts/test-context", r.URL.Path)
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
	err = client.DeleteContext(ctx, "test-tenant", "", "test-context")

	assert.NoError(t, err)
}

func TestClient_DeleteContext_WithNestedPath(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify slashes in context path are NOT escaped
		assert.Equal(t, "/api/admin/tenants/test-tenant/contexts/prod/eu/france", r.URL.Path)
		assert.Equal(t, "DELETE", r.Method)

		w.WriteHeader(http.StatusNoContent)
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
	err = client.DeleteContext(ctx, "test-tenant", "", "prod/eu/france")

	assert.NoError(t, err)
}

func TestClient_DeleteContext_WithProject(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/projects/my-project/contexts/staging", r.URL.Path)
		assert.Equal(t, "DELETE", r.Method)

		w.WriteHeader(http.StatusNoContent)
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
	err = client.DeleteContext(ctx, "test-tenant", "my-project", "staging")

	assert.NoError(t, err)
}

func TestClient_DeleteContext_WithProjectAndNestedPath(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify slashes in context path are NOT escaped for project contexts
		assert.Equal(t, "/api/admin/tenants/test-tenant/projects/my-project/contexts/prod/eu", r.URL.Path)
		assert.Equal(t, "DELETE", r.Method)

		w.WriteHeader(http.StatusNoContent)
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
	err = client.DeleteContext(ctx, "test-tenant", "my-project", "prod/eu")

	assert.NoError(t, err)
}
