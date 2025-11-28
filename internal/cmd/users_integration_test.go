package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// ============================================================================
// Test Setup Helper
// ============================================================================

// setupUsersTest sets up the global cfg for user command tests
func setupUsersTest(t *testing.T, env *IntegrationTestEnv) func() {
	t.Helper()

	// Get JWT token from session
	token := env.GetJwtToken(t)

	// Save original values
	origCfg := cfg
	origTenant := tenant
	origOutputFormat := outputFormat
	origUserName := userName
	origUserEmail := userEmail
	origUserPassword := userPassword
	origUserAdmin := userAdmin
	origUserType := userType
	origUserDefaultTenant := userDefaultTenant
	origUserRightsFile := userRightsFile
	origUserTenantRight := userTenantRight
	origUserProjectRight := userProjectRight
	origUsersDeleteForce := usersDeleteForce
	origUsersInviteFile := usersInviteFile
	origUsersSearchCount := usersSearchCount

	// Set up config
	cfg = &izanami.Config{
		BaseURL:  env.BaseURL,
		Username: env.Username,
		JwtToken: token,
		Timeout:  30,
	}
	tenant = ""
	outputFormat = "table"
	userName = ""
	userEmail = ""
	userPassword = ""
	userAdmin = false
	userType = "INTERNAL"
	userDefaultTenant = ""
	userRightsFile = ""
	userTenantRight = ""
	userProjectRight = ""
	usersDeleteForce = false
	usersInviteFile = ""
	usersSearchCount = 10

	return func() {
		cfg = origCfg
		tenant = origTenant
		outputFormat = origOutputFormat
		userName = origUserName
		userEmail = origUserEmail
		userPassword = origUserPassword
		userAdmin = origUserAdmin
		userType = origUserType
		userDefaultTenant = origUserDefaultTenant
		userRightsFile = origUserRightsFile
		userTenantRight = origUserTenantRight
		userProjectRight = origUserProjectRight
		usersDeleteForce = origUsersDeleteForce
		usersInviteFile = origUsersInviteFile
		usersSearchCount = origUsersSearchCount
	}
}

// setupUsersTestWithTenant sets up global cfg with a tenant for tenant-scoped tests
func setupUsersTestWithTenant(t *testing.T, env *IntegrationTestEnv, tenantName string) func() {
	t.Helper()
	cleanup := setupUsersTest(t, env)
	cfg.Tenant = tenantName
	tenant = tenantName
	return cleanup
}

// ============================================================================
// USERS LIST (Global)
// ============================================================================

func TestIntegration_UsersListAfterLogin(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupUsersTest(t, env)
	defer cleanup()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersListCmd.SetOut(&buf)
	usersListCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "list"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersListCmd.SetOut(nil)
	usersListCmd.SetErr(nil)

	require.NoError(t, err, "Users list should succeed")

	// Verify via API that we have users
	client := env.NewAuthenticatedClient(t)
	ctx := context.Background()
	users, err := izanami.ListUsers(client, ctx, izanami.ParseUserListItems)
	require.NoError(t, err)

	t.Logf("Listed %d users via API", len(users))
}

func TestIntegration_UsersListJSONOutput(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupUsersTest(t, env)
	defer cleanup()

	// Set JSON output format
	outputFormat = "json"

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersListCmd.SetOut(&buf)
	usersListCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "list"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersListCmd.SetOut(nil)
	usersListCmd.SetErr(nil)

	require.NoError(t, err, "Users list JSON should succeed")
	output := buf.String()

	// Should be valid JSON array
	assert.True(t, strings.HasPrefix(strings.TrimSpace(output), "["), "JSON output should start with [")

	t.Logf("Users list JSON output length: %d chars", len(output))
}

func TestIntegration_UsersListVerifyViaAPI(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupUsersTest(t, env)
	defer cleanup()

	// Set JSON output to capture structured data
	outputFormat = "json"

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersListCmd.SetOut(&buf)
	usersListCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "list"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersListCmd.SetOut(nil)
	usersListCmd.SetErr(nil)

	require.NoError(t, err)
	cliOutput := buf.String()

	// Verify via API
	client := env.NewAuthenticatedClient(t)
	ctx := context.Background()
	users, err := izanami.ListUsers(client, ctx, izanami.ParseUserListItems)
	require.NoError(t, err, "API list users should succeed")

	// Verify CLI JSON output contains usernames from API
	for _, user := range users {
		assert.Contains(t, cliOutput, user.Username, "CLI JSON output should contain user: %s", user.Username)
	}

	t.Logf("Verified %d users from API appear in CLI JSON output", len(users))
}

// ============================================================================
// USERS GET
// ============================================================================

func TestIntegration_UsersGetExisting(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupUsersTest(t, env)
	defer cleanup()

	// Get the logged-in user (which we know exists)
	username := env.Username

	// Set JSON output for reliable assertion
	outputFormat = "json"

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersGetCmd.SetOut(&buf)
	usersGetCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "get", username})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersGetCmd.SetOut(nil)
	usersGetCmd.SetErr(nil)

	require.NoError(t, err, "Users get should succeed")
	output := buf.String()

	// Should display username in JSON
	assert.Contains(t, output, username, "Output should contain username")
	assert.True(t, strings.HasPrefix(strings.TrimSpace(output), "{"), "JSON output should start with {")

	t.Logf("Users get output for '%s': %d chars", username, len(output))
}

func TestIntegration_UsersGetTableOutput(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupUsersTest(t, env)
	defer cleanup()

	// Use table output (default)
	outputFormat = "table"
	username := env.Username

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersGetCmd.SetOut(&buf)
	usersGetCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "get", username})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersGetCmd.SetOut(nil)
	usersGetCmd.SetErr(nil)

	require.NoError(t, err, "Users get with table output should succeed")
	output := buf.String()

	// Should display user details in fancy format
	assert.Contains(t, output, "User:", "Table output should show User: label")
	assert.Contains(t, output, username, "Output should contain username")

	t.Logf("Users get table output for '%s':\n%s", username, output)
}

func TestIntegration_UsersGetNonExistent(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupUsersTest(t, env)
	defer cleanup()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersGetCmd.SetOut(&buf)
	usersGetCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "get", "non-existent-user-12345"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersGetCmd.SetOut(nil)
	usersGetCmd.SetErr(nil)

	require.Error(t, err, "Getting non-existent user should fail")

	t.Logf("Expected error for non-existent user: %v", err)
}

func TestIntegration_UsersGetMissingArg(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupUsersTest(t, env)
	defer cleanup()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersGetCmd.SetOut(&buf)
	usersGetCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "get"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersGetCmd.SetOut(nil)
	usersGetCmd.SetErr(nil)

	require.Error(t, err, "Get without username should fail")
	assert.Contains(t, err.Error(), "accepts 1 arg", "Error should mention argument requirement")

	t.Logf("Expected error for missing arg: %v", err)
}

// ============================================================================
// USERS SEARCH
// ============================================================================

func TestIntegration_UsersSearch(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupUsersTest(t, env)
	defer cleanup()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersSearchCmd.SetOut(&buf)
	usersSearchCmd.SetErr(&buf)

	// Search with a query that should return results
	cmd.SetArgs([]string{"admin", "users", "search", "a"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersSearchCmd.SetOut(nil)
	usersSearchCmd.SetErr(nil)

	require.NoError(t, err, "User search should succeed")

	t.Logf("Search completed, output: %s", buf.String())
}

func TestIntegration_UsersSearchNoResults(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupUsersTest(t, env)
	defer cleanup()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersSearchCmd.SetOut(&buf)
	usersSearchCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "search", "xyz_nonexistent_query_xyz"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersSearchCmd.SetOut(nil)
	usersSearchCmd.SetErr(nil)

	require.NoError(t, err, "Search with no results should succeed")
	output := buf.String()
	assert.Contains(t, output, "No users found", "Should indicate no users found")

	t.Logf("Search with no results output: %s", output)
}

func TestIntegration_UsersSearchWithCount(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupUsersTest(t, env)
	defer cleanup()

	// Set count flag
	usersSearchCount = 5

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersSearchCmd.SetOut(&buf)
	usersSearchCmd.SetErr(&buf)

	// Search with a broad query that might return multiple users
	cmd.SetArgs([]string{"admin", "users", "search", "a", "--count", "5"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersSearchCmd.SetOut(nil)
	usersSearchCmd.SetErr(nil)

	require.NoError(t, err, "User search with count should succeed")

	t.Logf("Search with count=5 completed")
}

// ============================================================================
// TENANT-SCOPED USER OPERATIONS
// ============================================================================

func TestIntegration_UsersListForTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a tenant
	tempTenant := NewTempTenant(t, client, "Users list for tenant test").Cleanup(t).MustCreate(t)

	cleanup := setupUsersTestWithTenant(t, env, tempTenant.Name)
	defer cleanup()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersListForTenantCmd.SetOut(&buf)
	usersListForTenantCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "list-for-tenant"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersListForTenantCmd.SetOut(nil)
	usersListForTenantCmd.SetErr(nil)

	require.NoError(t, err, "List users for tenant should succeed")

	t.Logf("Listed users for tenant: %s", tempTenant.Name)
}

func TestIntegration_UsersListForTenantMissingTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupUsersTest(t, env)
	defer cleanup()

	// Ensure tenant is empty
	cfg.Tenant = ""
	tenant = ""

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersListForTenantCmd.SetOut(&buf)
	usersListForTenantCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "list-for-tenant"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersListForTenantCmd.SetOut(nil)
	usersListForTenantCmd.SetErr(nil)

	require.Error(t, err, "List for tenant without tenant should fail")

	t.Logf("Expected error for missing tenant: %v", err)
}

func TestIntegration_UsersGetForTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a tenant
	tempTenant := NewTempTenant(t, client, "User get for tenant test").Cleanup(t).MustCreate(t)

	cleanup := setupUsersTestWithTenant(t, env, tempTenant.Name)
	defer cleanup()

	// Get the logged-in user's tenant rights
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersGetForTenantCmd.SetOut(&buf)
	usersGetForTenantCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "get-for-tenant", env.Username})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersGetForTenantCmd.SetOut(nil)
	usersGetForTenantCmd.SetErr(nil)

	require.NoError(t, err, "Get user for tenant should succeed")

	t.Logf("Got user %s for tenant %s", env.Username, tempTenant.Name)
}

func TestIntegration_UsersGetForTenantMissingTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupUsersTest(t, env)
	defer cleanup()

	// Ensure tenant is empty
	cfg.Tenant = ""
	tenant = ""

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersGetForTenantCmd.SetOut(&buf)
	usersGetForTenantCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "get-for-tenant", "someuser"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersGetForTenantCmd.SetOut(nil)
	usersGetForTenantCmd.SetErr(nil)

	require.Error(t, err, "Get for tenant without tenant should fail")

	t.Logf("Expected error for missing tenant: %v", err)
}

func TestIntegration_UsersUpdateTenantRightsMissingTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupUsersTest(t, env)
	defer cleanup()

	// Ensure tenant is empty
	cfg.Tenant = ""
	tenant = ""

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersUpdateTenantRightsCmd.SetOut(&buf)
	usersUpdateTenantRightsCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "update-tenant-rights", "someuser", "--tenant-right", "Admin"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersUpdateTenantRightsCmd.SetOut(nil)
	usersUpdateTenantRightsCmd.SetErr(nil)

	require.Error(t, err, "Update tenant rights without tenant should fail")

	t.Logf("Expected error for missing tenant: %v", err)
}

func TestIntegration_UsersUpdateTenantRightsNoRights(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a tenant
	tempTenant := NewTempTenant(t, client, "Update tenant rights no rights test").Cleanup(t).MustCreate(t)

	cleanup := setupUsersTestWithTenant(t, env, tempTenant.Name)
	defer cleanup()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersUpdateTenantRightsCmd.SetOut(&buf)
	usersUpdateTenantRightsCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "update-tenant-rights", "someuser"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersUpdateTenantRightsCmd.SetOut(nil)
	usersUpdateTenantRightsCmd.SetErr(nil)

	require.Error(t, err, "Update tenant rights without rights should fail")
	assert.Contains(t, err.Error(), "no rights specified", "Error should mention no rights")

	t.Logf("Expected error for no rights: %v", err)
}

func TestIntegration_UsersInviteToTenantMissingTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupUsersTest(t, env)
	defer cleanup()

	// Ensure tenant is empty
	cfg.Tenant = ""
	tenant = ""

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersInviteToTenantCmd.SetOut(&buf)
	usersInviteToTenantCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "invite-to-tenant", "--invite-file", "somefile.json"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersInviteToTenantCmd.SetOut(nil)
	usersInviteToTenantCmd.SetErr(nil)

	require.Error(t, err, "Invite to tenant without tenant should fail")

	t.Logf("Expected error for missing tenant: %v", err)
}

func TestIntegration_UsersInviteToTenantMissingFile(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a tenant
	tempTenant := NewTempTenant(t, client, "Invite missing file test").Cleanup(t).MustCreate(t)

	cleanup := setupUsersTestWithTenant(t, env, tempTenant.Name)
	defer cleanup()

	// Don't set invite file
	usersInviteFile = ""

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersInviteToTenantCmd.SetOut(&buf)
	usersInviteToTenantCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "invite-to-tenant"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersInviteToTenantCmd.SetOut(nil)
	usersInviteToTenantCmd.SetErr(nil)

	require.Error(t, err, "Invite without file should fail")
	assert.Contains(t, err.Error(), "invite file is required", "Error should mention file requirement")

	t.Logf("Expected error for missing file: %v", err)
}

// ============================================================================
// PROJECT-SCOPED USER OPERATIONS
// ============================================================================

func TestIntegration_UsersListForProject(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a tenant and project
	tempTenant := NewTempTenant(t, client, "Users list for project test").Cleanup(t).MustCreate(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "Users list project").MustCreate(t)

	cleanup := setupUsersTestWithTenant(t, env, tempTenant.Name)
	defer cleanup()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersListForProjectCmd.SetOut(&buf)
	usersListForProjectCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "list-for-project", tempProject.Name})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersListForProjectCmd.SetOut(nil)
	usersListForProjectCmd.SetErr(nil)

	require.NoError(t, err, "List users for project should succeed")

	t.Logf("Listed users for project: %s/%s", tempTenant.Name, tempProject.Name)
}

func TestIntegration_UsersListForProjectMissingTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupUsersTest(t, env)
	defer cleanup()

	// Ensure tenant is empty
	cfg.Tenant = ""
	tenant = ""

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersListForProjectCmd.SetOut(&buf)
	usersListForProjectCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "list-for-project", "some-project"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersListForProjectCmd.SetOut(nil)
	usersListForProjectCmd.SetErr(nil)

	require.Error(t, err, "List for project without tenant should fail")

	t.Logf("Expected error for missing tenant: %v", err)
}

func TestIntegration_UsersListForProjectMissingArg(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a tenant
	tempTenant := NewTempTenant(t, client, "List for project missing arg test").Cleanup(t).MustCreate(t)

	cleanup := setupUsersTestWithTenant(t, env, tempTenant.Name)
	defer cleanup()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersListForProjectCmd.SetOut(&buf)
	usersListForProjectCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "list-for-project"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersListForProjectCmd.SetOut(nil)
	usersListForProjectCmd.SetErr(nil)

	require.Error(t, err, "List for project without project arg should fail")
	assert.Contains(t, err.Error(), "accepts 1 arg", "Error should mention argument requirement")

	t.Logf("Expected error for missing arg: %v", err)
}

func TestIntegration_UsersUpdateProjectRightsMissingTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupUsersTest(t, env)
	defer cleanup()

	// Ensure tenant is empty
	cfg.Tenant = ""
	tenant = ""

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersUpdateProjectRightsCmd.SetOut(&buf)
	usersUpdateProjectRightsCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "update-project-rights", "someuser", "someproject", "--project-right", "Admin"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersUpdateProjectRightsCmd.SetOut(nil)
	usersUpdateProjectRightsCmd.SetErr(nil)

	require.Error(t, err, "Update project rights without tenant should fail")

	t.Logf("Expected error for missing tenant: %v", err)
}

func TestIntegration_UsersUpdateProjectRightsNoRights(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a tenant and project
	tempTenant := NewTempTenant(t, client, "Update project rights no rights test").Cleanup(t).MustCreate(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "Update project rights project").MustCreate(t)

	cleanup := setupUsersTestWithTenant(t, env, tempTenant.Name)
	defer cleanup()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersUpdateProjectRightsCmd.SetOut(&buf)
	usersUpdateProjectRightsCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "update-project-rights", "someuser", tempProject.Name})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersUpdateProjectRightsCmd.SetOut(nil)
	usersUpdateProjectRightsCmd.SetErr(nil)

	require.Error(t, err, "Update project rights without rights should fail")
	assert.Contains(t, err.Error(), "no rights specified", "Error should mention no rights")

	t.Logf("Expected error for no rights: %v", err)
}

func TestIntegration_UsersInviteToProjectMissingTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupUsersTest(t, env)
	defer cleanup()

	// Ensure tenant is empty
	cfg.Tenant = ""
	tenant = ""

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersInviteToProjectCmd.SetOut(&buf)
	usersInviteToProjectCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "invite-to-project", "someproject", "--invite-file", "somefile.json"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersInviteToProjectCmd.SetOut(nil)
	usersInviteToProjectCmd.SetErr(nil)

	require.Error(t, err, "Invite to project without tenant should fail")

	t.Logf("Expected error for missing tenant: %v", err)
}

func TestIntegration_UsersInviteToProjectMissingFile(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a tenant and project
	tempTenant := NewTempTenant(t, client, "Invite to project missing file test").Cleanup(t).MustCreate(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "Invite project").MustCreate(t)

	cleanup := setupUsersTestWithTenant(t, env, tempTenant.Name)
	defer cleanup()

	// Don't set invite file
	usersInviteFile = ""

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersInviteToProjectCmd.SetOut(&buf)
	usersInviteToProjectCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "invite-to-project", tempProject.Name})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersInviteToProjectCmd.SetOut(nil)
	usersInviteToProjectCmd.SetErr(nil)

	require.Error(t, err, "Invite without file should fail")
	assert.Contains(t, err.Error(), "invite file is required", "Error should mention file requirement")

	t.Logf("Expected error for missing file: %v", err)
}

// ============================================================================
// AUTH ERROR CASES
// ============================================================================

func TestIntegration_UsersListWithoutLogin(t *testing.T) {
	_ = setupIntegrationTest(t) // No login

	// Save and clear cfg
	origCfg := cfg
	cfg = nil
	defer func() { cfg = origCfg }()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	usersCmd.SetOut(&buf)
	usersCmd.SetErr(&buf)
	usersListCmd.SetOut(&buf)
	usersListCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "users", "list"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	usersCmd.SetOut(nil)
	usersCmd.SetErr(nil)
	usersListCmd.SetOut(nil)
	usersListCmd.SetErr(nil)

	require.Error(t, err, "Users list without login should fail")

	t.Logf("Expected error without login: %v", err)
}

// ============================================================================
// API DIRECT TESTS
// ============================================================================

func TestIntegration_UsersAPIListUsers(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)
	ctx := context.Background()

	users, err := izanami.ListUsers(client, ctx, izanami.ParseUserListItems)
	require.NoError(t, err, "ListUsers API should succeed")

	t.Logf("ListUsers API returned %d users", len(users))

	// Should include at least the test user
	found := false
	for _, u := range users {
		if u.Username == env.Username {
			found = true
			break
		}
	}
	assert.True(t, found, "Should find logged-in user in list")
}

func TestIntegration_UsersAPIGetUser(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Get the logged-in user
	ctx := context.Background()
	user, err := izanami.GetUser(client, ctx, env.Username, izanami.ParseUser)
	require.NoError(t, err, "GetUser API should succeed")

	assert.Equal(t, env.Username, user.Username, "Username should match")

	t.Logf("GetUser API returned user: %s (admin=%t)", user.Username, user.Admin)
}

func TestIntegration_UsersAPISearchUsers(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Search with a simple query
	ctx := context.Background()
	usernames, err := client.SearchUsers(ctx, "a", 10)
	require.NoError(t, err, "SearchUsers API should succeed")

	t.Logf("SearchUsers API returned %d usernames", len(usernames))
}

func TestIntegration_UsersAPIListUsersForTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a tenant
	tempTenant := NewTempTenant(t, client, "API list users for tenant").Cleanup(t).MustCreate(t)

	ctx := context.Background()
	users, err := client.ListUsersForTenant(ctx, tempTenant.Name)
	require.NoError(t, err, "ListUsersForTenant API should succeed")

	t.Logf("ListUsersForTenant API returned %d users for tenant %s", len(users), tempTenant.Name)
}

func TestIntegration_UsersAPIListUsersForProject(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a tenant and project
	tempTenant := NewTempTenant(t, client, "API list users for project").Cleanup(t).MustCreate(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "API list users project").MustCreate(t)

	ctx := context.Background()
	users, err := client.ListUsersForProject(ctx, tempTenant.Name, tempProject.Name)
	require.NoError(t, err, "ListUsersForProject API should succeed")

	t.Logf("ListUsersForProject API returned %d users for project %s/%s", len(users), tempTenant.Name, tempProject.Name)
}
