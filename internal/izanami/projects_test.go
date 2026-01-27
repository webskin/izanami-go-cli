package izanami

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_ListProjects(t *testing.T) {
	expectedProjects := []Project{
		{
			ID:          "project-1",
			Name:        "project-1",
			Description: "Test project 1",
		},
		{
			ID:          "project-2",
			Name:        "project-2",
			Description: "Test project 2",
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/projects", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedProjects)
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
	projects, err := ListProjects(client, ctx, "test-tenant", ParseProjects)

	assert.NoError(t, err)
	assert.Len(t, projects, 2)
	assert.Equal(t, "project-1", projects[0].ID)
	assert.Equal(t, "project-2", projects[1].ID)
}

func TestClient_GetProject(t *testing.T) {
	expectedProject := &Project{
		ID:          "test-project",
		Name:        "test-project",
		Description: "Test project description",
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/projects/test-project", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedProject)
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
	project, err := GetProject(client, ctx, "test-tenant", "test-project", ParseProject)

	assert.NoError(t, err)
	assert.NotNil(t, project)
	assert.Equal(t, "test-project", project.ID)
	assert.Equal(t, "Test project description", project.Description)
}

func TestClient_CreateProject(t *testing.T) {
	projectData := map[string]interface{}{
		"id":          "new-project",
		"name":        "new-project",
		"description": "New test project",
	}

	expectedProject := &Project{
		ID:          "new-project",
		Name:        "new-project",
		Description: "New test project",
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/projects", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Decode and verify request body
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, "new-project", body["id"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(expectedProject)
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
	err = client.CreateProject(ctx, "test-tenant", projectData)

	assert.NoError(t, err)
}

func TestClient_DeleteProject(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/projects/test-project", r.URL.Path)
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
	err = client.DeleteProject(ctx, "test-tenant", "test-project")

	assert.NoError(t, err)
}

func TestClient_UpdateProject(t *testing.T) {
	projectData := map[string]interface{}{
		"name":        "updated-project",
		"description": "Updated project description",
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/projects/test-project", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Verify request body
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, "updated-project", body["name"])

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
	err = client.UpdateProject(ctx, "test-tenant", "test-project", projectData)

	assert.NoError(t, err)
}

func TestClient_ListProjectLogs(t *testing.T) {
	expectedLogs := &LogsResponse{
		Events: []AuditEvent{
			{
				EventID:   1,
				ID:        "feature1",
				Name:      "Feature 1",
				Tenant:    "test-tenant",
				Project:   "test-project",
				User:      "admin",
				Type:      "FEATURE_CREATED",
				EmittedAt: "2024-01-15T10:00:00Z",
			},
		},
		Count: 1,
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/projects/test-project/logs", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedLogs)
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
	logs, err := ListProjectLogs(client, ctx, "test-tenant", "test-project", nil, ParseLogsResponse)

	assert.NoError(t, err)
	assert.NotNil(t, logs)
	assert.Len(t, logs.Events, 1)
	assert.Equal(t, "FEATURE_CREATED", logs.Events[0].Type)
}

func TestClient_ListProjectLogs_WithFilters(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "desc", r.URL.Query().Get("order"))
		assert.Equal(t, "admin", r.URL.Query().Get("users"))
		assert.Equal(t, "FEATURE_CREATED,FEATURE_UPDATED", r.URL.Query().Get("types"))
		assert.Equal(t, "feature1", r.URL.Query().Get("features"))
		assert.Equal(t, "2024-01-01T00:00:00Z", r.URL.Query().Get("start"))
		assert.Equal(t, "2024-01-31T23:59:59Z", r.URL.Query().Get("end"))
		assert.Equal(t, "100", r.URL.Query().Get("cursor"))
		assert.Equal(t, "50", r.URL.Query().Get("count"))
		assert.Equal(t, "true", r.URL.Query().Get("total"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&LogsResponse{Events: []AuditEvent{}})
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

	opts := &LogsRequest{
		Order:    "desc",
		Users:    "admin",
		Types:    "FEATURE_CREATED,FEATURE_UPDATED",
		Features: "feature1",
		Start:    "2024-01-01T00:00:00Z",
		End:      "2024-01-31T23:59:59Z",
		Cursor:   100,
		Count:    50,
		Total:    true,
	}

	ctx := context.Background()
	_, err = ListProjectLogs(client, ctx, "test-tenant", "test-project", opts, ParseLogsResponse)

	assert.NoError(t, err)
}
