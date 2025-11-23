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

	client, err := NewClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	projects, err := client.ListProjects(ctx, "test-tenant")

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

	client, err := NewClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	project, err := client.GetProject(ctx, "test-tenant", "test-project")

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

	client, err := NewClient(config)
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

	client, err := NewClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.DeleteProject(ctx, "test-tenant", "test-project")

	assert.NoError(t, err)
}
