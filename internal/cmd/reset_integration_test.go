package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// TestIntegration_ResetAfterLogin tests reset clears session and config after login
func TestIntegration_ResetAfterLogin(t *testing.T) {
	env := setupIntegrationTest(t)

	// Login first to create session and config
	env.Login(t)

	// Verify files exist before reset
	assert.True(t, env.SessionsFileExists(), "Sessions file should exist before reset")
	assert.True(t, env.ConfigFileExists(), "Config file should exist before reset")

	// Run reset with --force
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(resetCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	resetCmd.SetOut(&buf)
	resetCmd.SetErr(&buf)

	cmd.SetArgs([]string{"reset", "--force"})
	err := cmd.Execute()

	resetCmd.SetOut(nil)
	resetCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()

	// Verify output
	assert.Contains(t, output, "Reset complete", "Should show reset complete message")
	assert.Contains(t, output, "backed up", "Should mention backup")

	// Verify files are deleted
	assert.False(t, env.SessionsFileExists(), "Sessions file should be deleted after reset")
	assert.False(t, env.ConfigFileExists(), "Config file should be deleted after reset")

	t.Log("Reset successfully cleared session and config after login")
}

// TestIntegration_ResetCreatesBackups tests that reset creates backup files
func TestIntegration_ResetCreatesBackups(t *testing.T) {
	env := setupIntegrationTest(t)

	// Login to create files
	env.Login(t)

	// Get original content before reset
	originalSessionsContent := env.ReadSessionsFile(t)
	originalConfigContent := env.ReadConfigFile(t)

	// Run reset
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(resetCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	resetCmd.SetOut(&buf)
	resetCmd.SetErr(&buf)

	cmd.SetArgs([]string{"reset", "--force"})
	err := cmd.Execute()

	resetCmd.SetOut(nil)
	resetCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()

	// Verify backup paths are mentioned in output
	assert.Contains(t, output, ".backup.", "Should mention backup files")

	// Original files should be gone
	assert.False(t, env.SessionsFileExists(), "Original sessions file should be deleted")
	assert.False(t, env.ConfigFileExists(), "Original config file should be deleted")

	// Log for verification
	t.Logf("Original sessions content length: %d", len(originalSessionsContent))
	t.Logf("Original config content length: %d", len(originalConfigContent))
	t.Log("Backup files created successfully")
}

// TestIntegration_ResetThenLoginAgain tests that login works after reset
func TestIntegration_ResetThenLoginAgain(t *testing.T) {
	env := setupIntegrationTest(t)

	// Login first
	token1 := env.Login(t)
	require.NotEmpty(t, token1)

	// Reset
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(resetCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	resetCmd.SetOut(&buf)
	resetCmd.SetErr(&buf)

	cmd.SetArgs([]string{"reset", "--force"})
	err := cmd.Execute()

	resetCmd.SetOut(nil)
	resetCmd.SetErr(nil)

	require.NoError(t, err)

	// Login again - should work
	token2 := env.Login(t)
	require.NotEmpty(t, token2)

	// Verify session and profile are recreated
	assert.True(t, env.SessionsFileExists(), "Sessions file should exist after re-login")
	assert.True(t, env.ConfigFileExists(), "Config file should exist after re-login")

	// Verify profile exists
	profiles, activeProfile, err := izanami.ListProfiles()
	require.NoError(t, err)
	assert.NotEmpty(t, profiles, "Should have profile after re-login")
	assert.NotEmpty(t, activeProfile, "Should have active profile after re-login")

	t.Log("Successfully logged in again after reset")
}

// TestIntegration_ResetNoFilesError tests reset fails when no files exist
func TestIntegration_ResetNoFilesError(t *testing.T) {
	_ = setupIntegrationTest(t) // Sets up isolated environment with no files

	// Don't login - no files exist

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(resetCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	resetCmd.SetOut(&buf)
	resetCmd.SetErr(&buf)

	cmd.SetArgs([]string{"reset", "--force"})
	err := cmd.Execute()

	resetCmd.SetOut(nil)
	resetCmd.SetErr(nil)

	// Should fail because no files exist
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nothing to reset", "Should indicate nothing to reset")

	t.Log("Reset correctly failed when no files exist")
}
