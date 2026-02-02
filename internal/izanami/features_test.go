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

	config := &ResolvedConfig{
		LeaderURL: server.URL,
		Username:  "test-user",
		JwtToken:  "test-jwt-token",
		Timeout:   30,
	}

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	features, err := ListFeatures(client, ctx, "test-tenant", "", ParseFeatures)

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

	config := &ResolvedConfig{
		LeaderURL: server.URL,
		Username:  "test-user",
		JwtToken:  "test-jwt-token",
		Timeout:   30,
	}

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	feature, err := GetFeature(client, ctx, "test-tenant", "feature-1", ParseFeature)

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

	config := &ResolvedConfig{
		LeaderURL: server.URL,
		Username:  "test-user",
		JwtToken:  "test-jwt-token",
		Timeout:   30,
	}

	client, err := NewAdminClient(config)
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

	config := &ResolvedConfig{
		LeaderURL: server.URL,
		Username:  "test-user",
		JwtToken:  "test-jwt-token",
		Timeout:   30,
	}

	client, err := NewAdminClient(config)
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

	config := &ResolvedConfig{
		LeaderURL: server.URL,
		Username:  "test-user",
		JwtToken:  "test-jwt-token",
		Timeout:   30,
	}

	client, err := NewAdminClient(config)
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

	config := &ResolvedConfig{
		LeaderURL:    server.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Timeout:      30,
	}

	client, err := NewFeatureCheckClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	result, err := CheckFeature(client, ctx, "my-feature", "user123", "prod", "", ParseFeatureCheckResult)

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

	config := &ResolvedConfig{
		LeaderURL:    server.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Timeout:      30,
	}

	client, err := NewFeatureCheckClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	request := CheckFeaturesRequest{
		User:     "user123",
		Context:  "prod",
		Features: []string{"feature-1", "feature-2"},
	}
	result, err := CheckFeatures(client, ctx, request, ParseActivationsWithConditions)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)
	assert.True(t, result["feature-1"].Active.(bool))
	assert.False(t, result["feature-2"].Active.(bool))
}

func TestClient_PatchFeatures(t *testing.T) {
	patches := []FeaturePatch{
		{Op: "replace", Path: "/feature-1/enabled", Value: true},
		{Op: "replace", Path: "/feature-2/enabled", Value: false},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/features", r.URL.Path)
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Verify request body
		var body []FeaturePatch
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Len(t, body, 2)
		assert.Equal(t, "replace", body[0].Op)
		assert.Equal(t, "/feature-1/enabled", body[0].Path)

		w.WriteHeader(http.StatusNoContent)
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
	err = client.PatchFeatures(ctx, "test-tenant", patches)

	assert.NoError(t, err)
}

func TestClient_TestFeature(t *testing.T) {
	expectedResult := &FeatureTestResult{
		Name:    "feature-1",
		Active:  true,
		Project: "test-project",
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/features/feature-1/test", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		// Check query params
		user := r.URL.Query().Get("user")
		date := r.URL.Query().Get("date")
		assert.Equal(t, "user123", user)
		assert.NotEmpty(t, date)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResult)
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
	result, err := TestFeature(client, ctx, "test-tenant", "feature-1", "", "user123", "2025-01-01T00:00:00Z", "", ParseFeatureTestResult)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "feature-1", result.Name)
	assert.Equal(t, true, result.Active)
}

func TestClient_TestFeatureWithContext(t *testing.T) {
	expectedResult := &FeatureTestResult{
		Name:    "feature-1",
		Active:  false,
		Project: "test-project",
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Context path should be appended to the URL
		assert.Equal(t, "/api/admin/tenants/test-tenant/features/feature-1/test/prod/env", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResult)
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
	// Test with leading slash - should be stripped
	result, err := TestFeature(client, ctx, "test-tenant", "feature-1", "/prod/env", "", "2025-01-01T00:00:00Z", "", ParseFeatureTestResult)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "feature-1", result.Name)
	assert.Equal(t, false, result.Active)
}

func TestClient_TestFeatureDefinition(t *testing.T) {
	definition := map[string]interface{}{
		"name":    "test-feature",
		"enabled": true,
		"conditions": map[string]interface{}{
			"": map[string]interface{}{
				"conditions": []interface{}{},
			},
		},
	}

	expectedResult := &FeatureTestResult{
		Name:   "test-feature",
		Active: true,
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/test", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Check query params
		user := r.URL.Query().Get("user")
		date := r.URL.Query().Get("date")
		assert.Equal(t, "user123", user)
		assert.NotEmpty(t, date)

		// Verify request body - definition should be wrapped in "feature" key
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		feature, ok := body["feature"].(map[string]interface{})
		assert.True(t, ok, "body should have 'feature' key")
		assert.Equal(t, "test-feature", feature["name"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResult)
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
	result, err := TestFeatureDefinition(client, ctx, "test-tenant", "user123", "2025-01-01T00:00:00Z", definition, ParseFeatureTestResult)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-feature", result.Name)
	assert.Equal(t, true, result.Active)
}

func TestClient_TestFeaturesBulk(t *testing.T) {
	expectedResults := FeatureTestResults{
		"feature-1": {Name: "feature-1", Active: true, Project: "test-project"},
		"feature-2": {Name: "feature-2", Active: false, Project: "test-project"},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/features/_test", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		// Check query params
		user := r.URL.Query().Get("user")
		date := r.URL.Query().Get("date")
		context := r.URL.Query().Get("context")
		features := r.URL.Query()["features"]
		projects := r.URL.Query()["projects"]

		assert.Equal(t, "user123", user)
		assert.NotEmpty(t, date)
		assert.Equal(t, "/prod/env", context) // normalizeContextPath ensures leading slash
		assert.Contains(t, features, "feature-1")
		assert.Contains(t, features, "feature-2")
		assert.Contains(t, projects, "test-project")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResults)
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
	request := TestFeaturesAdminRequest{
		User:     "user123",
		Date:     "2025-01-01T00:00:00Z",
		Features: []string{"feature-1", "feature-2"},
		Projects: []string{"test-project"},
		Context:  "/prod/env", // with leading slash - should be normalized
	}
	result, err := TestFeaturesBulk(client, ctx, "test-tenant", request, ParseFeatureTestResults)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)
	assert.Equal(t, true, result["feature-1"].Active)
	assert.Equal(t, false, result["feature-2"].Active)
}
