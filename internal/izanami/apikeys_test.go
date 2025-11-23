package izanami

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_ListAPIKeys(t *testing.T) {
	expectedKeys := []APIKey{
		{
			ClientID:     "key-1",
			Name:         "Test Key 1",
			ClientSecret: "secret-1",
			Description:  "Test API key 1",
		},
		{
			ClientID:     "key-2",
			Name:         "Test Key 2",
			ClientSecret: "secret-2",
			Description:  "Test API key 2",
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/keys", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedKeys)
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
	keys, err := client.ListAPIKeys(ctx, "test-tenant")

	assert.NoError(t, err)
	assert.Len(t, keys, 2)
	assert.Equal(t, "key-1", keys[0].ClientID)
	assert.Equal(t, "key-2", keys[1].ClientID)
}

func TestClient_GetAPIKey(t *testing.T) {
	keys := []APIKey{
		{
			ClientID:     "other-key",
			Name:         "Other Key",
			ClientSecret: "secret1",
		},
		{
			ClientID:     "test-key",
			Name:         "Test Key",
			ClientSecret: "secret",
			Description:  "Test API key description",
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/keys", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(keys)
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
	key, err := client.GetAPIKey(ctx, "test-tenant", "test-key")

	assert.NoError(t, err)
	assert.NotNil(t, key)
	assert.Equal(t, "test-key", key.ClientID)
	assert.Equal(t, "Test Key", key.Name)
}

func TestClient_CreateAPIKey(t *testing.T) {
	keyData := map[string]interface{}{
		"name":        "new-key",
		"description": "New test API key",
	}

	expectedKey := &APIKey{
		ClientID:     "generated-key-id",
		Name:         "new-key",
		ClientSecret: "generated-secret",
		Description:  "New test API key",
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/keys", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Decode and verify request body
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, "new-key", body["name"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(expectedKey)
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
	key, err := client.CreateAPIKey(ctx, "test-tenant", keyData)

	assert.NoError(t, err)
	assert.NotNil(t, key)
	assert.Equal(t, "generated-key-id", key.ClientID)
	assert.Equal(t, "new-key", key.Name)
}

func TestClient_UpdateAPIKey(t *testing.T) {
	keyData := map[string]interface{}{
		"name":        "updated-key",
		"description": "Updated API key description",
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/keys/test-key", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Verify request body
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, "updated-key", body["name"])

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
	err = client.UpdateAPIKey(ctx, "test-tenant", "test-key", keyData)

	assert.NoError(t, err)
}

func TestClient_DeleteAPIKey(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/keys/test-key", r.URL.Path)
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
	err = client.DeleteAPIKey(ctx, "test-tenant", "test-key")

	assert.NoError(t, err)
}
