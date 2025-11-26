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

	// Verify we have username and password
	if env.Username == "" || env.Password == "" {
		t.Skip("IZ_TEST_USERNAME or IZ_TEST_PASSWORD not set")
	}

	// Test using performLogin directly (simpler, no interactive prompts)
	token, err := performLogin(env.BaseURL, env.Username, env.Password)
	require.NoError(t, err, "Login should succeed with valid credentials")
	assert.NotEmpty(t, token, "Should receive a JWT token")

	t.Logf("Successfully logged in, received token of length %d", len(token))
}

// TestIntegration_LoginWithInvalidCredentials tests that login fails with invalid credentials
func TestIntegration_LoginWithInvalidCredentials(t *testing.T) {
	env := setupIntegrationTest(t)

	// Test with invalid password
	_, err := performLogin(env.BaseURL, "invalid_user", "invalid_password")
	require.Error(t, err, "Login should fail with invalid credentials")

	t.Logf("Login correctly failed with error: %v", err)
}

// TestIntegration_LoginCommand tests the full login command flow
func TestIntegration_LoginCommand(t *testing.T) {
	env := setupIntegrationTest(t)

	if env.Username == "" || env.Password == "" {
		t.Skip("IZ_TEST_USERNAME or IZ_TEST_PASSWORD not set")
	}

	// Setup command with stdin for profile name prompt
	var buf bytes.Buffer
	input := bytes.NewBufferString("testprofile\n") // Answer for profile name prompt

	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(loginCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(input)
	loginCmd.SetOut(&buf)
	loginCmd.SetErr(&buf)
	loginCmd.SetIn(input)

	// Execute login command with --password flag to avoid interactive password prompt
	cmd.SetArgs([]string{"login", env.BaseURL, env.Username, "--password", env.Password})

	err := cmd.Execute()

	// Cleanup command state
	loginCmd.SetIn(nil)
	loginCmd.SetOut(nil)
	loginCmd.SetErr(nil)

	output := buf.String()
	t.Logf("Command output:\n%s", output)

	require.NoError(t, err, "Login command should succeed")
	assert.Contains(t, output, "Successfully logged in", "Should show success message")

	// Verify session file was created
	assert.True(t, env.SessionsFileExists(), "Sessions file should exist after login")

	// Verify session content
	sessions, err := izanami.LoadSessions()
	require.NoError(t, err)
	assert.NotEmpty(t, sessions.Sessions, "Should have at least one session")

	// Find and verify the session
	var foundSession *izanami.Session
	for _, session := range sessions.Sessions {
		if session.URL == env.BaseURL && session.Username == env.Username {
			foundSession = session
			break
		}
	}
	require.NotNil(t, foundSession, "Should find session for our URL and username")
	assert.NotEmpty(t, foundSession.JwtToken, "Session should have JWT token")
}

// TestIntegration_LogoutAfterLogin tests logout after a successful login
func TestIntegration_LogoutAfterLogin(t *testing.T) {
	env := setupIntegrationTest(t)

	if env.Username == "" || env.Password == "" {
		t.Skip("IZ_TEST_USERNAME or IZ_TEST_PASSWORD not set")
	}

	// First, login using the command to set up profile and session
	var loginBuf bytes.Buffer
	loginInput := bytes.NewBufferString("testprofile\n")

	loginParent := &cobra.Command{Use: "iz"}
	loginParent.AddCommand(loginCmd)
	loginParent.SetOut(&loginBuf)
	loginParent.SetErr(&loginBuf)
	loginParent.SetIn(loginInput)
	loginCmd.SetOut(&loginBuf)
	loginCmd.SetErr(&loginBuf)
	loginCmd.SetIn(loginInput)

	loginParent.SetArgs([]string{"login", env.BaseURL, env.Username, "--password", env.Password})
	err := loginParent.Execute()

	loginCmd.SetIn(nil)
	loginCmd.SetOut(nil)
	loginCmd.SetErr(nil)

	require.NoError(t, err, "Login should succeed before testing logout")

	// Verify we have a session with a token
	sessions, err := izanami.LoadSessions()
	require.NoError(t, err)
	var sessionName string
	var originalToken string
	for name, session := range sessions.Sessions {
		if session.URL == env.BaseURL {
			sessionName = name
			originalToken = session.JwtToken
			break
		}
	}
	require.NotEmpty(t, sessionName, "Should have a session")
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
	err = logoutParent.Execute()

	logoutCmd.SetOut(nil)
	logoutCmd.SetErr(nil)

	logoutOutput := logoutBuf.String()
	t.Logf("Logout output:\n%s", logoutOutput)

	require.NoError(t, err, "Logout should succeed")
	assert.Contains(t, logoutOutput, "Logged out", "Should show logout success message")

	// Verify the session token is cleared
	sessions, err = izanami.LoadSessions()
	require.NoError(t, err)

	session, err := sessions.GetSession(sessionName)
	require.NoError(t, err, "Session should still exist")
	assert.Empty(t, session.JwtToken, "JWT token should be cleared after logout")
}

// TestIntegration_LogoutWithoutSession tests logout when there's no active session
func TestIntegration_LogoutWithoutSession(t *testing.T) {
	_ = setupIntegrationTest(t) // Sets up isolated environment

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

	if env.Username == "" || env.Password == "" {
		t.Skip("IZ_TEST_USERNAME or IZ_TEST_PASSWORD not set")
	}

	// Verify no sessions file exists initially
	assert.False(t, env.SessionsFileExists(), "Sessions file should not exist before login")

	// Perform login
	token, err := performLogin(env.BaseURL, env.Username, env.Password)
	require.NoError(t, err, "Login should succeed")
	require.NotEmpty(t, token, "Should receive a token")

	// Save the session manually (as the login command would do)
	sessions, err := izanami.LoadSessions()
	require.NoError(t, err)

	sessions.AddSession("test-session", &izanami.Session{
		URL:      env.BaseURL,
		Username: env.Username,
		JwtToken: token,
	})

	err = sessions.Save()
	require.NoError(t, err, "Should save session")

	// Verify sessions file was created
	assert.True(t, env.SessionsFileExists(), "Sessions file should exist after saving session")

	// Verify content
	content := env.ReadSessionsFile(t)
	assert.Contains(t, content, "test-session", "Sessions file should contain session name")
	assert.Contains(t, content, env.BaseURL, "Sessions file should contain URL")
}
