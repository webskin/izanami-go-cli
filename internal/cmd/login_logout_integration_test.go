package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// TestIntegration_LoginWithValidCredentials tests that login succeeds with valid credentials
func TestIntegration_LoginWithValidCredentials(t *testing.T) {
	env := setupIntegrationTest(t)

	token := env.Login(t)
	assert.NotEmpty(t, token, "Should receive a JWT token")

	t.Logf("Successfully logged in, received token of length %d", len(token))
}

// TestIntegration_LoginWithInvalidCredentials tests that login fails with invalid credentials
func TestIntegration_LoginWithInvalidCredentials(t *testing.T) {
	env := setupIntegrationTest(t)

	// Test with invalid password (use performLogin directly for invalid creds)
	_, err := performLogin(env.LeaderURL, "invalid_user", "invalid_password")
	require.Error(t, err, "Login should fail with invalid credentials")

	t.Logf("Login correctly failed with error: %v", err)
}

// TestIntegration_LoginCommand tests the full login command flow
func TestIntegration_LoginCommand(t *testing.T) {
	env := setupIntegrationTest(t)

	token := env.Login(t)

	// Verify session file was created
	assert.True(t, env.SessionsFileExists(), "Sessions file should exist after login")

	// Verify session content
	sessions, err := izanami.LoadSessions()
	require.NoError(t, err)
	assert.NotEmpty(t, sessions.Sessions, "Should have at least one session")

	// Find and verify the session
	var foundSession *izanami.Session
	for _, session := range sessions.Sessions {
		if session.URL == env.LeaderURL && session.Username == env.Username {
			foundSession = session
			break
		}
	}
	require.NotNil(t, foundSession, "Should find session for our URL and username")
	assert.Equal(t, token, foundSession.JwtToken, "Session should have the returned JWT token")
}

// TestIntegration_LogoutAfterLogin tests logout after a successful login
func TestIntegration_LogoutAfterLogin(t *testing.T) {
	env := setupIntegrationTest(t)

	// Login first
	originalToken := env.Login(t)
	require.NotEmpty(t, originalToken, "Session should have a token before logout")

	// Now logout
	var logoutBuf bytes.Buffer
	logoutParent := &cobra.Command{Use: "iz"}
	logoutParent.AddCommand(logoutCmd)
	logoutParent.SetOut(&logoutBuf)
	logoutParent.SetErr(&logoutBuf)
	logoutCmd.SetOut(&logoutBuf)
	logoutCmd.SetErr(&logoutBuf)

	logoutParent.SetArgs([]string{"logout"})
	err := logoutParent.Execute()

	logoutCmd.SetOut(nil)
	logoutCmd.SetErr(nil)

	logoutOutput := logoutBuf.String()
	t.Logf("Logout output:\n%s", logoutOutput)

	require.NoError(t, err, "Logout should succeed")
	assert.Contains(t, logoutOutput, "Logged out", "Should show logout success message")

	// Verify the session token is cleared
	sessions, err := izanami.LoadSessions()
	require.NoError(t, err)

	// Find the session and verify token is cleared
	for _, session := range sessions.Sessions {
		if session.URL == env.LeaderURL {
			assert.Empty(t, session.JwtToken, "JWT token should be cleared after logout")
			break
		}
	}
}

// TestIntegration_LogoutWithoutSession tests logout when there's no active session
func TestIntegration_LogoutWithoutSession(t *testing.T) {
	_ = setupIntegrationTest(t) // Sets up isolated environment, no login

	// Don't login, just try to logout
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(logoutCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	logoutCmd.SetOut(&buf)
	logoutCmd.SetErr(&buf)

	cmd.SetArgs([]string{"logout"})
	err := cmd.Execute()

	logoutCmd.SetOut(nil)
	logoutCmd.SetErr(nil)

	// Should fail because there's no active profile
	require.Error(t, err, "Logout should fail when there's no active profile")

	output := buf.String()
	t.Logf("Logout error output:\n%s", output)

	// Error should mention no active profile
	assert.True(t,
		strings.Contains(err.Error(), "no active profile") ||
			strings.Contains(err.Error(), "profile"),
		"Error should mention profile issue")
}

// TestIntegration_LoginCreatesSessionFile tests that login creates the sessions file
func TestIntegration_LoginCreatesSessionFile(t *testing.T) {
	env := setupIntegrationTest(t)

	// Verify no sessions file exists initially
	assert.False(t, env.SessionsFileExists(), "Sessions file should not exist before login")

	// Perform login using helper
	env.Login(t)

	// Verify sessions file was created
	assert.True(t, env.SessionsFileExists(), "Sessions file should exist after login")

	// Verify content
	content := env.ReadSessionsFile(t)
	assert.Contains(t, content, env.LeaderURL, "Sessions file should contain URL")
}

// TestIntegration_AuthenticatedClient tests that NewAuthenticatedClient works
func TestIntegration_AuthenticatedClient(t *testing.T) {
	env := setupIntegrationTest(t)

	// Login first
	env.Login(t)

	// Create authenticated client
	client := env.NewAuthenticatedClient(t)
	require.NotNil(t, client, "Should create authenticated client")

	// Verify client can make authenticated requests (health check doesn't need auth, but tests client creation)
	t.Log("Authenticated client created successfully")
}

// TestIntegration_LoginOneArg_Username tests login with a single username arg
// when the URL is available from a prior session.
func TestIntegration_LoginOneArg_Username(t *testing.T) {
	env := setupIntegrationTest(t)

	// First: full login to establish session and profile
	env.Login(t)

	// Second: login with username only (URL resolved from session)
	var buf bytes.Buffer
	input := bytes.NewBufferString("default\n")

	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(loginCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(input)
	loginCmd.SetOut(&buf)
	loginCmd.SetErr(&buf)
	loginCmd.SetIn(input)

	cmd.SetArgs([]string{"login", env.Username, "--password", env.Password})
	err := cmd.Execute()

	loginCmd.SetIn(nil)
	loginCmd.SetOut(nil)
	loginCmd.SetErr(nil)

	output := buf.String()
	t.Logf("Login one-arg username output:\n%s", output)

	require.NoError(t, err, "Login with username only should succeed")
	assert.Contains(t, output, "Successfully logged in")
}

// TestIntegration_LoginOneArg_URL tests login with a single URL arg
// when the username is available from a prior session.
func TestIntegration_LoginOneArg_URL(t *testing.T) {
	env := setupIntegrationTest(t)

	// First: full login to establish session and profile
	env.Login(t)

	// Second: login with URL only (username resolved from session)
	var buf bytes.Buffer
	input := bytes.NewBufferString("default\n")

	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(loginCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(input)
	loginCmd.SetOut(&buf)
	loginCmd.SetErr(&buf)
	loginCmd.SetIn(input)

	cmd.SetArgs([]string{"login", env.LeaderURL, "--password", env.Password})
	err := cmd.Execute()

	loginCmd.SetIn(nil)
	loginCmd.SetOut(nil)
	loginCmd.SetErr(nil)

	output := buf.String()
	t.Logf("Login one-arg URL output:\n%s", output)

	require.NoError(t, err, "Login with URL only should succeed")
	assert.Contains(t, output, "Successfully logged in")
}

// TestIntegration_LoginZeroArgs_ReusesSession tests login with zero args
// when both URL and username are available from a prior session.
func TestIntegration_LoginZeroArgs_ReusesSession(t *testing.T) {
	env := setupIntegrationTest(t)

	// First: full login to establish session and profile
	env.Login(t)

	// Second: login with zero args (both resolved from session)
	var buf bytes.Buffer
	input := bytes.NewBufferString("default\n")

	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(loginCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(input)
	loginCmd.SetOut(&buf)
	loginCmd.SetErr(&buf)
	loginCmd.SetIn(input)

	cmd.SetArgs([]string{"login", "--password", env.Password})
	err := cmd.Execute()

	loginCmd.SetIn(nil)
	loginCmd.SetOut(nil)
	loginCmd.SetErr(nil)

	output := buf.String()
	t.Logf("Login zero-args output:\n%s", output)

	require.NoError(t, err, "Login with zero args should succeed")
	assert.Contains(t, output, "Successfully logged in")
}

// TestIntegration_LoginPasswordCustomSessionName tests that the --name flag
// is respected in the password login flow, saving the session with the custom name.
func TestIntegration_LoginPasswordCustomSessionName(t *testing.T) {
	env := setupIntegrationTest(t)

	if env.Username == "" || env.Password == "" {
		t.Skip("IZ_TEST_USERNAME or IZ_TEST_PASSWORD not set")
	}

	var buf bytes.Buffer
	input := bytes.NewBufferString("default\n") // Profile name

	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(loginCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(input)
	loginCmd.SetOut(&buf)
	loginCmd.SetErr(&buf)
	loginCmd.SetIn(input)

	customName := "my-custom-session"
	cmd.SetArgs([]string{"login", env.LeaderURL, env.Username, "--password", env.Password, "--name", customName})
	err := cmd.Execute()

	loginCmd.SetIn(nil)
	loginCmd.SetOut(nil)
	loginCmd.SetErr(nil)
	loginSessionName = "" // Reset global flag

	output := buf.String()
	t.Logf("Login custom session name output:\n%s", output)

	require.NoError(t, err, "Login with --name should succeed")
	assert.Contains(t, output, "Successfully logged in")
	assert.Contains(t, output, customName)

	// Verify the session was saved with the custom name
	sessions, err := izanami.LoadSessions()
	require.NoError(t, err, "Should load sessions")

	_, exists := sessions.Sessions[customName]
	assert.True(t, exists, "Session should be saved with custom name %q, found keys: %v",
		customName, func() []string {
			keys := make([]string, 0, len(sessions.Sessions))
			for k := range sessions.Sessions {
				keys = append(keys, k)
			}
			return keys
		}())
}
