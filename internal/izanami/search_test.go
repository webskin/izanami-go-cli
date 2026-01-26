package izanami

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Search(t *testing.T) {
	expectedResults := []SearchResult{
		{
			Type:   "feature",
			Name:   "Feature 1",
			Tenant: "test-tenant",
			Path: []SearchPathEntry{
				{Type: "tenant", Name: "test-tenant"},
				{Type: "project", Name: "test-project"},
			},
		},
		{
			Type:   "project",
			Name:   "Project 2",
			Tenant: "test-tenant",
			Path: []SearchPathEntry{
				{Type: "tenant", Name: "test-tenant"},
			},
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/search", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		// Check query params
		query := r.URL.Query().Get("query")
		filters := r.URL.Query()["filter"]
		assert.Equal(t, "test", query)
		assert.Equal(t, []string{"feature", "project"}, filters)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResults)
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
	results, err := Search(client, ctx, "test-tenant", "test", []string{"feature", "project"}, ParseSearchResults)

	assert.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "Feature 1", results[0].Name)
	assert.Equal(t, "feature", results[0].Type)
	assert.Equal(t, "test-tenant", results[0].Tenant)
	assert.Equal(t, "Project 2", results[1].Name)
	assert.Equal(t, "project", results[1].Type)
}

func TestClient_Search_Global(t *testing.T) {
	expectedResults := []SearchResult{
		{
			Type:   "feature",
			Name:   "Global Feature",
			Tenant: "global-tenant",
			Path: []SearchPathEntry{
				{Type: "tenant", Name: "global-tenant"},
				{Type: "project", Name: "global-project"},
			},
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
		BaseURL:  server.URL,
		Username: "test-user",
		JwtToken: "test-jwt-token",
		Timeout:  30,
	}

	client, err := NewAdminClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	results, err := Search(client, ctx, "", "test", nil, ParseSearchResults)

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Global Feature", results[0].Name)
	assert.Equal(t, "feature", results[0].Type)
}

func TestSearchResult_ToTableView(t *testing.T) {
	tests := []struct {
		name     string
		result   SearchResult
		expected SearchResultTableView
	}{
		{
			name: "feature with project in path",
			result: SearchResult{
				Type:   "feature",
				Name:   "my-feature",
				Tenant: "my-tenant",
				Path: []SearchPathEntry{
					{Type: "tenant", Name: "my-tenant"},
					{Type: "project", Name: "my-project"},
				},
			},
			expected: SearchResultTableView{
				Type:    "feature",
				Name:    "my-feature",
				Tenant:  "my-tenant",
				Project: "my-project",
			},
		},
		{
			name: "project without project in path",
			result: SearchResult{
				Type:   "project",
				Name:   "my-project",
				Tenant: "my-tenant",
				Path: []SearchPathEntry{
					{Type: "tenant", Name: "my-tenant"},
				},
			},
			expected: SearchResultTableView{
				Type:    "project",
				Name:    "my-project",
				Tenant:  "my-tenant",
				Project: "",
			},
		},
		{
			name: "key at tenant level",
			result: SearchResult{
				Type:   "key",
				Name:   "my-key",
				Tenant: "my-tenant",
				Path: []SearchPathEntry{
					{Type: "tenant", Name: "my-tenant"},
				},
			},
			expected: SearchResultTableView{
				Type:    "key",
				Name:    "my-key",
				Tenant:  "my-tenant",
				Project: "",
			},
		},
		{
			name: "empty path",
			result: SearchResult{
				Type:   "tag",
				Name:   "my-tag",
				Tenant: "my-tenant",
				Path:   nil,
			},
			expected: SearchResultTableView{
				Type:    "tag",
				Name:    "my-tag",
				Tenant:  "my-tenant",
				Project: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := tt.result.ToTableView()
			assert.Equal(t, tt.expected, view)
		})
	}
}

func TestSearchResultsToTableView(t *testing.T) {
	results := []SearchResult{
		{
			Type:   "feature",
			Name:   "feature-1",
			Tenant: "tenant-1",
			Path: []SearchPathEntry{
				{Type: "tenant", Name: "tenant-1"},
				{Type: "project", Name: "project-1"},
			},
		},
		{
			Type:   "key",
			Name:   "key-1",
			Tenant: "tenant-1",
			Path: []SearchPathEntry{
				{Type: "tenant", Name: "tenant-1"},
			},
		},
	}

	views := SearchResultsToTableView(results)

	assert.Len(t, views, 2)
	assert.Equal(t, "feature", views[0].Type)
	assert.Equal(t, "feature-1", views[0].Name)
	assert.Equal(t, "project-1", views[0].Project)
	assert.Equal(t, "key", views[1].Type)
	assert.Equal(t, "key-1", views[1].Name)
	assert.Equal(t, "", views[1].Project)
}
