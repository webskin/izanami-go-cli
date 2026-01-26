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

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	tenants, err := ListTenants(client, ctx, nil, ParseTenants)

	assert.NoError(t, err)
	assert.Len(t, tenants, 2)
	assert.Equal(t, "tenant-1", tenants[0].Name)
	assert.Equal(t, "tenant-2", tenants[1].Name)
}

func TestClient_ListTenants_WithRightFilter(t *testing.T) {
	expectedTenants := []Tenant{
		{
			Name:        "admin-tenant",
			Description: "Tenant with admin rights",
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "Admin", r.URL.Query().Get("right"))

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

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	rightLevel := RightLevelAdmin
	tenants, err := ListTenants(client, ctx, &rightLevel, ParseTenants)

	assert.NoError(t, err)
	assert.Len(t, tenants, 1)
	assert.Equal(t, "admin-tenant", tenants[0].Name)
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

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	tenant, err := GetTenant(client, ctx, "test-tenant", ParseTenant)

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

	client, err := NewAdminClient(config)
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

	client, err := NewAdminClient(config)
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

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.DeleteTenant(ctx, "test-tenant")

	assert.NoError(t, err)
}

func TestClient_DeleteTenant_NoContent(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant", r.URL.Path)
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

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.DeleteTenant(ctx, "test-tenant")

	assert.NoError(t, err)
}

func TestClient_UpdateTenant_NoContent(t *testing.T) {
	tenantData := map[string]interface{}{
		"name":        "updated-tenant",
		"description": "Updated tenant description",
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant", r.URL.Path)
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

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.UpdateTenant(ctx, "test-tenant", tenantData)

	assert.NoError(t, err)
}

func TestClient_ListTenantLogs(t *testing.T) {
	expectedEvents := []AuditEvent{
		{
			EventID: 1,
			ID:      "feature-1",
			Name:    "Feature 1",
			Tenant:  "test-tenant",
			Type:    "FEATURE_UPDATED",
			User:    "admin",
		},
		{
			EventID: 2,
			ID:      "feature-2",
			Name:    "Feature 2",
			Tenant:  "test-tenant",
			Type:    "FEATURE_CREATED",
			User:    "admin",
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/logs", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedEvents)
	})
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Username: "test-user",
		JwtToken: "test-jwt-token",
		Timeout:  30,
	}

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	events, err := ListTenantLogs(client, ctx, "test-tenant", nil, ParseAuditEvents)

	assert.NoError(t, err)
	assert.Len(t, events, 2)
	assert.Equal(t, int64(1), events[0].EventID)
	assert.Equal(t, "FEATURE_UPDATED", events[0].Type)
}

func TestClient_ListTenantLogs_WithFilters(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/logs", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		// Check query params
		assert.Equal(t, "desc", r.URL.Query().Get("order"))
		assert.Equal(t, "admin,user1", r.URL.Query().Get("users"))
		assert.Equal(t, "FEATURE_CREATED,FEATURE_UPDATED", r.URL.Query().Get("types"))
		assert.Equal(t, "feature-1", r.URL.Query().Get("features"))
		assert.Equal(t, "project-1", r.URL.Query().Get("projects"))
		assert.Equal(t, "2024-01-01T00:00:00Z", r.URL.Query().Get("start"))
		assert.Equal(t, "2024-12-31T23:59:59Z", r.URL.Query().Get("end"))
		assert.Equal(t, "100", r.URL.Query().Get("cursor"))
		assert.Equal(t, "50", r.URL.Query().Get("count"))
		assert.Equal(t, "true", r.URL.Query().Get("total"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]AuditEvent{})
	})
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Username: "test-user",
		JwtToken: "test-jwt-token",
		Timeout:  30,
	}

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	opts := &LogsRequest{
		Order:    "desc",
		Users:    "admin,user1",
		Types:    "FEATURE_CREATED,FEATURE_UPDATED",
		Features: "feature-1",
		Projects: "project-1",
		Start:    "2024-01-01T00:00:00Z",
		End:      "2024-12-31T23:59:59Z",
		Cursor:   100,
		Count:    50,
		Total:    true,
	}
	events, err := ListTenantLogs(client, ctx, "test-tenant", opts, ParseAuditEvents)

	assert.NoError(t, err)
	assert.Len(t, events, 0)
}

func TestClient_ListTenants_Identity(t *testing.T) {
	expectedJSON := `[{"name":"tenant-1"},{"name":"tenant-2"}]`

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(expectedJSON))
	})
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Username: "test-user",
		JwtToken: "test-jwt-token",
		Timeout:  30,
	}

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	raw, err := ListTenants(client, ctx, nil, Identity)

	assert.NoError(t, err)
	assert.Equal(t, expectedJSON, string(raw))
}

func TestClient_GetTenant_Identity(t *testing.T) {
	expectedJSON := `{"name":"test-tenant","description":"Test description"}`

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(expectedJSON))
	})
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Username: "test-user",
		JwtToken: "test-jwt-token",
		Timeout:  30,
	}

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	raw, err := GetTenant(client, ctx, "test-tenant", Identity)

	assert.NoError(t, err)
	assert.Equal(t, expectedJSON, string(raw))
}

func TestClient_ListTenants_Error(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "Access denied"})
	})
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Username: "test-user",
		JwtToken: "test-jwt-token",
		Timeout:  30,
	}

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = ListTenants(client, ctx, nil, ParseTenants)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestClient_GetTenant_NotFound(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "Tenant not found"})
	})
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Username: "test-user",
		JwtToken: "test-jwt-token",
		Timeout:  30,
	}

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = GetTenant(client, ctx, "nonexistent", ParseTenant)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
	assert.Contains(t, err.Error(), "Tenant not found")
}

func TestClient_CreateTenant_Error(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "Tenant already exists"})
	})
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Username: "test-user",
		JwtToken: "test-jwt-token",
		Timeout:  30,
	}

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.CreateTenant(ctx, map[string]interface{}{"name": "existing-tenant"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "409")
}

func TestClient_UpdateTenant_NotFound(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "Tenant not found"})
	})
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Username: "test-user",
		JwtToken: "test-jwt-token",
		Timeout:  30,
	}

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.UpdateTenant(ctx, "nonexistent", map[string]interface{}{"name": "updated"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestClient_DeleteTenant_NotFound(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "Tenant not found"})
	})
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Username: "test-user",
		JwtToken: "test-jwt-token",
		Timeout:  30,
	}

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.DeleteTenant(ctx, "nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestClient_CreateTenant_WithTenantStruct(t *testing.T) {
	tenant := &Tenant{
		Name:        "struct-tenant",
		Description: "Created from struct",
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		var body Tenant
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, "struct-tenant", body.Name)
		assert.Equal(t, "Created from struct", body.Description)

		w.WriteHeader(http.StatusCreated)
	})
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Username: "test-user",
		JwtToken: "test-jwt-token",
		Timeout:  30,
	}

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.CreateTenant(ctx, tenant)

	assert.NoError(t, err)
}
