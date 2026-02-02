package izanami

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_SetOverload(t *testing.T) {
	strategy := map[string]interface{}{
		"enabled":    true,
		"resultType": "boolean",
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/projects/test-project/contexts/PROD/features/my-feature", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Verify body
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, true, body["enabled"])
		assert.Equal(t, "boolean", body["resultType"])

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
	err = client.SetOverload(ctx, "test-tenant", "test-project", "PROD", "my-feature", strategy, false)

	assert.NoError(t, err)
}

func TestClient_SetOverload_NestedContext(t *testing.T) {
	strategy := map[string]interface{}{
		"enabled":    true,
		"resultType": "boolean",
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/projects/test-project/contexts/PROD/mobile/EU/features/my-feature", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)

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
	err = client.SetOverload(ctx, "test-tenant", "test-project", "PROD/mobile/EU", "my-feature", strategy, false)

	assert.NoError(t, err)
}

func TestClient_SetOverload_WithPreserveProtected(t *testing.T) {
	strategy := map[string]interface{}{
		"enabled":    true,
		"resultType": "boolean",
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/projects/test-project/contexts/PROD/features/my-feature", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "true", r.URL.Query().Get("preserveProtectedContexts"))

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
	err = client.SetOverload(ctx, "test-tenant", "test-project", "PROD", "my-feature", strategy, true)

	assert.NoError(t, err)
}

func TestClient_SetOverload_WithConditions(t *testing.T) {
	strategy := map[string]interface{}{
		"enabled":    true,
		"resultType": "boolean",
		"conditions": []map[string]interface{}{
			{
				"rule": map[string]interface{}{
					"type":  "UserList",
					"users": []string{"alice", "bob"},
				},
			},
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)

		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, true, body["enabled"])
		assert.NotNil(t, body["conditions"])

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
	err = client.SetOverload(ctx, "test-tenant", "test-project", "PROD", "my-feature", strategy, false)

	assert.NoError(t, err)
}

func TestClient_SetOverload_Error(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"message": "invalid strategy"})
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
	err = client.SetOverload(ctx, "test-tenant", "test-project", "PROD", "my-feature", nil, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid strategy")
}

func TestClient_SetOverload_FeatureNameWithSlash(t *testing.T) {
	strategy := map[string]interface{}{
		"enabled":    true,
		"resultType": "boolean",
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Feature name "feature/with/slashes" should be URL-encoded
		// url.PathEscape encodes "/" as "%2F"
		assert.Equal(t, "PUT", r.Method)
		// RawPath contains the encoded version
		assert.Contains(t, r.URL.RawPath, "my%2Ffeature")

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
	err = client.SetOverload(ctx, "test-tenant", "test-project", "PROD", "my/feature", strategy, false)

	assert.NoError(t, err)
}

func TestClient_DeleteOverload(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/projects/test-project/contexts/PROD/features/my-feature", r.URL.Path)
		assert.Equal(t, "DELETE", r.Method)

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
	err = client.DeleteOverload(ctx, "test-tenant", "test-project", "PROD", "my-feature", false)

	assert.NoError(t, err)
}

func TestClient_DeleteOverload_NestedContext(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/projects/test-project/contexts/PROD/mobile/features/my-feature", r.URL.Path)
		assert.Equal(t, "DELETE", r.Method)

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
	err = client.DeleteOverload(ctx, "test-tenant", "test-project", "PROD/mobile", "my-feature", false)

	assert.NoError(t, err)
}

func TestClient_DeleteOverload_WithPreserveProtected(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "true", r.URL.Query().Get("preserveProtectedContexts"))

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
	err = client.DeleteOverload(ctx, "test-tenant", "test-project", "PROD", "my-feature", true)

	assert.NoError(t, err)
}

func TestClient_DeleteOverload_NotFound(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"message": "overload not found"})
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
	err = client.DeleteOverload(ctx, "test-tenant", "test-project", "PROD", "my-feature", false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetOverload(t *testing.T) {
	// Mock context tree with overloads
	contextTree := []Context{
		{
			Name:    "PROD",
			Project: "test-project",
			Overloads: []FeatureOverload{
				{
					ID:         "feature-1",
					Name:       "my-feature",
					Project:    "test-project",
					Enabled:    true,
					ResultType: "boolean",
				},
			},
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/projects/test-project/contexts", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "true", r.URL.Query().Get("all"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(contextTree)
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
	raw, err := GetOverload(client, ctx, "test-tenant", "test-project", "my-feature", "PROD", Identity)

	require.NoError(t, err)
	assert.NotEmpty(t, raw)

	// Verify the returned overload
	var overload FeatureOverload
	err = json.Unmarshal(raw, &overload)
	require.NoError(t, err)
	assert.Equal(t, "my-feature", overload.Name)
	assert.True(t, overload.Enabled)
}

func TestGetOverload_NestedContext(t *testing.T) {
	// Mock context tree with nested overloads
	contextTree := []Context{
		{
			Name:    "PROD",
			Project: "test-project",
			Children: []*Context{
				{
					Name:    "mobile",
					Project: "test-project",
					Overloads: []FeatureOverload{
						{
							ID:         "feature-1",
							Name:       "my-feature",
							Project:    "test-project",
							Enabled:    false,
							ResultType: "boolean",
						},
					},
				},
			},
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(contextTree)
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
	raw, err := GetOverload(client, ctx, "test-tenant", "test-project", "my-feature", "PROD/mobile", Identity)

	require.NoError(t, err)
	assert.NotEmpty(t, raw)

	var overload FeatureOverload
	err = json.Unmarshal(raw, &overload)
	require.NoError(t, err)
	assert.Equal(t, "my-feature", overload.Name)
	assert.False(t, overload.Enabled)
}

func TestGetOverload_NotFound(t *testing.T) {
	// Mock context tree without the requested feature
	contextTree := []Context{
		{
			Name:      "PROD",
			Project:   "test-project",
			Overloads: []FeatureOverload{},
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(contextTree)
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
	_, err = GetOverload(client, ctx, "test-tenant", "test-project", "nonexistent-feature", "PROD", Identity)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no overload found")
}

func TestGetOverload_ContextNotFound(t *testing.T) {
	// Mock context tree without the requested context
	contextTree := []Context{
		{
			Name:    "DEV",
			Project: "test-project",
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(contextTree)
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
	_, err = GetOverload(client, ctx, "test-tenant", "test-project", "my-feature", "PROD", Identity)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no overload found")
}

func TestGetOverload_EmptyContextTree(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Context{})
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
	_, err = GetOverload(client, ctx, "test-tenant", "test-project", "my-feature", "PROD", Identity)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no overload found")
}

func TestGetOverload_DeeplyNestedContext(t *testing.T) {
	// Mock deeply nested context tree
	contextTree := []Context{
		{
			Name:    "PROD",
			Project: "test-project",
			Children: []*Context{
				{
					Name:    "mobile",
					Project: "test-project",
					Children: []*Context{
						{
							Name:    "EU",
							Project: "test-project",
							Overloads: []FeatureOverload{
								{
									ID:         "feature-1",
									Name:       "my-feature",
									Project:    "test-project",
									Enabled:    true,
									ResultType: "boolean",
								},
							},
						},
					},
				},
			},
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(contextTree)
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
	raw, err := GetOverload(client, ctx, "test-tenant", "test-project", "my-feature", "PROD/mobile/EU", Identity)

	require.NoError(t, err)
	assert.NotEmpty(t, raw)

	var overload FeatureOverload
	err = json.Unmarshal(raw, &overload)
	require.NoError(t, err)
	assert.Equal(t, "my-feature", overload.Name)
}

func Test_findOverloadInContextTree(t *testing.T) {
	tests := []struct {
		name        string
		contexts    []Context
		targetPath  string
		featureName string
		want        *FeatureOverload
	}{
		{
			name: "finds overload in root context",
			contexts: []Context{
				{
					Name: "PROD",
					Overloads: []FeatureOverload{
						{Name: "feature-a", Enabled: true},
						{Name: "feature-b", Enabled: false},
					},
				},
			},
			targetPath:  "PROD",
			featureName: "feature-a",
			want:        &FeatureOverload{Name: "feature-a", Enabled: true},
		},
		{
			name: "finds overload in nested context",
			contexts: []Context{
				{
					Name: "PROD",
					Children: []*Context{
						{
							Name: "mobile",
							Overloads: []FeatureOverload{
								{Name: "feature-a", Enabled: true},
							},
						},
					},
				},
			},
			targetPath:  "PROD/mobile",
			featureName: "feature-a",
			want:        &FeatureOverload{Name: "feature-a", Enabled: true},
		},
		{
			name: "returns nil when context not found",
			contexts: []Context{
				{
					Name: "DEV",
				},
			},
			targetPath:  "PROD",
			featureName: "feature-a",
			want:        nil,
		},
		{
			name: "returns nil when feature not in context",
			contexts: []Context{
				{
					Name: "PROD",
					Overloads: []FeatureOverload{
						{Name: "feature-b", Enabled: false},
					},
				},
			},
			targetPath:  "PROD",
			featureName: "feature-a",
			want:        nil,
		},
		{
			name:        "returns nil for empty contexts",
			contexts:    []Context{},
			targetPath:  "PROD",
			featureName: "feature-a",
			want:        nil,
		},
		{
			name: "handles nil children",
			contexts: []Context{
				{
					Name:     "PROD",
					Children: nil,
				},
			},
			targetPath:  "PROD/mobile",
			featureName: "feature-a",
			want:        nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findOverloadInContextTree(tt.contexts, tt.targetPath, tt.featureName, "")
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, tt.want.Name, got.Name)
				assert.Equal(t, tt.want.Enabled, got.Enabled)
			}
		})
	}
}

func Test_contextsToSlice(t *testing.T) {
	tests := []struct {
		name string
		ptrs []*Context
		want []Context
	}{
		{
			name: "converts non-nil pointers",
			ptrs: []*Context{
				{Name: "PROD"},
				{Name: "DEV"},
			},
			want: []Context{
				{Name: "PROD"},
				{Name: "DEV"},
			},
		},
		{
			name: "handles empty slice",
			ptrs: []*Context{},
			want: []Context{},
		},
		{
			name: "handles nil pointers in slice",
			ptrs: []*Context{
				{Name: "PROD"},
				nil,
				{Name: "DEV"},
			},
			want: []Context{
				{Name: "PROD"},
				{},
				{Name: "DEV"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contextsToSlice(tt.ptrs)
			assert.Equal(t, len(tt.want), len(got))
			for i := range tt.want {
				assert.Equal(t, tt.want[i].Name, got[i].Name)
			}
		})
	}
}
