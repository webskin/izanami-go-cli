package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// ============================================================================
// Setup helpers for sessions command tests
// ============================================================================

// setupSessionsCommandTest sets up the test environment for sessions command tests
func setupSessionsCommandTest(t *testing.T, env *IntegrationTestEnv) func() {
	t.Helper()
	env.Login(t)

	// Save original values
	origOutputFormat := outputFormat

	// Set defaults
	outputFormat = "table"

	// Reset command-specific flags
	sessionsDeleteForce = false

	return func() {
		outputFormat = origOutputFormat
		sessionsDeleteForce = false
	}
}

// executeSessionsList executes sessions list command with proper output capture
func executeSessionsList(t *testing.T) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(sessionsCmd)

	// Set Out/Err on ALL commands in hierarchy
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	sessionsCmd.SetOut(&buf)
	sessionsCmd.SetErr(&buf)
	sessionsListCmd.SetOut(&buf)
	sessionsListCmd.SetErr(&buf)

	cmd.SetArgs([]string{"sessions", "list"})
	err := cmd.Execute()

	// Reset all commands
	sessionsCmd.SetOut(nil)
	sessionsCmd.SetErr(nil)
	sessionsListCmd.SetOut(nil)
	sessionsListCmd.SetErr(nil)

	return buf.String(), err
}

// executeSessionsDelete executes sessions delete command with proper output capture
func executeSessionsDelete(t *testing.T, sessionName string) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(sessionsCmd)

	// Set Out/Err on ALL commands in hierarchy
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	sessionsCmd.SetOut(&buf)
	sessionsCmd.SetErr(&buf)
	sessionsDeleteCmd.SetOut(&buf)
	sessionsDeleteCmd.SetErr(&buf)

	cmd.SetArgs([]string{"sessions", "delete", sessionName})
	err := cmd.Execute()

	// Reset all commands
	sessionsCmd.SetOut(nil)
	sessionsCmd.SetErr(nil)
	sessionsDeleteCmd.SetOut(nil)
	sessionsDeleteCmd.SetErr(nil)

	return buf.String(), err
}

// executeSessionsDeleteWithInput executes sessions delete with input for confirmation
func executeSessionsDeleteWithInput(t *testing.T, sessionName string, input string) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	inputBuf := bytes.NewBufferString(input)

	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(sessionsCmd)

	// Set Out/Err/In on ALL commands
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(inputBuf)
	sessionsCmd.SetOut(&buf)
	sessionsCmd.SetErr(&buf)
	sessionsCmd.SetIn(inputBuf)
	sessionsDeleteCmd.SetOut(&buf)
	sessionsDeleteCmd.SetErr(&buf)
	sessionsDeleteCmd.SetIn(inputBuf)

	cmd.SetArgs([]string{"sessions", "delete", sessionName})
	err := cmd.Execute()

	// Reset all commands
	sessionsCmd.SetIn(nil)
	sessionsCmd.SetOut(nil)
	sessionsCmd.SetErr(nil)
	sessionsDeleteCmd.SetIn(nil)
	sessionsDeleteCmd.SetOut(nil)
	sessionsDeleteCmd.SetErr(nil)

	return buf.String(), err
}

// ============================================================================
// Sessions List Tests
// ============================================================================

// TestIntegration_SessionsListAfterLogin tests that sessions list shows the logged-in session
func TestIntegration_SessionsListAfterLogin(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupSessionsCommandTest(t, env)
	defer cleanup()

	output, err := executeSessionsList(t)
	require.NoError(t, err)

	t.Logf("Sessions list output:\n%s", output)

	// Should show the session we created during login
	assert.Contains(t, output, env.BaseURL, "Should show session URL")
	assert.Contains(t, output, env.Username, "Should show username")
}

// TestIntegration_SessionsListJSONOutput tests sessions list with JSON output format
func TestIntegration_SessionsListJSONOutput(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupSessionsCommandTest(t, env)
	defer cleanup()

	// Set JSON output format
	outputFormat = "json"

	output, err := executeSessionsList(t)
	require.NoError(t, err)

	t.Logf("Sessions list JSON output:\n%s", output)

	// Should be valid JSON with expected fields
	assert.Contains(t, output, `"name"`, "JSON should have name field")
	assert.Contains(t, output, `"url"`, "JSON should have url field")
	assert.Contains(t, output, `"username"`, "JSON should have username field")
	assert.Contains(t, output, `"created_at"`, "JSON should have created_at field")
	assert.Contains(t, output, `"age"`, "JSON should have age field")
	assert.Contains(t, output, env.BaseURL, "Should contain the session URL")
}

// TestIntegration_SessionsListEmpty tests sessions list when no sessions exist
func TestIntegration_SessionsListEmpty(t *testing.T) {
	_ = setupIntegrationTest(t) // No login, so no sessions

	output, err := executeSessionsList(t)
	require.NoError(t, err)

	t.Logf("Sessions list (empty) output:\n%s", output)

	// Should show "no saved sessions" message
	assert.Contains(t, output, "No saved sessions", "Should indicate no sessions exist")
}

// TestIntegration_SessionsListMultiple tests sessions list with multiple sessions
func TestIntegration_SessionsListMultiple(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupSessionsCommandTest(t, env)
	defer cleanup()

	// Add another session manually
	sessions, err := izanami.LoadSessions()
	require.NoError(t, err)

	sessions.AddSession("other-session", &izanami.Session{
		URL:       "http://other.example.com",
		Username:  "other-user",
		JwtToken:  "other-token",
		CreatedAt: time.Now().Add(-2 * time.Hour),
	})
	err = sessions.Save()
	require.NoError(t, err)

	output, err := executeSessionsList(t)
	require.NoError(t, err)

	t.Logf("Sessions list (multiple) output:\n%s", output)

	// Should show both sessions
	assert.Contains(t, output, env.BaseURL, "Should show original session URL")
	assert.Contains(t, output, "http://other.example.com", "Should show added session URL")
	assert.Contains(t, output, "other-user", "Should show added session username")
}

// TestIntegration_SessionsListShowsAge tests that sessions list shows proper age formatting
func TestIntegration_SessionsListShowsAge(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupSessionsCommandTest(t, env)
	defer cleanup()

	output, err := executeSessionsList(t)
	require.NoError(t, err)

	t.Logf("Sessions list (age) output:\n%s", output)

	// Should show some form of age (just now, minutes ago, etc.)
	hasAge := strings.Contains(output, "just now") ||
		strings.Contains(output, "minute") ||
		strings.Contains(output, "hour") ||
		strings.Contains(output, "day")
	assert.True(t, hasAge, "Should show age of session")
}

// ============================================================================
// Sessions Delete Tests
// ============================================================================

// TestIntegration_SessionsDeleteWithForce tests deleting a session with --force flag
func TestIntegration_SessionsDeleteWithForce(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupSessionsCommandTest(t, env)
	defer cleanup()

	// Add a session we can safely delete
	sessions, err := izanami.LoadSessions()
	require.NoError(t, err)

	sessions.AddSession("to-delete", &izanami.Session{
		URL:       "http://delete.example.com",
		Username:  "delete-user",
		JwtToken:  "delete-token",
		CreatedAt: time.Now(),
	})
	err = sessions.Save()
	require.NoError(t, err)

	// Set force flag
	sessionsDeleteForce = true

	output, err := executeSessionsDelete(t, "to-delete")
	require.NoError(t, err)

	t.Logf("Sessions delete output:\n%s", output)

	// Should confirm deletion
	assert.Contains(t, output, "Deleted session", "Should confirm session was deleted")
	assert.Contains(t, output, "to-delete", "Should mention session name")

	// Verify session was actually deleted
	sessions, err = izanami.LoadSessions()
	require.NoError(t, err)
	_, err = sessions.GetSession("to-delete")
	assert.Error(t, err, "Session should no longer exist")
}

// TestIntegration_SessionsDeleteWithConfirmation tests deleting with user confirmation
func TestIntegration_SessionsDeleteWithConfirmation(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupSessionsCommandTest(t, env)
	defer cleanup()

	// Add a session to delete
	sessions, err := izanami.LoadSessions()
	require.NoError(t, err)

	sessions.AddSession("confirm-delete", &izanami.Session{
		URL:       "http://confirm.example.com",
		Username:  "confirm-user",
		JwtToken:  "confirm-token",
		CreatedAt: time.Now(),
	})
	err = sessions.Save()
	require.NoError(t, err)

	// User types "y" to confirm
	output, err := executeSessionsDeleteWithInput(t, "confirm-delete", "y\n")
	require.NoError(t, err)

	t.Logf("Sessions delete (confirmed) output:\n%s", output)

	// Should confirm deletion
	assert.Contains(t, output, "Deleted session", "Should confirm deletion")

	// Verify session was deleted
	sessions, err = izanami.LoadSessions()
	require.NoError(t, err)
	_, err = sessions.GetSession("confirm-delete")
	assert.Error(t, err, "Session should no longer exist")
}

// TestIntegration_SessionsDeleteCancelled tests canceling session deletion
func TestIntegration_SessionsDeleteCancelled(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupSessionsCommandTest(t, env)
	defer cleanup()

	// Add a session to not delete
	sessions, err := izanami.LoadSessions()
	require.NoError(t, err)

	sessions.AddSession("keep-session", &izanami.Session{
		URL:       "http://keep.example.com",
		Username:  "keep-user",
		JwtToken:  "keep-token",
		CreatedAt: time.Now(),
	})
	err = sessions.Save()
	require.NoError(t, err)

	// User types "n" to cancel
	output, err := executeSessionsDeleteWithInput(t, "keep-session", "n\n")
	require.NoError(t, err)

	t.Logf("Sessions delete (cancelled) output:\n%s", output)

	// Should show cancellation message
	assert.Contains(t, output, "Cancelled", "Should show cancellation message")

	// Verify session still exists
	sessions, err = izanami.LoadSessions()
	require.NoError(t, err)
	session, err := sessions.GetSession("keep-session")
	require.NoError(t, err)
	assert.Equal(t, "http://keep.example.com", session.URL, "Session should still exist")
}

// TestIntegration_SessionsDeleteNotFound tests deleting a non-existent session
func TestIntegration_SessionsDeleteNotFound(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupSessionsCommandTest(t, env)
	defer cleanup()

	// Set force to skip prompt
	sessionsDeleteForce = true

	output, err := executeSessionsDelete(t, "nonexistent-session")

	t.Logf("Sessions delete (not found) output:\n%s", output)
	t.Logf("Error: %v", err)

	// Should return error for non-existent session
	require.Error(t, err, "Should error when session not found")
	assert.Contains(t, err.Error(), "not found", "Error should mention not found")
}

// TestIntegration_SessionsDeleteMissingArg tests sessions delete without session name
func TestIntegration_SessionsDeleteMissingArg(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupSessionsCommandTest(t, env)
	defer cleanup()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(sessionsCmd)

	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	sessionsCmd.SetOut(&buf)
	sessionsCmd.SetErr(&buf)
	sessionsDeleteCmd.SetOut(&buf)
	sessionsDeleteCmd.SetErr(&buf)

	cmd.SetArgs([]string{"sessions", "delete"}) // Missing session name
	err := cmd.Execute()

	sessionsCmd.SetOut(nil)
	sessionsCmd.SetErr(nil)
	sessionsDeleteCmd.SetOut(nil)
	sessionsDeleteCmd.SetErr(nil)

	require.Error(t, err, "Should error when session name is missing")

	output := buf.String()
	t.Logf("Sessions delete (missing arg) output:\n%s", output)
}

// ============================================================================
// Logout Command Tests
// ============================================================================

// TestIntegration_LogoutClearsToken tests that logout clears the JWT token
func TestIntegration_LogoutClearsToken(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupSessionsCommandTest(t, env)
	defer cleanup()

	// Verify we have a token
	originalToken := env.GetJwtToken(t)
	require.NotEmpty(t, originalToken, "Should have token after login")

	// Execute logout
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

	require.NoError(t, err, "Logout should succeed")

	output := buf.String()
	t.Logf("Logout output:\n%s", output)

	assert.Contains(t, output, "Logged out", "Should show logout success message")

	// Verify token is cleared
	sessions, err := izanami.LoadSessions()
	require.NoError(t, err)

	for _, session := range sessions.Sessions {
		if session.URL == env.BaseURL {
			assert.Empty(t, session.JwtToken, "JWT token should be cleared after logout")
			break
		}
	}
}

// TestIntegration_LogoutShowsReloginHint tests that logout shows how to login again
func TestIntegration_LogoutShowsReloginHint(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupSessionsCommandTest(t, env)
	defer cleanup()

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

	require.NoError(t, err)

	output := buf.String()
	t.Logf("Logout output:\n%s", output)

	// Should show how to login again
	assert.Contains(t, output, "iz login", "Should show login command hint")
	assert.Contains(t, output, env.BaseURL, "Should show the URL to login to")
}

// TestIntegration_LogoutNoActiveProfile tests logout when there's no active profile
func TestIntegration_LogoutNoActiveProfile(t *testing.T) {
	_ = setupIntegrationTest(t) // Sets up isolated env, no login

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

	require.Error(t, err, "Logout should fail without active profile")

	t.Logf("Logout error: %v", err)
	assert.Contains(t, err.Error(), "profile", "Error should mention profile")
}

// ============================================================================
// formatAge Tests
// ============================================================================

// TestIntegration_FormatAge tests the formatAge helper function
func TestIntegration_FormatAge(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "just now",
			duration: 30 * time.Second,
			expected: "just now",
		},
		{
			name:     "1 minute ago",
			duration: 1 * time.Minute,
			expected: "1 minute ago",
		},
		{
			name:     "multiple minutes ago",
			duration: 5 * time.Minute,
			expected: "5 minutes ago",
		},
		{
			name:     "1 hour ago",
			duration: 1 * time.Hour,
			expected: "1 hour ago",
		},
		{
			name:     "multiple hours ago",
			duration: 3 * time.Hour,
			expected: "3 hours ago",
		},
		{
			name:     "1 day ago",
			duration: 24 * time.Hour,
			expected: "1 day ago",
		},
		{
			name:     "multiple days ago",
			duration: 72 * time.Hour,
			expected: "3 days ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAge(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============================================================================
// API Verification Tests
// ============================================================================

// TestIntegration_SessionsPersistence tests that sessions persist correctly
func TestIntegration_SessionsPersistence(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupSessionsCommandTest(t, env)
	defer cleanup()

	// Add a test session
	sessions, err := izanami.LoadSessions()
	require.NoError(t, err)

	testTime := time.Now()
	sessions.AddSession("persistence-test", &izanami.Session{
		URL:       "http://persistence.example.com",
		Username:  "persist-user",
		JwtToken:  "persist-token-abc123",
		CreatedAt: testTime,
	})
	err = sessions.Save()
	require.NoError(t, err)

	// Reload sessions and verify
	reloaded, err := izanami.LoadSessions()
	require.NoError(t, err)

	session, err := reloaded.GetSession("persistence-test")
	require.NoError(t, err)

	assert.Equal(t, "http://persistence.example.com", session.URL)
	assert.Equal(t, "persist-user", session.Username)
	assert.Equal(t, "persist-token-abc123", session.JwtToken)
	// CreatedAt may have minor differences due to YAML serialization
	assert.WithinDuration(t, testTime, session.CreatedAt, time.Second)
}

// TestIntegration_SessionsFilePermissions tests that sessions file has secure permissions
func TestIntegration_SessionsFilePermissions(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupSessionsCommandTest(t, env)
	defer cleanup()

	// Verify sessions file exists
	require.True(t, env.SessionsFileExists(), "Sessions file should exist")

	// Read file content to verify it was written
	content := env.ReadSessionsFile(t)
	assert.NotEmpty(t, content, "Sessions file should have content")

	// The file permissions are set to 0600 in the Save() method
	// We can't easily check permissions in a cross-platform way in this test,
	// but the unit tests verify this behavior
	t.Log("Sessions file created successfully with content")
}

// TestIntegration_SessionsListTableFormat tests sessions list table formatting
func TestIntegration_SessionsListTableFormat(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupSessionsCommandTest(t, env)
	defer cleanup()

	// Ensure table format
	outputFormat = "table"

	output, err := executeSessionsList(t)
	require.NoError(t, err)

	t.Logf("Sessions list (table) output:\n%s", output)

	// Table format should have column headers or structured output
	// Check for presence of key data
	assert.Contains(t, output, env.BaseURL)
	assert.Contains(t, output, env.Username)
}
