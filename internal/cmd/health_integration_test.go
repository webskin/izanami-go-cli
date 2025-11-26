package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// setupHealthTest sets up the global cfg for health command tests
func setupHealthTest(t *testing.T, env *IntegrationTestEnv) func() {
	t.Helper()

	// Save original values
	origCfg := cfg
	origOutputFormat := outputFormat

	// Set up config for health check
	cfg = &izanami.Config{
		BaseURL: env.BaseURL,
		Timeout: 30,
	}
	outputFormat = "table"

	return func() {
		cfg = origCfg
		outputFormat = origOutputFormat
	}
}

// TestIntegration_HealthCheckNoAuth tests health check works without authentication
func TestIntegration_HealthCheckNoAuth(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupHealthTest(t, env)
	defer cleanup()

	// Health check should work without login
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

	// Verify health output
	assert.Contains(t, output, "UP", "Should show server is UP")
	assert.Contains(t, output, "Database", "Should show database status")
	assert.Contains(t, output, env.BaseURL, "Should show URL")

	t.Logf("Health check output:\n%s", output)
}

// TestIntegration_HealthCheckAfterLogin tests health check works after login
func TestIntegration_HealthCheckAfterLogin(t *testing.T) {
	env := setupIntegrationTest(t)

	// Login first
	env.Login(t)

	cleanup := setupHealthTest(t, env)
	defer cleanup()

	// Health check should still work
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

	assert.Contains(t, output, "UP", "Should show server is UP after login")

	t.Log("Health check works after login")
}

// TestIntegration_HealthCheckNoURL tests health check fails when no URL configured
func TestIntegration_HealthCheckNoURL(t *testing.T) {
	_ = setupIntegrationTest(t) // Sets up isolated environment

	// Save and clear cfg
	origCfg := cfg
	cfg = nil
	defer func() { cfg = origCfg }()

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

	// Should fail because no URL is configured
	require.Error(t, err)
	assert.Contains(t, err.Error(), "base URL is required", "Should indicate URL is required")

	t.Log("Health check correctly failed when no URL configured")
}

// TestIntegration_HealthCheckDatabaseStatus tests health check returns database status
func TestIntegration_HealthCheckDatabaseStatus(t *testing.T) {
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

	// Verify database status is shown as true
	assert.Contains(t, output, "Database: true", "Should show database is healthy")

	t.Logf("Health check shows database status:\n%s", output)
}
