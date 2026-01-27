package izanami

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_ListUsers(t *testing.T) {
	expectedUsers := []User{
		{
			Username: "user1",
			Email:    "user1@example.com",
			Admin:    false,
		},
		{
			Username: "user2",
			Email:    "user2@example.com",
			Admin:    true,
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/users", r.URL.Path)
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
	users, err := ListUsers(client, ctx, ParseUsers)

	assert.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Equal(t, "user1", users[0].Username)
	assert.Equal(t, "user2", users[1].Username)
}

func TestClient_GetUser(t *testing.T) {
	expectedUser := &User{
		Username: "testuser",
		Email:    "testuser@example.com",
		Admin:    false,
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/users/testuser", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedUser)
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
	user, err := GetUser(client, ctx, "testuser", ParseUser)

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, "testuser@example.com", user.Email)
}

func TestClient_CreateUser(t *testing.T) {
	userData := map[string]interface{}{
		"username": "newuser",
		"email":    "newuser@example.com",
		"password": "securepassword",
		"admin":    false,
	}

	expectedUser := &User{
		Username: "newuser",
		Email:    "newuser@example.com",
		Admin:    false,
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/users", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Decode and verify request body
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, "newuser", body["username"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(expectedUser)
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
	user, err := client.CreateUser(ctx, userData)

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "newuser", user.Username)
}

func TestClient_UpdateUser(t *testing.T) {
	userData := map[string]interface{}{
		"email": "updated@example.com",
		"admin": true,
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/users/testuser", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Verify request body
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, "updated@example.com", body["email"])

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
	err = client.UpdateUser(ctx, "testuser", userData)

	assert.NoError(t, err)
}

func TestClient_DeleteUser(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/users/testuser", r.URL.Path)
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
	err = client.DeleteUser(ctx, "testuser")

	assert.NoError(t, err)
}

func TestClient_ListUsersForTenant(t *testing.T) {
	expectedUsers := []UserWithSingleLevelRight{
		{
			Username: "user1",
			Email:    "user1@example.com",
			Admin:    false,
			Right:    "Read",
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/users", r.URL.Path)
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
	users, err := client.ListUsersForTenant(ctx, "test-tenant")

	assert.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "user1", users[0].Username)
	assert.Equal(t, "Read", users[0].Right)
}

func TestClient_UpdateUserTenantRights(t *testing.T) {
	rightsData := map[string]interface{}{
		"level": "Write",
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/users")
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

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
	err = client.UpdateUserTenantRights(ctx, "test-tenant", "testuser", rightsData)

	assert.NoError(t, err)
}

func TestClient_InviteUsersToTenant(t *testing.T) {
	inviteData := []UserInvitation{
		{Username: "user1@example.com", Level: "Read"},
		{Username: "user2@example.com", Level: "Read"},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/users")
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

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
	err = client.InviteUsersToTenant(ctx, "test-tenant", inviteData)

	assert.NoError(t, err)
}

func TestClient_UpdateUserRights(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/users/testuser/rights", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Verify request body
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.NotNil(t, body["rights"])

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

	rightsReq := UserRightsUpdateRequest{
		Rights: map[string]TenantRight{
			"tenant1": {Level: "Admin"},
		},
	}

	ctx := context.Background()
	err = client.UpdateUserRights(ctx, "testuser", rightsReq)

	assert.NoError(t, err)
}

func TestClient_SearchUsers(t *testing.T) {
	expectedUsers := []string{"user1@example.com", "user2@example.com"}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/_search/users", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "user", r.URL.Query().Get("query"))

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
	users, err := client.SearchUsers(ctx, "user", 0)

	assert.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Equal(t, "user1@example.com", users[0])
}

func TestClient_SearchUsers_WithCount(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "5", r.URL.Query().Get("count"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]string{"user1@example.com"})
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
	_, err = client.SearchUsers(ctx, "user", 5)

	assert.NoError(t, err)
}

func TestClient_GetUserForTenant(t *testing.T) {
	expectedUser := &User{
		Username: "testuser",
		Email:    "testuser@example.com",
		Admin:    false,
		Rights: UserRights{
			Tenants: map[string]TenantRight{
				"test-tenant": {Level: "Admin"},
			},
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/users/testuser", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedUser)
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
	user, err := client.GetUserForTenant(ctx, "test-tenant", "testuser")

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "testuser", user.Username)
}

func TestClient_ListUsersForProject(t *testing.T) {
	expectedUsers := []ProjectScopedUser{
		{
			Username: "user1",
			Email:    "user1@example.com",
			Right:    "Read",
		},
		{
			Username: "user2",
			Email:    "user2@example.com",
			Right:    "Admin",
		},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/projects/test-project/users", r.URL.Path)
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
	users, err := client.ListUsersForProject(ctx, "test-tenant", "test-project")

	assert.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Equal(t, "user1", users[0].Username)
	assert.Equal(t, "Read", users[0].Right)
}

func TestClient_UpdateUserProjectRights(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/projects/test-project/users/testuser", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Verify request body
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, "Write", body["level"])

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

	rightsReq := ProjectRightUpdateRequest{Level: "Write"}

	ctx := context.Background()
	err = client.UpdateUserProjectRights(ctx, "test-tenant", "test-project", "testuser", rightsReq)

	assert.NoError(t, err)
}

func TestClient_InviteUsersToProject(t *testing.T) {
	inviteData := []UserInvitation{
		{Username: "user1@example.com", Level: "Read"},
		{Username: "user2@example.com", Level: "Write"},
	}

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/projects/test-project/users", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Verify request body
		var body []UserInvitation
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Len(t, body, 2)

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
	err = client.InviteUsersToProject(ctx, "test-tenant", "test-project", inviteData)

	assert.NoError(t, err)
}
