package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"gopkg.in/yaml.v3"
)

// setupLoginCommand sets up loginCmd with proper I/O streams for testing.
// Returns the root command and a cleanup function that resets streams and
// the verbose/baseURL global flags.
func setupLoginCommand(buf *bytes.Buffer, input *bytes.Buffer, args []string) (*cobra.Command, func()) {
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(loginCmd)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if input != nil {
		cmd.SetIn(input)
		loginCmd.SetIn(input)
	}
	loginCmd.SetOut(buf)
	loginCmd.SetErr(buf)
	cmd.SetArgs(args)

	cleanup := func() {
		loginCmd.SetIn(nil)
		loginCmd.SetOut(nil)
		loginCmd.SetErr(nil)
		// Reset global flags that tests may modify
		verbose = false
		baseURL = ""
		profileName = ""
		loginPassword = ""
		loginOIDC = false
	}

	return cmd, cleanup
}

// TestLogin_TwoArgs_URLAndUsername verifies that two args set URL and username
// directly, reaching the password/auth phase (which fails with a connection error
// since there's no server).
func TestLogin_TwoArgs_URLAndUsername(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	var buf bytes.Buffer
	cmd, cleanup := setupLoginCommand(&buf, nil, []string{
		"login", "http://localhost:9999", "testuser", "--password", "dummy",
	})
	defer cleanup()

	err := cmd.Execute()
	// Will fail with connection error, but that proves both args were accepted
	require.Error(t, err)
	assert.Contains(t, err.Error(), "login failed")
}

// TestLogin_OneArg_Username_ResolvesURL verifies that a single non-URL argument
// is treated as a username, and the URL is resolved from the active profile/session.
func TestLogin_OneArg_Username_ResolvesURL(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Create session with a URL
	createTestSessions(t, paths.sessionsPath, map[string]*izanami.Session{
		"my-session": {URL: "http://localhost:9999", Username: "olduser"},
	})
	// Create config with active profile referencing the session
	createTestConfig(t, paths.configPath, map[string]*izanami.Profile{
		"test": {Session: "my-session"},
	}, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupLoginCommand(&buf, nil, []string{
		"login", "newuser", "--password", "dummy",
	})
	defer cleanup()

	err := cmd.Execute()
	// Should reach the auth phase (connection error), proving URL was resolved
	require.Error(t, err)
	assert.Contains(t, err.Error(), "login failed")
	// The output should show authenticating against the session's URL
	output := buf.String()
	assert.Contains(t, output, "http://localhost:9999")
}

// TestLogin_OneArg_Username_NoURLAvailable verifies that providing only a username
// errors when no URL can be resolved.
func TestLogin_OneArg_Username_NoURLAvailable(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// No config, no sessions, no env
	var buf bytes.Buffer
	cmd, cleanup := setupLoginCommand(&buf, nil, []string{
		"login", "testuser", "--password", "dummy",
	})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no base URL available for login")
	assert.Contains(t, err.Error(), "iz login <url> testuser")
}

// TestLogin_OneArg_URL_ResolvesUsername verifies that a single URL argument
// resolves the username from the active session.
func TestLogin_OneArg_URL_ResolvesUsername(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	createTestSessions(t, paths.sessionsPath, map[string]*izanami.Session{
		"my-session": {URL: "http://old-url.com", Username: "testuser"},
	})
	createTestConfig(t, paths.configPath, map[string]*izanami.Profile{
		"test": {Session: "my-session"},
	}, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupLoginCommand(&buf, nil, []string{
		"login", "http://localhost:9999", "--password", "dummy",
	})
	defer cleanup()

	err := cmd.Execute()
	// Connection error proves resolution worked and proceeded to auth
	require.Error(t, err)
	assert.Contains(t, err.Error(), "login failed")
	output := buf.String()
	assert.Contains(t, output, "http://localhost:9999")
}

// TestLogin_OneArg_URL_NoUsernameAvailable verifies that providing only a URL
// errors when no username can be resolved from the session.
func TestLogin_OneArg_URL_NoUsernameAvailable(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// No config or sessions
	var buf bytes.Buffer
	cmd, cleanup := setupLoginCommand(&buf, nil, []string{
		"login", "http://localhost:9999", "--password", "dummy",
	})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no username available for login")
	assert.Contains(t, err.Error(), "iz login http://localhost:9999 <username>")
}

// TestLogin_ZeroArgs_ResolvesBoth verifies that zero args resolves both URL and
// username from the active session.
func TestLogin_ZeroArgs_ResolvesBoth(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	createTestSessions(t, paths.sessionsPath, map[string]*izanami.Session{
		"my-session": {URL: "http://localhost:9999", Username: "testuser"},
	})
	createTestConfig(t, paths.configPath, map[string]*izanami.Profile{
		"test": {Session: "my-session"},
	}, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupLoginCommand(&buf, nil, []string{
		"login", "--password", "dummy",
	})
	defer cleanup()

	err := cmd.Execute()
	// Connection error proves both URL and username were resolved
	require.Error(t, err)
	assert.Contains(t, err.Error(), "login failed")
	output := buf.String()
	assert.Contains(t, output, "http://localhost:9999")
}

// TestLogin_ZeroArgs_NoURLOrUsername verifies that zero args with no config/session
// returns an informative error.
func TestLogin_ZeroArgs_NoURLOrUsername(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	var buf bytes.Buffer
	cmd, cleanup := setupLoginCommand(&buf, nil, []string{
		"login", "--password", "dummy",
	})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no URL or username available for login")
}

// TestLogin_ZeroArgs_URLOnly_NoUsername verifies that when only URL is available
// but no username, a specific error is returned.
func TestLogin_ZeroArgs_URLOnly_NoUsername(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Session has URL but no username
	createTestSessions(t, paths.sessionsPath, map[string]*izanami.Session{
		"my-session": {URL: "http://localhost:9999", Username: ""},
	})
	// Profile with BaseURL set directly (no session username)
	createTestConfig(t, paths.configPath, map[string]*izanami.Profile{
		"test": {BaseURL: "http://localhost:9999"},
	}, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupLoginCommand(&buf, nil, []string{
		"login", "--password", "dummy",
	})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no username available")
}

// TestLogin_ZeroArgs_UsernameOnly_NoURL verifies that when only username is
// available but no URL, a specific error is returned.
func TestLogin_ZeroArgs_UsernameOnly_NoURL(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Session has username but URL is empty (no BaseURL in profile either)
	createTestSessions(t, paths.sessionsPath, map[string]*izanami.Session{
		"my-session": {URL: "", Username: "testuser"},
	})
	createTestConfig(t, paths.configPath, map[string]*izanami.Profile{
		"test": {Session: "my-session"},
	}, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupLoginCommand(&buf, nil, []string{
		"login", "--password", "dummy",
	})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no base URL available")
}

// TestLogin_ResolveDefaults_VerboseConfigError verifies that when verbose is enabled
// and config loading fails, the error is logged to stderr.
func TestLogin_ResolveDefaults_VerboseConfigError(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Create a config with an active profile name that references a non-existent profile.
	// This causes LoadConfigWithProfile to fail when trying to load the profile.
	createTestConfig(t, paths.configPath, nil, "nonexistent-profile")

	verbose = true

	var buf bytes.Buffer
	cmd, cleanup := setupLoginCommand(&buf, nil, []string{
		"login", "--password", "dummy",
	})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	output := buf.String()
	assert.Contains(t, output, "[verbose] Could not load config for defaults")
}

// TestLogin_ResolveDefaults_FlagURL verifies that --url flag takes priority
// over profile URL.
func TestLogin_ResolveDefaults_FlagURL(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Session with a different URL
	createTestSessions(t, paths.sessionsPath, map[string]*izanami.Session{
		"my-session": {URL: "http://session-url.com:9000", Username: "testuser"},
	})
	createTestConfig(t, paths.configPath, map[string]*izanami.Profile{
		"test": {Session: "my-session"},
	}, "test")

	// Set the global baseURL variable to simulate --url flag
	baseURL = "http://flag-url.com:9999"

	var buf bytes.Buffer
	cmd, cleanup := setupLoginCommand(&buf, nil, []string{
		"login", "testuser", "--password", "dummy",
	})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "login failed")
	output := buf.String()
	// Should use the flag URL, not the session URL
	assert.Contains(t, output, "http://flag-url.com:9999")
	assert.NotContains(t, output, "http://session-url.com:9000")
}

// TestLogin_ResolveDefaults_EnvURL verifies that IZ_BASE_URL env var is used
// when no --url flag is set.
func TestLogin_ResolveDefaults_EnvURL(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Set env var
	t.Setenv("IZ_BASE_URL", "http://env-url.com:9999")

	var buf bytes.Buffer
	cmd, cleanup := setupLoginCommand(&buf, nil, []string{
		"login", "testuser", "--password", "dummy",
	})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "login failed")
	output := buf.String()
	assert.Contains(t, output, "http://env-url.com:9999")
}

// TestLogin_ZeroArgs_OIDCAutoDetect verifies that when a previous OIDC session
// exists and no password flag is provided, the login auto-redirects to OIDC flow.
func TestLogin_ZeroArgs_OIDCAutoDetect(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Create session with OIDC auth method
	createTestSessions(t, paths.sessionsPath, map[string]*izanami.Session{
		"my-session": {
			URL:        "http://localhost:9999",
			Username:   "oidc-user",
			AuthMethod: izanami.AuthMethodOIDC,
		},
	})
	createTestConfig(t, paths.configPath, map[string]*izanami.Profile{
		"test": {Session: "my-session"},
	}, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupLoginCommand(&buf, nil, []string{"login"})
	defer cleanup()

	err := cmd.Execute()
	// OIDC flow will fail (no server) but the error should come from the OIDC path
	require.Error(t, err)
	// The OIDC flow tries to resolve URL and check server support —
	// it should NOT ask for a password (which would be the password flow)
	assert.NotContains(t, err.Error(), "password cannot be empty")
}

// TestLogin_ZeroArgs_OIDCAutoDetect_PasswordOverride verifies that providing
// --password flag overrides OIDC auto-detection and stays in password flow.
func TestLogin_ZeroArgs_OIDCAutoDetect_PasswordOverride(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Create session with OIDC auth method
	createTestSessions(t, paths.sessionsPath, map[string]*izanami.Session{
		"my-session": {
			URL:        "http://localhost:9999",
			Username:   "oidc-user",
			AuthMethod: izanami.AuthMethodOIDC,
		},
	})
	createTestConfig(t, paths.configPath, map[string]*izanami.Profile{
		"test": {Session: "my-session"},
	}, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupLoginCommand(&buf, nil, []string{
		"login", "--password", "dummy",
	})
	defer cleanup()

	err := cmd.Execute()
	// With --password flag, should stay in password flow (connection error)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "login failed")
}

// TestLogin_OneArg_URL_OIDCAutoDetect verifies that a URL arg with an OIDC
// session auto-redirects to the OIDC flow.
func TestLogin_OneArg_URL_OIDCAutoDetect(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Create session with OIDC auth method
	createTestSessions(t, paths.sessionsPath, map[string]*izanami.Session{
		"my-session": {
			URL:        "http://localhost:9999",
			Username:   "oidc-user",
			AuthMethod: izanami.AuthMethodOIDC,
		},
	})
	createTestConfig(t, paths.configPath, map[string]*izanami.Profile{
		"test": {Session: "my-session"},
	}, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupLoginCommand(&buf, nil, []string{
		"login", "http://localhost:9999",
	})
	defer cleanup()

	err := cmd.Execute()
	// OIDC flow will fail (no server) but should NOT be a password flow error
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "password cannot be empty")
}

// TestLogin_OneArg_Username_NoOIDCAutoDetect verifies that a username arg
// (non-URL) does NOT trigger OIDC auto-detection since providing a username
// implies password-based login.
func TestLogin_OneArg_Username_NoOIDCAutoDetect(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Create session with OIDC auth method
	createTestSessions(t, paths.sessionsPath, map[string]*izanami.Session{
		"my-session": {
			URL:        "http://localhost:9999",
			Username:   "oidc-user",
			AuthMethod: izanami.AuthMethodOIDC,
		},
	})
	createTestConfig(t, paths.configPath, map[string]*izanami.Profile{
		"test": {Session: "my-session"},
	}, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupLoginCommand(&buf, nil, []string{
		"login", "newuser", "--password", "dummy",
	})
	defer cleanup()

	err := cmd.Execute()
	// Username arg takes the username-path (case 1, non-URL), which does NOT
	// check for OIDC auto-detect. Should stay in password flow.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "login failed")
}

// TestLogin_ZeroArgs_NoAuthMethod_DefaultsToPassword verifies backward
// compatibility: old sessions without AuthMethod default to password flow.
func TestLogin_ZeroArgs_NoAuthMethod_DefaultsToPassword(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Create session WITHOUT AuthMethod (backward compat)
	createTestSessions(t, paths.sessionsPath, map[string]*izanami.Session{
		"my-session": {
			URL:      "http://localhost:9999",
			Username: "testuser",
			// AuthMethod deliberately empty
		},
	})
	createTestConfig(t, paths.configPath, map[string]*izanami.Profile{
		"test": {Session: "my-session"},
	}, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupLoginCommand(&buf, nil, []string{
		"login", "--password", "dummy",
	})
	defer cleanup()

	err := cmd.Execute()
	// Should stay in password flow (connection error), not redirect to OIDC
	require.Error(t, err)
	assert.Contains(t, err.Error(), "login failed")
}

// TestDetermineProfileName_PrefersActiveProfile verifies that when multiple profiles
// share the same URL, the active profile is preferred over an arbitrary match.
func TestDetermineProfileName_PrefersActiveProfile(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Create two profiles with the same URL, "other" is active
	createTestConfig(t, paths.configPath, map[string]*izanami.Profile{
		"first":  {BaseURL: "http://localhost:9000", Tenant: "t1"},
		"second": {BaseURL: "http://localhost:9000", Tenant: "t2"},
	}, "second")

	var buf bytes.Buffer
	name, created, updated := determineProfileName(&buf, &buf, "http://localhost:9000", "user")

	assert.Equal(t, "second", name)
	assert.False(t, created)
	assert.True(t, updated)
}

// TestDetermineProfileName_FallsBackToURLMatch verifies that when the active profile
// has a different URL, login falls back to finding a profile by URL.
func TestDetermineProfileName_FallsBackToURLMatch(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	createTestConfig(t, paths.configPath, map[string]*izanami.Profile{
		"dev":  {BaseURL: "http://localhost:9000"},
		"prod": {BaseURL: "https://prod.example.com"},
	}, "dev")

	var buf bytes.Buffer
	name, created, updated := determineProfileName(&buf, &buf, "https://prod.example.com", "user")

	assert.Equal(t, "prod", name)
	assert.False(t, created)
	assert.True(t, updated)
}

// TestDetermineProfileName_ActiveProfileViaSession verifies that the active profile
// is matched even when its URL comes from a session reference.
func TestDetermineProfileName_ActiveProfileViaSession(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	createTestSessions(t, paths.sessionsPath, map[string]*izanami.Session{
		"my-session": {URL: "http://localhost:9000", Username: "user"},
	})
	createTestConfig(t, paths.configPath, map[string]*izanami.Profile{
		"via-session": {Session: "my-session"},
		"via-url":     {BaseURL: "http://localhost:9000"},
	}, "via-session")

	var buf bytes.Buffer
	name, created, updated := determineProfileName(&buf, &buf, "http://localhost:9000", "user")

	assert.Equal(t, "via-session", name)
	assert.False(t, created)
	assert.True(t, updated)
}

// TestSaveLoginSession_RefreshesExistingSessions verifies that logging in refreshes
// tokens on all sessions with the same URL+username instead of deleting them.
func TestSaveLoginSession_RefreshesExistingSessions(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Create two sessions for the same URL+username under different names
	createTestSessions(t, paths.sessionsPath, map[string]*izanami.Session{
		"profile-a-session": {URL: "http://localhost:9000", Username: "admin", JwtToken: "old-token-a"},
		"profile-b-session": {URL: "http://localhost:9000", Username: "admin", JwtToken: "old-token-b"},
		"other-session":     {URL: "http://other:9000", Username: "admin", JwtToken: "unrelated"},
	})

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetErr(&buf)

	err := saveLoginSession(cmd, "http://localhost:9000", "admin", "new-token", "password", "profile-a-session")
	require.NoError(t, err)

	// Reload sessions and verify
	sessions, err := izanami.LoadSessions()
	require.NoError(t, err)

	// Both sessions should still exist
	assert.Contains(t, sessions.Sessions, "profile-a-session")
	assert.Contains(t, sessions.Sessions, "profile-b-session")
	assert.Contains(t, sessions.Sessions, "other-session")

	// Both same-URL sessions should have the new token
	assert.Equal(t, "new-token", sessions.Sessions["profile-a-session"].JwtToken)
	assert.Equal(t, "new-token", sessions.Sessions["profile-b-session"].JwtToken)

	// Unrelated session should be untouched
	assert.Equal(t, "unrelated", sessions.Sessions["other-session"].JwtToken)
}

// TestUpdateProfileWithSession_DoesNotOverrideActiveProfile verifies that
// updateProfileWithSession does not change the active profile when one is already set.
func TestUpdateProfileWithSession_DoesNotOverrideActiveProfile(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	createTestConfig(t, paths.configPath, map[string]*izanami.Profile{
		"active-one": {BaseURL: "http://localhost:9000"},
		"other":      {BaseURL: "http://other:9000"},
	}, "active-one")

	err := updateProfileWithSession("other", "other-session")
	require.NoError(t, err)

	// Active profile should still be "active-one"
	data, err := os.ReadFile(paths.configPath)
	require.NoError(t, err)
	var config map[string]interface{}
	require.NoError(t, yaml.Unmarshal(data, &config))
	assert.Equal(t, "active-one", config["active_profile"])

	// "other" profile should have the session reference
	profilesMap := config["profiles"].(map[string]interface{})
	otherProfile := profilesMap["other"].(map[string]interface{})
	assert.Equal(t, "other-session", otherProfile["session"])
}

// TestUpdateProfileWithSession_SetsActiveWhenNoneSet verifies that
// updateProfileWithSession sets the active profile when none is currently active.
func TestUpdateProfileWithSession_SetsActiveWhenNoneSet(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	createTestConfig(t, paths.configPath, map[string]*izanami.Profile{
		"new-profile": {BaseURL: "http://localhost:9000"},
	}, "") // no active profile

	err := updateProfileWithSession("new-profile", "new-session")
	require.NoError(t, err)

	data, err := os.ReadFile(paths.configPath)
	require.NoError(t, err)
	var config map[string]interface{}
	require.NoError(t, yaml.Unmarshal(data, &config))
	assert.Equal(t, "new-profile", config["active_profile"])
}

// TestUpdateProfileWithSession_NewProfileBecomesActive verifies that a newly created
// profile becomes active even when another profile is already active.
func TestUpdateProfileWithSession_NewProfileBecomesActive(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	createTestConfig(t, paths.configPath, map[string]*izanami.Profile{
		"existing": {BaseURL: "http://localhost:9000"},
	}, "existing")

	// "brand-new" doesn't exist yet — updateProfileWithSession will create it
	err := updateProfileWithSession("brand-new", "brand-new-session")
	require.NoError(t, err)

	data, err := os.ReadFile(paths.configPath)
	require.NoError(t, err)
	var config map[string]interface{}
	require.NoError(t, yaml.Unmarshal(data, &config))
	assert.Equal(t, "brand-new", config["active_profile"])
}
