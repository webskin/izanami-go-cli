package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/webskin/izanami-go-cli/internal/izanami"
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
