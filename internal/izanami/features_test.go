package izanami

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		BaseURL:  server.URL,
		Username: "test-user",
		JwtToken: "test-jwt-token",
		Timeout:  30,
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
		BaseURL:  server.URL,
		Username: "test-user",
		JwtToken: "test-jwt-token",
		Timeout:  30,
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
		BaseURL:  server.URL,
		Username: "test-user",
		JwtToken: "test-jwt-token",
		Timeout:  30,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	feature, err := client.CreateFeature(ctx, "test-tenant", "test-project", featureData)

	assert.NoError(t, err)
	assert.NotNil(t, feature)
	assert.Equal(t, "new-feature", feature.ID)
}

func TestClient_UpdateFeature(t *testing.T) {
	featureData := map[string]interface{}{
		"name":        "updated-feature",
		"description": "Updated test feature",
		"enabled":     false,
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/features/feature-1", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Verify request body
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, "updated-feature", body["name"])

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
	err = client.UpdateFeature(ctx, "test-tenant", "feature-1", featureData, false)

	assert.NoError(t, err)
}

func TestClient_DeleteFeature(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/features/feature-1", r.URL.Path)
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
	err = client.DeleteFeature(ctx, "test-tenant", "feature-1")

	assert.NoError(t, err)
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
		assert.Equal(t, "/prod", context) // context paths are normalized to have leading slash

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
	result, err := client.CheckFeature(ctx, "my-feature", "user123", "prod", "")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, true, result.Active)
	assert.Equal(t, "my-feature", result.Name)
}

func TestClient_CheckFeatures(t *testing.T) {
	expectedResult := ActivationsWithConditions{
		"feature-1": {Active: true, Name: "feature-1"},
		"feature-2": {Active: false, Name: "feature-2"},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/features", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		// Check query params
		user := r.URL.Query().Get("user")
		context := r.URL.Query().Get("context")
		features := r.URL.Query().Get("features")
		assert.Equal(t, "user123", user)
		assert.Equal(t, "/prod", context)
		assert.Equal(t, "feature-1,feature-2", features)

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
	request := CheckFeaturesRequest{
		User:     "user123",
		Context:  "prod",
		Features: []string{"feature-1", "feature-2"},
	}
	result, err := client.CheckFeatures(ctx, request)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)
	assert.True(t, result["feature-1"].Active.(bool))
	assert.False(t, result["feature-2"].Active.(bool))
}
