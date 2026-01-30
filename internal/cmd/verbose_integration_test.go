package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// ============================================================================
// Config-layer integration tests: real config/session files on disk
// ============================================================================

// TestIntegration_VerboseConfig_TimeoutFromFile tests that timeout set in
// config.yaml is reported with source "file" in verbose output.
func TestIntegration_VerboseConfig_TimeoutFromFile(t *testing.T) {
	env := setupIntegrationTest(t)

	// Init config and set timeout
	require.NoError(t, izanami.InitConfigFile())
	require.True(t, env.ConfigFileExists())
	require.NoError(t, izanami.SetConfigValue("timeout", "60"))

	origProfileName := profileName
	defer func() { profileName = origProfileName }()
	profileName = ""

	testCfg := &izanami.Config{
		BaseURL: env.BaseURL,
		Timeout: 60,
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logEffectiveConfig(cmd, testCfg)

	output := buf.String()
	assert.Contains(t, output, "timeout=60 (source: file)", "timeout from config.yaml should show source 'file'")
	t.Logf("Verbose output:\n%s", output)
}

// TestIntegration_VerboseConfig_TimeoutDefault tests that timeout using
// the viper default is reported with source "default" in verbose output.
func TestIntegration_VerboseConfig_TimeoutDefault(t *testing.T) {
	env := setupIntegrationTest(t)

	// Write a minimal config file WITHOUT timeout so that GetConfigValue
	// falls through to the viper default. InitConfigFile() cannot be used
	// here because it writes "timeout: 30" into the file.
	createTestFile(t, env.ConfigPath, "verbose: false\ncolor: auto\n", 0600)
	require.True(t, env.ConfigFileExists())

	origProfileName := profileName
	defer func() { profileName = origProfileName }()
	profileName = ""

	testCfg := &izanami.Config{
		BaseURL: env.BaseURL,
		Timeout: 30,
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logEffectiveConfig(cmd, testCfg)

	output := buf.String()
	assert.Contains(t, output, "timeout=30 (source: default)", "default timeout should show source 'default'")
	t.Logf("Verbose output:\n%s", output)
}

// TestIntegration_VerboseConfig_ProfileSource tests that values coming from
// a real profile on disk are reported with source "profile".
func TestIntegration_VerboseConfig_ProfileSource(t *testing.T) {
	env := setupIntegrationTest(t)

	// Login to create a real profile with session
	env.Login(t)

	origProfileName := profileName
	defer func() { profileName = origProfileName }()
	profileName = ""

	// Build config that reflects what the profile provides
	testCfg := &izanami.Config{
		BaseURL: env.BaseURL,
		Timeout: 30,
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logEffectiveConfig(cmd, testCfg)

	output := buf.String()
	// After login, base-url comes from the session (referenced by the profile)
	assert.Contains(t, output, "base-url=", "should show base-url")
	assert.Contains(t, output, "timeout=", "should show timeout")
	t.Logf("Verbose output (after login):\n%s", output)
}

// TestIntegration_VerboseConfig_EnvVarSource tests that values from
// environment variables are reported with source "env".
func TestIntegration_VerboseConfig_EnvVarSource(t *testing.T) {
	_ = setupIntegrationTest(t)

	require.NoError(t, izanami.InitConfigFile())

	t.Setenv("IZ_BASE_URL", "http://env-override.example.com")

	origProfileName := profileName
	defer func() { profileName = origProfileName }()
	profileName = ""

	testCfg := &izanami.Config{
		BaseURL: "http://env-override.example.com",
		Timeout: 30,
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)
	// Add the url flag so determineConfigSource can check it
	cmd.Flags().String("url", "", "")

	logEffectiveConfig(cmd, testCfg)

	output := buf.String()
	assert.Contains(t, output, "base-url=http://env-override.example.com (source: env)",
		"base-url from env var should show source 'env'")
	t.Logf("Verbose output (env):\n%s", output)
}

// TestIntegration_VerboseConfig_FlagSource tests that values from
// command-line flags are reported with source "flag".
func TestIntegration_VerboseConfig_FlagSource(t *testing.T) {
	_ = setupIntegrationTest(t)

	require.NoError(t, izanami.InitConfigFile())

	origProfileName := profileName
	defer func() { profileName = origProfileName }()
	profileName = ""

	testCfg := &izanami.Config{
		BaseURL: "http://flag-override.example.com",
		Tenant:  "flag-tenant",
		Timeout: 30,
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)
	cmd.Flags().String("url", "", "")
	cmd.Flags().String("tenant", "", "")
	// Mark flags as explicitly set
	require.NoError(t, cmd.Flags().Set("url", "http://flag-override.example.com"))
	require.NoError(t, cmd.Flags().Set("tenant", "flag-tenant"))

	logEffectiveConfig(cmd, testCfg)

	output := buf.String()
	assert.Contains(t, output, "base-url=http://flag-override.example.com (source: flag)",
		"base-url from flag should show source 'flag'")
	assert.Contains(t, output, "tenant=flag-tenant (source: flag)",
		"tenant from flag should show source 'flag'")
	t.Logf("Verbose output (flag):\n%s", output)
}

// TestIntegration_VerboseConfig_SessionSource tests that base-url coming from
// a session (via profile) is reported with source "session".
func TestIntegration_VerboseConfig_SessionSource(t *testing.T) {
	env := setupIntegrationTest(t)

	// Login creates both a profile and a session
	env.Login(t)

	origProfileName := profileName
	defer func() { profileName = origProfileName }()
	profileName = ""

	// Config with the session URL (login sets this via the profile's session)
	testCfg := &izanami.Config{
		BaseURL: env.BaseURL,
		Timeout: 30,
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)
	// Add required flags for determineConfigSource
	cmd.Flags().String("url", "", "")

	logEffectiveConfig(cmd, testCfg)

	output := buf.String()
	// After login, base-url should come from "session" (profile references session)
	assert.Contains(t, output, "base-url="+env.BaseURL, "should show base-url from session")
	t.Logf("Verbose output (session):\n%s", output)
}

// TestIntegration_VerboseConfig_InsecureSkipped tests that insecure=false
// is not shown in verbose output when using real config files.
func TestIntegration_VerboseConfig_InsecureSkipped(t *testing.T) {
	env := setupIntegrationTest(t)

	require.NoError(t, izanami.InitConfigFile())

	origProfileName := profileName
	defer func() { profileName = origProfileName }()
	profileName = ""

	testCfg := &izanami.Config{
		BaseURL:            env.BaseURL,
		Timeout:            30,
		InsecureSkipVerify: false,
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logEffectiveConfig(cmd, testCfg)

	output := buf.String()
	assert.NotContains(t, output, "insecure=false", "insecure=false should not appear in verbose output")
	t.Logf("Verbose output (insecure skipped):\n%s", output)
}

// TestIntegration_VerboseConfig_SensitiveRedacted tests that sensitive values
// from real config files are properly redacted in verbose output.
func TestIntegration_VerboseConfig_SensitiveRedacted(t *testing.T) {
	env := setupIntegrationTest(t)

	// Create config with profile containing client-secret
	profiles := map[string]*izanami.Profile{
		"test": {
			BaseURL:      env.BaseURL,
			ClientID:     "test-client-id",
			ClientSecret: "super-secret-key-12345",
		},
	}
	createConfigTestFile(t, env.ConfigPath, profiles, "test")

	origProfileName := profileName
	defer func() { profileName = origProfileName }()
	profileName = ""

	testCfg := &izanami.Config{
		BaseURL:      env.BaseURL,
		ClientID:     "test-client-id",
		ClientSecret: "super-secret-key-12345",
		Timeout:      30,
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logEffectiveConfig(cmd, testCfg)

	output := buf.String()
	assert.Contains(t, output, "client-secret=<redacted>", "client-secret should be redacted")
	assert.NotContains(t, output, "super-secret-key-12345", "actual secret must not appear")
	t.Logf("Verbose output (redacted):\n%s", output)
}

// TestIntegration_VerboseConfig_MultipleSourcesEndToEnd tests the full verbose
// config output with multiple sources in a realistic configuration.
func TestIntegration_VerboseConfig_MultipleSourcesEndToEnd(t *testing.T) {
	env := setupIntegrationTest(t)

	// Create config with profile
	profiles := map[string]*izanami.Profile{
		"sandbox": {
			BaseURL: env.BaseURL,
			Tenant:  "sandbox-tenant",
			Project: "sandbox-project",
		},
	}
	createConfigTestFile(t, env.ConfigPath, profiles, "sandbox")
	// Set timeout in config file (distinct from default)
	require.NoError(t, izanami.SetConfigValue("timeout", "45"))

	// Set an env var
	t.Setenv("IZ_CONTEXT", "staging/eu-west")

	origProfileName := profileName
	defer func() { profileName = origProfileName }()
	profileName = ""

	testCfg := &izanami.Config{
		BaseURL: env.BaseURL,
		Tenant:  "sandbox-tenant",
		Project: "sandbox-project",
		Context: "staging/eu-west",
		Timeout: 45,
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)
	// Add flags that determineConfigSource needs to check
	cmd.Flags().String("url", "", "")
	cmd.Flags().String("tenant", "", "")
	cmd.Flags().String("project", "", "")
	cmd.Flags().String("context", "", "")
	cmd.Flags().Int("timeout", 0, "")

	logEffectiveConfig(cmd, testCfg)

	output := buf.String()

	// Profile-sourced fields
	assert.Contains(t, output, "base-url="+env.BaseURL+" (source: profile)", "base-url should come from profile")
	assert.Contains(t, output, "tenant=sandbox-tenant (source: profile)", "tenant should come from profile")
	assert.Contains(t, output, "project=sandbox-project (source: profile)", "project should come from profile")

	// Env-sourced field
	assert.Contains(t, output, "context=staging/eu-west (source: env)", "context should come from env var")

	// File-sourced field
	assert.Contains(t, output, "timeout=45 (source: file)", "timeout should come from config file")

	t.Logf("Verbose end-to-end output:\n%s", output)
}

// ============================================================================
// Full-pipeline integration tests: verbose functions with real server config
// ============================================================================

// TestIntegration_VerboseHealthCheck tests that all verbose functions produce
// expected output when invoked with a real login session.
func TestIntegration_VerboseHealthCheck(t *testing.T) {
	env := setupIntegrationTest(t)

	// Login to set up real config/session/profile on disk
	env.Login(t)

	// Set timeout in config file
	require.NoError(t, izanami.SetConfigValue("timeout", "30"))

	origProfileName := profileName
	defer func() { profileName = origProfileName }()
	profileName = ""

	// Build config matching what LoadConfigWithProfile would produce
	token := env.GetJwtToken(t)
	testCfg := &izanami.Config{
		BaseURL:  env.BaseURL,
		JwtToken: token,
		Timeout:  30,
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	// Run the three verbose functions in sequence (as PersistentPreRunE does)
	logEnvironmentVariables(cmd)
	logEffectiveConfig(cmd, testCfg)
	logAuthenticationMode(cmd, testCfg)

	output := buf.String()

	// Verbose config output should contain config lines
	assert.Contains(t, output, "[verbose] Config:", "verbose should show config lines")
	assert.Contains(t, output, "[verbose] Authentication", "verbose should show auth mode")
	assert.Contains(t, output, "timeout=", "verbose should show timeout")
	assert.Contains(t, output, "base-url=", "verbose should show base-url")
	assert.Contains(t, output, "JWT Cookie (session)", "auth mode should show JWT after login")

	t.Logf("Full verbose output:\n%s", output)
}

// TestIntegration_VerboseHealthCheckWithConfigTimeout tests that timeout
// set in config file appears with correct source in verbose output.
func TestIntegration_VerboseHealthCheckWithConfigTimeout(t *testing.T) {
	env := setupIntegrationTest(t)

	// Login to create profile/session
	env.Login(t)

	// Set a non-default timeout in the config file
	require.NoError(t, izanami.SetConfigValue("timeout", "90"))

	origProfileName := profileName
	defer func() { profileName = origProfileName }()
	profileName = ""

	testCfg := &izanami.Config{
		BaseURL: env.BaseURL,
		Timeout: 90,
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logEffectiveConfig(cmd, testCfg)

	output := buf.String()
	assert.Contains(t, output, "timeout=90 (source: file)", "timeout from config should show source 'file'")

	t.Logf("Verbose (config timeout) output:\n%s", output)
}

// TestIntegration_VerboseHealthCheckNoVerbose verifies that verbose output
// does NOT appear when -v flag is not set.
func TestIntegration_VerboseHealthCheckNoVerbose(t *testing.T) {
	env := setupIntegrationTest(t)

	cleanup := setupHealthTest(t, env)
	defer cleanup()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(healthCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	healthCmd.SetOut(&buf)
	healthCmd.SetErr(&buf)

	cmd.SetArgs([]string{"health"})
	err := cmd.Execute()

	healthCmd.SetOut(nil)
	healthCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()

	// Verbose output should NOT be present
	assert.NotContains(t, output, "[verbose]", "non-verbose health should not show verbose lines")
	// Normal output should be present
	assert.Contains(t, output, "UP", "health output should still work")

	t.Logf("Health (no verbose) output:\n%s", output)
}

// TestIntegration_VerboseConfig_EnvironmentVariablesLogged tests that
// IZ_* environment variables are logged in verbose mode.
func TestIntegration_VerboseConfig_EnvironmentVariablesLogged(t *testing.T) {
	_ = setupIntegrationTest(t)

	t.Setenv("IZ_TENANT", "env-tenant")

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logEnvironmentVariables(cmd)

	output := buf.String()
	assert.Contains(t, output, "IZ_TENANT=env-tenant", "should log IZ_TENANT")
	// IZ_TEST_BASE_URL is also set (by integration test env)
	assert.Contains(t, output, "IZ_TEST_BASE_URL=", "should log IZ_TEST_BASE_URL")

	t.Logf("Environment variables output:\n%s", output)
}

// TestIntegration_VerboseConfig_AuthModeAfterLogin tests that authentication
// mode is correctly reported after login.
func TestIntegration_VerboseConfig_AuthModeAfterLogin(t *testing.T) {
	env := setupIntegrationTest(t)

	// Login to get a JWT token
	token := env.Login(t)
	require.NotEmpty(t, token, "login should return a JWT token")

	testCfg := &izanami.Config{
		BaseURL:  env.BaseURL,
		JwtToken: token,
		Timeout:  30,
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logAuthenticationMode(cmd, testCfg)

	output := buf.String()
	assert.Contains(t, output, "Admin operations: JWT Cookie (session)",
		"should show JWT auth mode after login")

	t.Logf("Auth mode output:\n%s", output)
}
