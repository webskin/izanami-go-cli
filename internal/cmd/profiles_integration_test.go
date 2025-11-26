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
	tenants, err := client.ListTenants(ctx, nil)
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
