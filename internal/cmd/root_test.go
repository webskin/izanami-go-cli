package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// ============================================================================
// Helper: create a cobra command with the same flags as rootCmd
// ============================================================================

// setupVerboseTestCommand creates a minimal cobra command with the flags
// needed by determineConfigSource (url, tenant, project, context, timeout,
// insecure). It does NOT mutate the package-level flag variables.
func setupVerboseTestCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("url", "", "")
	cmd.Flags().String("tenant", "", "")
	cmd.Flags().String("project", "", "")
	cmd.Flags().String("context", "", "")
	cmd.Flags().Int("timeout", 0, "")
	cmd.Flags().Bool("insecure", false, "")
	return cmd
}

// ============================================================================
// logEffectiveConfig tests
// ============================================================================

func TestLogEffectiveConfig_TimeoutAlwaysShown(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Save and restore package-level profileName
	origProfileName := profileName
	defer func() { profileName = origProfileName }()
	profileName = ""

	testCfg := &izanami.ResolvedConfig{Timeout: 30}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logEffectiveConfig(cmd, testCfg)

	output := buf.String()
	assert.Contains(t, output, "timeout=30", "timeout should always appear in verbose output")
}

func TestLogEffectiveConfig_TimeoutZeroShown(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	origProfileName := profileName
	defer func() { profileName = origProfileName }()
	profileName = ""

	// timeout=0: previously hidden by the "0" skip, now shown.
	testCfg := &izanami.ResolvedConfig{Timeout: 0}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logEffectiveConfig(cmd, testCfg)

	output := buf.String()
	assert.Contains(t, output, "timeout=0", "timeout=0 should appear in verbose output (not skipped)")
}

func TestLogEffectiveConfig_InsecureFalseSkipped(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	origProfileName := profileName
	defer func() { profileName = origProfileName }()
	profileName = ""

	testCfg := &izanami.ResolvedConfig{
		Timeout:            30,
		InsecureSkipVerify: false,
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logEffectiveConfig(cmd, testCfg)

	output := buf.String()
	assert.NotContains(t, output, "insecure=false", "insecure=false should be hidden (uninteresting default)")
}

func TestLogEffectiveConfig_InsecureTrueShown(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	origProfileName := profileName
	defer func() { profileName = origProfileName }()
	profileName = ""

	testCfg := &izanami.ResolvedConfig{
		Timeout:            30,
		InsecureSkipVerify: true,
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logEffectiveConfig(cmd, testCfg)

	output := buf.String()
	assert.Contains(t, output, "insecure=true", "insecure=true should be shown")
}

func TestLogEffectiveConfig_EmptyFieldsSkipped(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	origProfileName := profileName
	defer func() { profileName = origProfileName }()
	profileName = ""

	// Only timeout has a value; string fields are empty
	testCfg := &izanami.ResolvedConfig{Timeout: 30}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logEffectiveConfig(cmd, testCfg)

	output := buf.String()
	assert.NotContains(t, output, "leader-url=", "empty base-url should be skipped")
	assert.NotContains(t, output, "tenant=", "empty tenant should be skipped")
	assert.NotContains(t, output, "project=", "empty project should be skipped")
	assert.NotContains(t, output, "context=", "empty context should be skipped")
	assert.NotContains(t, output, "client-id=", "empty client-id should be skipped")
	assert.NotContains(t, output, "client-secret=", "empty client-secret should be skipped")
	assert.NotContains(t, output, "leader-url=", "empty leader-url should be skipped")
}

func TestLogEffectiveConfig_SensitiveFieldsRedacted(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	origProfileName := profileName
	defer func() { profileName = origProfileName }()
	profileName = ""

	testCfg := &izanami.ResolvedConfig{
		Timeout:      30,
		ClientSecret: "my-super-secret",
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logEffectiveConfig(cmd, testCfg)

	output := buf.String()
	assert.Contains(t, output, "client-secret=<redacted>", "client-secret should be redacted")
	assert.NotContains(t, output, "my-super-secret", "actual secret must not appear")
}

func TestLogEffectiveConfig_OutputFormat(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	origProfileName := profileName
	defer func() { profileName = origProfileName }()
	profileName = ""

	testCfg := &izanami.ResolvedConfig{
		LeaderURL: "http://localhost:9000",
		Timeout:   30,
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logEffectiveConfig(cmd, testCfg)

	output := buf.String()
	// Every config line must follow the format: [verbose] Config: key=value (source: ...)
	assert.Contains(t, output, "[verbose] Config: leader-url=http://localhost:9000 (source: ", "output should follow [verbose] Config format")
	assert.Contains(t, output, "[verbose] Config: timeout=30 (source: ", "timeout line should follow format")
}

func TestLogEffectiveConfig_WithProfile(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Create config file with a profile
	profiles := map[string]*izanami.Profile{
		"sandbox": {
			LeaderURL: "http://sandbox.example.com",
			Tenant:    "sandbox-tenant",
		},
	}
	createConfigTestFile(t, paths.configPath, profiles, "sandbox")

	origProfileName := profileName
	defer func() { profileName = origProfileName }()
	profileName = ""

	testCfg := &izanami.ResolvedConfig{
		LeaderURL: "http://sandbox.example.com",
		Tenant:    "sandbox-tenant",
		Timeout:   30,
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logEffectiveConfig(cmd, testCfg)

	output := buf.String()
	assert.Contains(t, output, "leader-url=http://sandbox.example.com (source: profile)", "base-url should come from profile")
	assert.Contains(t, output, "tenant=sandbox-tenant (source: profile)", "tenant should come from profile")
}

// ============================================================================
// determineConfigSource tests
// ============================================================================

func TestDetermineConfigSource_Flag(t *testing.T) {
	cmd := setupVerboseTestCommand()
	// Mark the "url" flag as changed
	require.NoError(t, cmd.Flags().Set("url", "http://flag.example.com"))

	field := configFieldInfo{key: "leader-url", flagName: "url"}

	source := determineConfigSource(cmd, field, nil, nil)
	assert.Equal(t, "flag", source, "should return 'flag' when flag is explicitly set")
}

func TestDetermineConfigSource_EnvVar(t *testing.T) {
	cmd := setupVerboseTestCommand()

	t.Setenv("IZ_LEADER_URL", "http://env.example.com")

	field := configFieldInfo{key: "leader-url", flagName: "url", envVar: "IZ_LEADER_URL"}

	source := determineConfigSource(cmd, field, nil, nil)
	assert.Equal(t, "env", source, "should return 'env' when env var is set")
}

func TestDetermineConfigSource_FlagOverridesEnv(t *testing.T) {
	cmd := setupVerboseTestCommand()
	require.NoError(t, cmd.Flags().Set("url", "http://flag.example.com"))

	t.Setenv("IZ_LEADER_URL", "http://env.example.com")

	field := configFieldInfo{key: "leader-url", flagName: "url", envVar: "IZ_LEADER_URL"}

	source := determineConfigSource(cmd, field, nil, nil)
	assert.Equal(t, "flag", source, "flag should take priority over env var")
}

func TestDetermineConfigSource_Session(t *testing.T) {
	cmd := setupVerboseTestCommand()

	session := &izanami.Session{
		URL: "http://session.example.com",
	}

	field := configFieldInfo{key: "leader-url", flagName: "url", envVar: "IZ_LEADER_URL"}

	// Session URL used when profile has no base-url
	source := determineConfigSource(cmd, field, nil, session)
	assert.Equal(t, "session", source, "should return 'session' when session has URL and no profile base-url")
}

func TestDetermineConfigSource_SessionNotUsedWhenProfileHasURL(t *testing.T) {
	cmd := setupVerboseTestCommand()

	session := &izanami.Session{
		URL: "http://session.example.com",
	}
	profile := &izanami.Profile{
		LeaderURL: "http://profile.example.com",
	}

	field := configFieldInfo{key: "leader-url", flagName: "url", envVar: "IZ_LEADER_URL"}

	source := determineConfigSource(cmd, field, profile, session)
	assert.Equal(t, "profile", source, "should return 'profile' when profile has base-url (not session)")
}

func TestDetermineConfigSource_Profile(t *testing.T) {
	cmd := setupVerboseTestCommand()

	profile := &izanami.Profile{
		Tenant: "my-tenant",
	}

	field := configFieldInfo{key: "tenant", flagName: "tenant", envVar: "IZ_TENANT"}

	source := determineConfigSource(cmd, field, profile, nil)
	assert.Equal(t, "profile", source, "should return 'profile' when profile has a value")
}

func TestDetermineConfigSource_ProfileInsecureFalseNotReturned(t *testing.T) {
	cmd := setupVerboseTestCommand()

	// Profile with insecure=false — should NOT return "profile" since false
	// is the default and indistinguishable from unset.
	profile := &izanami.Profile{
		InsecureSkipVerify: false,
	}

	field := configFieldInfo{key: "insecure", flagName: "insecure"}

	source := determineConfigSource(cmd, field, profile, nil)
	assert.NotEqual(t, "profile", source, "insecure=false should not be attributed to profile")
}

func TestDetermineConfigSource_GlobalConfigKey_FileSource(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Create config file with timeout explicitly set
	createConfigTestFile(t, paths.configPath, nil, "")
	require.NoError(t, izanami.SetConfigValue("timeout", "60"))

	cmd := setupVerboseTestCommand()

	field := configFieldInfo{key: "timeout", flagName: "timeout"}

	source := determineConfigSource(cmd, field, nil, nil)
	assert.Equal(t, "file", source, "should return 'file' when timeout is set in config.yaml")
}

func TestDetermineConfigSource_GlobalConfigKey_DefaultSource(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Write a minimal config file that does NOT include timeout,
	// so GetConfigValue sees it's not in the file and returns "default".
	createTestFile(t, paths.configPath, "verbose: false\ncolor: auto\n", 0600)

	cmd := setupVerboseTestCommand()

	field := configFieldInfo{key: "timeout", flagName: "timeout"}

	source := determineConfigSource(cmd, field, nil, nil)
	assert.Equal(t, "default", source, "should return 'default' when timeout uses viper default")
}

func TestDetermineConfigSource_GlobalConfigKey_EnvSource(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Write a minimal config file WITHOUT timeout
	createTestFile(t, paths.configPath, "verbose: false\ncolor: auto\n", 0600)

	// Set env var matching what convertToEnvKey("timeout") actually produces.
	// Note: convertToEnvKey replaces hyphens with underscores but does not
	// uppercase, so GetConfigValue looks for "IZ_timeout" (not "IZ_TIMEOUT").
	t.Setenv("IZ_timeout", "10")

	cmd := setupVerboseTestCommand()

	// timeout has no envVar in configFields (intentionally), so step 2 won't
	// fire. The GetConfigValue fallback (step 5) should detect the env var.
	field := configFieldInfo{key: "timeout", flagName: "timeout"}

	source := determineConfigSource(cmd, field, nil, nil)
	assert.Equal(t, "env", source, "should return 'env' when IZ_timeout env var is set via GetConfigValue fallback")
}

func TestDetermineConfigSource_FallbackDefault(t *testing.T) {
	cmd := setupVerboseTestCommand()

	// A field that is not a GlobalConfigKey and has no flag/env/profile/session
	// should fall through to "default".
	field := configFieldInfo{key: "leader-url", envVar: "IZ_CLIENT_BASE_URL"}

	source := determineConfigSource(cmd, field, nil, nil)
	assert.Equal(t, "default", source, "non-global key with no other source should return 'default'")
}

// ============================================================================
// getProfileFieldValue tests
// ============================================================================

func TestGetProfileFieldValue_AllFields(t *testing.T) {
	profile := &izanami.Profile{
		LeaderURL:          "http://example.com",
		DefaultWorker:      "eu-west",
		ClientID:           "cid",
		ClientSecret:       "csecret",
		Tenant:             "my-tenant",
		Project:            "my-project",
		Context:            "prod",
		InsecureSkipVerify: true,
	}

	tests := []struct {
		key      string
		expected string
	}{
		{"leader-url", "http://example.com"},
		{"default-worker", "eu-west"},
		{"client-id", "cid"},
		{"client-secret", "csecret"},
		{"tenant", "my-tenant"},
		{"project", "my-project"},
		{"context", "prod"},
		{"insecure", "true"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := getProfileFieldValue(profile, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetProfileFieldValue_UnknownKey(t *testing.T) {
	profile := &izanami.Profile{LeaderURL: "http://example.com"}

	result := getProfileFieldValue(profile, "unknown-key")
	assert.Equal(t, "", result, "unknown key should return empty string")
}

func TestGetProfileFieldValue_InsecureFalse(t *testing.T) {
	profile := &izanami.Profile{
		InsecureSkipVerify: false,
	}

	result := getProfileFieldValue(profile, "insecure")
	assert.Equal(t, "false", result, "insecure=false should return 'false'")
}

// ============================================================================
// logEnvironmentVariables tests
// ============================================================================

func TestLogEnvironmentVariables_NoVars(t *testing.T) {
	// Clear all IZ_* vars
	for _, env := range os.Environ() {
		if len(env) > 3 && env[:3] == "IZ_" {
			key, _, _ := splitEnv(env)
			t.Setenv(key, "")
			os.Unsetenv(key)
		}
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logEnvironmentVariables(cmd)

	output := buf.String()
	assert.Contains(t, output, "no IZ_* variables set", "should show 'no IZ_ variables' message")
}

func TestLogEnvironmentVariables_WithVars(t *testing.T) {
	t.Setenv("IZ_LEADER_URL", "http://test.example.com")
	t.Setenv("IZ_TENANT", "test-tenant")

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logEnvironmentVariables(cmd)

	output := buf.String()
	assert.Contains(t, output, "IZ_LEADER_URL=http://test.example.com")
	assert.Contains(t, output, "IZ_TENANT=test-tenant")
}

func TestLogEnvironmentVariables_SensitiveRedacted(t *testing.T) {
	t.Setenv("IZ_CLIENT_SECRET", "super-secret-value")

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logEnvironmentVariables(cmd)

	output := buf.String()
	assert.Contains(t, output, "IZ_CLIENT_SECRET=<redacted>")
	assert.NotContains(t, output, "super-secret-value")
}

// ============================================================================
// logAuthenticationMode tests
// ============================================================================

func TestLogAuthenticationMode_NoAuth(t *testing.T) {
	testCfg := &izanami.ResolvedConfig{}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logAuthenticationMode(cmd, testCfg)

	output := buf.String()
	assert.Contains(t, output, "Admin operations: none")
	assert.Contains(t, output, "Feature checks: none")
}

func TestLogAuthenticationMode_PAT(t *testing.T) {
	testCfg := &izanami.ResolvedConfig{
		PersonalAccessToken: "pat-123",
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logAuthenticationMode(cmd, testCfg)

	output := buf.String()
	assert.Contains(t, output, "Admin operations: Personal Access Token (PAT)")
}

func TestLogAuthenticationMode_JWT(t *testing.T) {
	testCfg := &izanami.ResolvedConfig{
		JwtToken: "jwt-token-here",
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logAuthenticationMode(cmd, testCfg)

	output := buf.String()
	assert.Contains(t, output, "Admin operations: JWT Cookie (session)")
}

func TestLogAuthenticationMode_ClientKey(t *testing.T) {
	testCfg := &izanami.ResolvedConfig{
		ClientID:     "cid",
		ClientSecret: "csecret",
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&buf)

	logAuthenticationMode(cmd, testCfg)

	output := buf.String()
	assert.Contains(t, output, "Feature checks: Client API Key")
}

// splitEnv splits "KEY=VALUE" into key and value.
func splitEnv(s string) (string, string, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}
