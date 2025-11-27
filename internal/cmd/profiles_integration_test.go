package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// TestIntegration_LoginCreatesProfile tests that login creates a profile with session reference
func TestIntegration_LoginCreatesProfile(t *testing.T) {
	env := setupIntegrationTest(t)

	// Login
	env.Login(t)

	// Verify profile was created
	profiles, activeProfile, err := izanami.ListProfiles()
	require.NoError(t, err)
	assert.NotEmpty(t, profiles, "Should have at least one profile after login")
	assert.NotEmpty(t, activeProfile, "Should have an active profile after login")

	// Verify profile has session reference
	profile, err := izanami.GetProfile(activeProfile)
	require.NoError(t, err)
	assert.NotEmpty(t, profile.Session, "Profile should have a session reference")

	// Verify session has correct URL
	sessions, err := izanami.LoadSessions()
	require.NoError(t, err)
	session, err := sessions.GetSession(profile.Session)
	require.NoError(t, err)
	assert.Equal(t, env.BaseURL, session.URL, "Session should have correct URL")

	t.Logf("Login created profile '%s' with session '%s'", activeProfile, profile.Session)
}

// TestIntegration_ProfileUseAfterLogin tests switching profiles after login
func TestIntegration_ProfileUseAfterLogin(t *testing.T) {
	env := setupIntegrationTest(t)

	// Login (creates profile)
	env.Login(t)

	// Get the profile name that was created
	activeProfile, err := izanami.GetActiveProfileName()
	require.NoError(t, err)

	// Use profiles use command to switch (even though it's already active, tests the command)
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(profileCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	profileCmd.SetOut(&buf)
	profileCmd.SetErr(&buf)

	// Get the actual active profile name
	activeName, err := izanami.GetActiveProfileName()
	require.NoError(t, err)

	cmd.SetArgs([]string{"profiles", "use", activeName})
	err = cmd.Execute()

	profileCmd.SetOut(nil)
	profileCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "Switched to profile", "Should show switch confirmation")
	assert.Contains(t, output, activeName, "Should mention profile name")

	t.Logf("Successfully used profile: %s", activeProfile)
}

// TestIntegration_ProfileListShowsSessionURL tests that profile list resolves URL from session
func TestIntegration_ProfileListShowsSessionURL(t *testing.T) {
	env := setupIntegrationTest(t)

	// Login (creates profile with session)
	env.Login(t)

	// Run profiles list
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(profileCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	profileCmd.SetOut(&buf)
	profileCmd.SetErr(&buf)
	profileListCmd.SetOut(&buf)
	profileListCmd.SetErr(&buf)

	cmd.SetArgs([]string{"profiles", "list"})
	err := cmd.Execute()

	profileCmd.SetOut(nil)
	profileCmd.SetErr(nil)
	profileListCmd.SetOut(nil)
	profileListCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()

	// Should show URL resolved from session
	assert.Contains(t, output, env.BaseURL, "Profile list should show URL from session")
	assert.Contains(t, output, "Active profile:", "Should indicate active profile")

	t.Logf("Profile list output:\n%s", output)
}

// TestIntegration_ProfileCurrentAfterLogin tests profiles current shows session details
func TestIntegration_ProfileCurrentAfterLogin(t *testing.T) {
	env := setupIntegrationTest(t)

	// Login
	env.Login(t)

	// Run profiles current
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(profileCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	profileCmd.SetOut(&buf)
	profileCmd.SetErr(&buf)
	profileCurrentCmd.SetOut(&buf)
	profileCurrentCmd.SetErr(&buf)

	cmd.SetArgs([]string{"profiles", "current"})
	err := cmd.Execute()

	profileCmd.SetOut(nil)
	profileCmd.SetErr(nil)
	profileCurrentCmd.SetOut(nil)
	profileCurrentCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()

	// Should show profile details
	assert.Contains(t, output, "Active Profile:", "Should show active profile header")
	assert.Contains(t, output, "Session:", "Should show session reference")
	assert.Contains(t, output, env.BaseURL, "Should show URL resolved from session")

	t.Logf("Profile current output:\n%s", output)
}

// TestIntegration_AuthenticatedClientFromProfile tests making API calls with profile auth
func TestIntegration_AuthenticatedClientFromProfile(t *testing.T) {
	env := setupIntegrationTest(t)

	// Login
	env.Login(t)

	// Create authenticated client
	client := env.NewAuthenticatedClient(t)
	require.NotNil(t, client)

	// Make a real API call - list tenants
	ctx := context.Background()
	tenants, err := izanami.ListTenants(client, ctx, nil, izanami.ParseTenants)
	require.NoError(t, err, "Should be able to list tenants with authenticated client")

	t.Logf("Successfully listed %d tenants using authenticated client from profile", len(tenants))
}

// TestIntegration_ProfileSetTenantPersists tests that profile set persists changes
func TestIntegration_ProfileSetTenantPersists(t *testing.T) {
	env := setupIntegrationTest(t)

	// Login
	env.Login(t)

	testTenant := "integration-test-tenant"

	// Run profiles set tenant
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(profileCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	profileCmd.SetOut(&buf)
	profileCmd.SetErr(&buf)
	profileSetCmd.SetOut(&buf)
	profileSetCmd.SetErr(&buf)

	cmd.SetArgs([]string{"profiles", "set", "tenant", testTenant})
	err := cmd.Execute()

	profileCmd.SetOut(nil)
	profileCmd.SetErr(nil)
	profileSetCmd.SetOut(nil)
	profileSetCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "Updated", "Should confirm update")
	assert.Contains(t, output, testTenant, "Should mention new tenant value")

	// Verify tenant was persisted
	activeProfileName, err := izanami.GetActiveProfileName()
	require.NoError(t, err)

	profile, err := izanami.GetProfile(activeProfileName)
	require.NoError(t, err)
	assert.Equal(t, testTenant, profile.Tenant, "Tenant should be persisted in profile")

	t.Logf("Successfully set and persisted tenant: %s", testTenant)
}

// TestIntegration_ProfileClientKeysAddMultipleTenants tests adding client keys for multiple tenants
func TestIntegration_ProfileClientKeysAddMultipleTenants(t *testing.T) {
	env := setupIntegrationTest(t)

	// Login to create profile
	env.Login(t)

	// Define test credentials for multiple tenants
	tenants := []struct {
		name         string
		clientID     string
		clientSecret string
	}{
		{"tenant-alpha", "client-id-alpha", "secret-alpha"},
		{"tenant-beta", "client-id-beta", "secret-beta"},
		{"tenant-gamma", "client-id-gamma", "secret-gamma"},
	}

	// Add credentials for each tenant
	for _, tenant := range tenants {
		err := izanami.AddClientKeys(tenant.name, nil, tenant.clientID, tenant.clientSecret)
		require.NoError(t, err, "Should add client keys for tenant %s", tenant.name)
	}

	// Verify credentials were persisted for each tenant
	activeProfileName, err := izanami.GetActiveProfileName()
	require.NoError(t, err)

	profile, err := izanami.GetProfile(activeProfileName)
	require.NoError(t, err)
	require.NotNil(t, profile.ClientKeys, "Profile should have client keys")

	for _, tenant := range tenants {
		tenantConfig, exists := profile.ClientKeys[tenant.name]
		require.True(t, exists, "Should have client keys for tenant %s", tenant.name)
		assert.Equal(t, tenant.clientID, tenantConfig.ClientID, "Client ID should match for tenant %s", tenant.name)
		assert.Equal(t, tenant.clientSecret, tenantConfig.ClientSecret, "Client secret should match for tenant %s", tenant.name)
		t.Logf("Verified client keys for tenant '%s': client-id=%s", tenant.name, tenant.clientID)
	}

	t.Logf("Successfully added and verified client keys for %d tenants", len(tenants))
}

// TestIntegration_ProfileClientKeysAddProjectsInTenants tests adding client keys for specific projects within tenants
func TestIntegration_ProfileClientKeysAddProjectsInTenants(t *testing.T) {
	env := setupIntegrationTest(t)

	// Login to create profile
	env.Login(t)

	// Define test credentials for projects in tenants
	testCases := []struct {
		tenant       string
		projects     []string
		clientID     string
		clientSecret string
	}{
		{"prod-tenant", []string{"project-api"}, "api-client-id", "api-secret"},
		{"prod-tenant", []string{"project-web"}, "web-client-id", "web-secret"},
		{"staging-tenant", []string{"project-backend", "project-frontend"}, "staging-client-id", "staging-secret"},
	}

	// Add credentials for each project configuration
	for _, tc := range testCases {
		err := izanami.AddClientKeys(tc.tenant, tc.projects, tc.clientID, tc.clientSecret)
		require.NoError(t, err, "Should add client keys for tenant %s projects %v", tc.tenant, tc.projects)
	}

	// Verify credentials were persisted correctly
	activeProfileName, err := izanami.GetActiveProfileName()
	require.NoError(t, err)

	profile, err := izanami.GetProfile(activeProfileName)
	require.NoError(t, err)
	require.NotNil(t, profile.ClientKeys, "Profile should have client keys")

	// Verify prod-tenant project credentials
	prodConfig, exists := profile.ClientKeys["prod-tenant"]
	require.True(t, exists, "Should have config for prod-tenant")
	require.NotNil(t, prodConfig.Projects, "prod-tenant should have project configs")

	apiProject, exists := prodConfig.Projects["project-api"]
	require.True(t, exists, "Should have project-api config")
	assert.Equal(t, "api-client-id", apiProject.ClientID)
	assert.Equal(t, "api-secret", apiProject.ClientSecret)

	webProject, exists := prodConfig.Projects["project-web"]
	require.True(t, exists, "Should have project-web config")
	assert.Equal(t, "web-client-id", webProject.ClientID)
	assert.Equal(t, "web-secret", webProject.ClientSecret)

	// Verify staging-tenant project credentials (multiple projects share same credentials)
	stagingConfig, exists := profile.ClientKeys["staging-tenant"]
	require.True(t, exists, "Should have config for staging-tenant")
	require.NotNil(t, stagingConfig.Projects, "staging-tenant should have project configs")

	backendProject, exists := stagingConfig.Projects["project-backend"]
	require.True(t, exists, "Should have project-backend config")
	assert.Equal(t, "staging-client-id", backendProject.ClientID)

	frontendProject, exists := stagingConfig.Projects["project-frontend"]
	require.True(t, exists, "Should have project-frontend config")
	assert.Equal(t, "staging-client-id", frontendProject.ClientID)

	t.Log("Successfully added and verified project-level client keys")
	t.Logf("  prod-tenant: project-api, project-web")
	t.Logf("  staging-tenant: project-backend, project-frontend")

	// Print config.yaml content for inspection
	configContent := env.ReadConfigFile(t)
	t.Logf("\nConfig file content:\n%s", configContent)
}

// TestIntegration_ProfileClientKeysTenantAndProjectLevels tests mixing tenant-level and project-level credentials
func TestIntegration_ProfileClientKeysTenantAndProjectLevels(t *testing.T) {
	env := setupIntegrationTest(t)

	// Login to create profile
	env.Login(t)

	tenant := "mixed-tenant"

	// Add tenant-level credentials (fallback for all projects)
	err := izanami.AddClientKeys(tenant, nil, "tenant-default-id", "tenant-default-secret")
	require.NoError(t, err, "Should add tenant-level credentials")

	// Add project-specific credentials (override for specific project)
	err = izanami.AddClientKeys(tenant, []string{"special-project"}, "special-project-id", "special-project-secret")
	require.NoError(t, err, "Should add project-level credentials")

	// Verify both levels exist
	activeProfileName, err := izanami.GetActiveProfileName()
	require.NoError(t, err)

	profile, err := izanami.GetProfile(activeProfileName)
	require.NoError(t, err)

	tenantConfig, exists := profile.ClientKeys[tenant]
	require.True(t, exists, "Should have tenant config")

	// Verify tenant-level credentials
	assert.Equal(t, "tenant-default-id", tenantConfig.ClientID, "Tenant-level client ID should be set")
	assert.Equal(t, "tenant-default-secret", tenantConfig.ClientSecret, "Tenant-level client secret should be set")

	// Verify project-level credentials
	require.NotNil(t, tenantConfig.Projects, "Should have project configs")
	specialProject, exists := tenantConfig.Projects["special-project"]
	require.True(t, exists, "Should have special-project config")
	assert.Equal(t, "special-project-id", specialProject.ClientID, "Project-level client ID should be set")
	assert.Equal(t, "special-project-secret", specialProject.ClientSecret, "Project-level client secret should be set")

	t.Logf("Successfully configured mixed tenant/project credentials for '%s'", tenant)
	t.Log("  Tenant-level: tenant-default-id (fallback)")
	t.Log("  Project-level: special-project -> special-project-id (override)")

	// Print config.yaml content for inspection
	configContent := env.ReadConfigFile(t)
	t.Logf("\nConfig file content:\n%s", configContent)
}
