package izanami

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_ListTenants(t *testing.T) {
	expectedTenants := []Tenant{
		{
			Name:        "tenant-1",
			Description: "Test tenant 1",
		},
		{
			Name:        "tenant-2",
			Description: "Test tenant 2",
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedTenants)
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
	tenants, err := client.ListTenants(ctx, nil)

	assert.NoError(t, err)
	assert.Len(t, tenants, 2)
	assert.Equal(t, "tenant-1", tenants[0].Name)
	assert.Equal(t, "tenant-2", tenants[1].Name)
}

func TestClient_GetTenant(t *testing.T) {
	expectedTenant := &Tenant{
		Name:        "test-tenant",
		Description: "Test tenant description",
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedTenant)
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
	tenant, err := client.GetTenant(ctx, "test-tenant")

	assert.NoError(t, err)
	assert.NotNil(t, tenant)
	assert.Equal(t, "test-tenant", tenant.Name)
	assert.Equal(t, "Test tenant description", tenant.Description)
}

func TestClient_CreateTenant(t *testing.T) {
	tenantData := map[string]interface{}{
		"name":        "new-tenant",
		"description": "New test tenant",
	}

	expectedTenant := &Tenant{
		Name:        "new-tenant",
		Description: "New test tenant",
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Decode and verify request body
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, "new-tenant", body["name"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(expectedTenant)
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
	err = client.CreateTenant(ctx, tenantData)

	assert.NoError(t, err)
}

func TestClient_UpdateTenant(t *testing.T) {
	tenantData := map[string]interface{}{
		"name":        "updated-tenant",
		"description": "Updated tenant description",
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Verify request body
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, "updated-tenant", body["name"])

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
	err = client.UpdateTenant(ctx, "test-tenant", tenantData)

	assert.NoError(t, err)
}

func TestClient_DeleteTenant(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant", r.URL.Path)
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
	err = client.DeleteTenant(ctx, "test-tenant")

	assert.NoError(t, err)
}
