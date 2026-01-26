package izanami

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListWebhooks(t *testing.T) {
	expectedWebhooks := []WebhookFull{
		{
			ID:      "webhook-1",
			Name:    "Test Webhook 1",
			URL:     "https://example.com/hook1",
			Enabled: true,
			Global:  true,
		},
		{
			ID:      "webhook-2",
			Name:    "Test Webhook 2",
			URL:     "https://example.com/hook2",
			Enabled: false,
			Global:  false,
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/webhooks", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedWebhooks)
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
	webhooks, err := ListWebhooks(client, ctx, "test-tenant", ParseWebhooks)

	assert.NoError(t, err)
	assert.Len(t, webhooks, 2)
	assert.Equal(t, "webhook-1", webhooks[0].ID)
	assert.Equal(t, "Test Webhook 1", webhooks[0].Name)
	assert.Equal(t, "webhook-2", webhooks[1].ID)
}

func TestListWebhooks_WithIdentityMapper(t *testing.T) {
	expectedResponse := `[{"id":"webhook-1","name":"Test Webhook"}]`

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/webhooks", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(expectedResponse))
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
	raw, err := ListWebhooks(client, ctx, "test-tenant", Identity)

	assert.NoError(t, err)
	assert.Equal(t, expectedResponse, string(raw))
}

func TestListWebhooks_ServerError(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "Internal server error"})
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
	_, err = ListWebhooks(client, ctx, "test-tenant", ParseWebhooks)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestListWebhooks_Empty(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]WebhookFull{})
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
	webhooks, err := ListWebhooks(client, ctx, "test-tenant", ParseWebhooks)

	assert.NoError(t, err)
	assert.Empty(t, webhooks)
}

func TestClient_CreateWebhook(t *testing.T) {
	webhookData := map[string]interface{}{
		"name":    "new-webhook",
		"url":     "https://example.com/hook",
		"enabled": true,
		"global":  true,
	}

	expectedWebhook := &WebhookFull{
		ID:      "generated-webhook-id",
		Name:    "new-webhook",
		URL:     "https://example.com/hook",
		Enabled: true,
		Global:  true,
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/webhooks", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Decode and verify request body
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, "new-webhook", body["name"])
		assert.Equal(t, "https://example.com/hook", body["url"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(expectedWebhook)
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
	webhook, err := client.CreateWebhook(ctx, "test-tenant", webhookData)

	assert.NoError(t, err)
	assert.NotNil(t, webhook)
	assert.Equal(t, "generated-webhook-id", webhook.ID)
	assert.Equal(t, "new-webhook", webhook.Name)
}

func TestClient_CreateWebhook_WithStatusOK(t *testing.T) {
	// Some servers may return 200 instead of 201
	expectedWebhook := &WebhookFull{
		ID:   "webhook-id",
		Name: "webhook",
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(expectedWebhook)
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
	webhook, err := client.CreateWebhook(ctx, "test-tenant", map[string]interface{}{})

	assert.NoError(t, err)
	assert.NotNil(t, webhook)
}

func TestClient_CreateWebhook_BadRequest(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Webhook must either be global or specify features or projects",
		})
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
	webhook, err := client.CreateWebhook(ctx, "test-tenant", map[string]interface{}{
		"name": "invalid-webhook",
		"url":  "https://example.com",
	})

	assert.Error(t, err)
	assert.Nil(t, webhook)
	assert.Contains(t, err.Error(), "400")
}

func TestClient_UpdateWebhook(t *testing.T) {
	webhookData := map[string]interface{}{
		"name":    "updated-webhook",
		"url":     "https://example.com/updated",
		"enabled": true,
		"global":  true,
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/webhooks/webhook-id", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Verify request body
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, "updated-webhook", body["name"])

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
	err = client.UpdateWebhook(ctx, "test-tenant", "webhook-id", webhookData)

	assert.NoError(t, err)
}

func TestClient_UpdateWebhook_WithStatusOK(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
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
	err = client.UpdateWebhook(ctx, "test-tenant", "webhook-id", map[string]interface{}{})

	assert.NoError(t, err)
}

func TestClient_UpdateWebhook_NotFound(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"message": "Webhook not found"})
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
	err = client.UpdateWebhook(ctx, "test-tenant", "nonexistent", map[string]interface{}{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestClient_UpdateWebhook_BadRequest(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Bad body request and / or bad uuid provided as id",
		})
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
	err = client.UpdateWebhook(ctx, "test-tenant", "webhook-id", map[string]interface{}{
		"enabled": true,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestClient_DeleteWebhook(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/webhooks/webhook-id", r.URL.Path)
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
	err = client.DeleteWebhook(ctx, "test-tenant", "webhook-id")

	assert.NoError(t, err)
}

func TestClient_DeleteWebhook_WithStatusOK(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
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
	err = client.DeleteWebhook(ctx, "test-tenant", "webhook-id")

	assert.NoError(t, err)
}

func TestClient_DeleteWebhook_NotFound(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"message": "Webhook not found"})
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
	err = client.DeleteWebhook(ctx, "test-tenant", "nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestListWebhookUsers(t *testing.T) {
	expectedUsers := []UserWithWebhookRight{
		{
			Username:    "user1",
			Email:       "user1@example.com",
			UserType:    "INTERNAL",
			Admin:       true,
			TenantAdmin: true,
			Right:       "Admin",
		},
		{
			Username:    "user2",
			Email:       "user2@example.com",
			UserType:    "INTERNAL",
			Admin:       false,
			TenantAdmin: false,
			Right:       "Read",
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/webhooks/webhook-id/users", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedUsers)
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
	users, err := ListWebhookUsers(client, ctx, "test-tenant", "webhook-id", ParseWebhookUsers)

	assert.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Equal(t, "user1", users[0].Username)
	assert.Equal(t, "Admin", users[0].Right)
	assert.Equal(t, "user2", users[1].Username)
	assert.Equal(t, "Read", users[1].Right)
}

func TestListWebhookUsers_WithIdentityMapper(t *testing.T) {
	expectedResponse := `[{"username":"user1","right":"Admin"}]`

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/webhooks/webhook-id/users", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(expectedResponse))
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
	raw, err := ListWebhookUsers(client, ctx, "test-tenant", "webhook-id", Identity)

	assert.NoError(t, err)
	assert.Equal(t, expectedResponse, string(raw))
}

func TestListWebhookUsers_Empty(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]UserWithWebhookRight{})
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
	users, err := ListWebhookUsers(client, ctx, "test-tenant", "webhook-id", ParseWebhookUsers)

	assert.NoError(t, err)
	assert.Empty(t, users)
}

func TestListWebhookUsers_NotFound(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"message": "Webhook not found"})
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
	_, err = ListWebhookUsers(client, ctx, "test-tenant", "nonexistent", ParseWebhookUsers)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestListWebhookUsers_Unauthorized(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"message": "Invalid token"})
	})
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Username: "test-user",
		JwtToken: "invalid-token",
		Timeout:  30,
	}

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = ListWebhookUsers(client, ctx, "test-tenant", "webhook-id", ParseWebhookUsers)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestClient_CreateWebhook_WithFeatures(t *testing.T) {
	webhookData := map[string]interface{}{
		"name":     "feature-webhook",
		"url":      "https://example.com/hook",
		"enabled":  true,
		"global":   false,
		"features": []string{"feature-1", "feature-2"},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		features, ok := body["features"].([]interface{})
		assert.True(t, ok)
		assert.Len(t, features, 2)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(&WebhookFull{ID: "webhook-id"})
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
	webhook, err := client.CreateWebhook(ctx, "test-tenant", webhookData)

	assert.NoError(t, err)
	assert.NotNil(t, webhook)
}

func TestClient_CreateWebhook_WithProjects(t *testing.T) {
	webhookData := map[string]interface{}{
		"name":     "project-webhook",
		"url":      "https://example.com/hook",
		"enabled":  true,
		"global":   false,
		"projects": []string{"project-1"},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		projects, ok := body["projects"].([]interface{})
		assert.True(t, ok)
		assert.Len(t, projects, 1)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(&WebhookFull{ID: "webhook-id"})
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
	webhook, err := client.CreateWebhook(ctx, "test-tenant", webhookData)

	assert.NoError(t, err)
	assert.NotNil(t, webhook)
}

func TestClient_CreateWebhook_WithHeaders(t *testing.T) {
	webhookData := map[string]interface{}{
		"name":    "header-webhook",
		"url":     "https://example.com/hook",
		"enabled": true,
		"global":  true,
		"headers": map[string]string{
			"Authorization": "Bearer token",
			"X-Custom":      "value",
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		headers, ok := body["headers"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "Bearer token", headers["Authorization"])
		assert.Equal(t, "value", headers["X-Custom"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(&WebhookFull{ID: "webhook-id"})
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
	webhook, err := client.CreateWebhook(ctx, "test-tenant", webhookData)

	assert.NoError(t, err)
	assert.NotNil(t, webhook)
}
