package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// TestIntegration_ConfigInit tests config init creates config file
func TestIntegration_ConfigInit(t *testing.T) {
	env := setupIntegrationTest(t)

	// Verify config file doesn't exist initially
	assert.False(t, env.ConfigFileExists(), "Config file should not exist initially")

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "init"})
	err := cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()

	// Verify output
	assert.Contains(t, output, "Configuration file created", "Should confirm file creation")
	assert.Contains(t, output, "Next steps", "Should show next steps")
	assert.Contains(t, output, "SECURITY NOTICE", "Should show security notice")

	// Verify config file was created
	assert.True(t, env.ConfigFileExists(), "Config file should exist after init")

	t.Logf("Config init output:\n%s", output)
}

// TestIntegration_ConfigInitWithDefaults tests config init --defaults flag
func TestIntegration_ConfigInitWithDefaults(t *testing.T) {
	env := setupIntegrationTest(t)

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "init", "--defaults"})
	err := cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.NoError(t, err)

	// Verify config file was created
	assert.True(t, env.ConfigFileExists(), "Config file should exist after init --defaults")

	t.Log("Config init --defaults completed successfully")
}

// TestIntegration_ConfigInitAlreadyExists tests config init when file exists
func TestIntegration_ConfigInitAlreadyExists(t *testing.T) {
	env := setupIntegrationTest(t)

	// Create config first
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "init"})
	err := cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.NoError(t, err)
	assert.True(t, env.ConfigFileExists(), "Config file should exist")

	// Try to init again
	var buf2 bytes.Buffer
	cmd2 := &cobra.Command{Use: "iz"}
	cmd2.AddCommand(configCmd)
	cmd2.SetOut(&buf2)
	cmd2.SetErr(&buf2)
	configCmd.SetOut(&buf2)
	configCmd.SetErr(&buf2)

	cmd2.SetArgs([]string{"config", "init"})
	err = cmd2.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	// Should fail because file already exists
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists", "Should indicate config already exists")

	t.Log("Config init correctly fails when file already exists")
}

// TestIntegration_ConfigPath tests config path shows correct paths
func TestIntegration_ConfigPath(t *testing.T) {
	env := setupIntegrationTest(t)

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "path"})
	err := cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()

	// Verify output contains path info
	assert.Contains(t, output, "Config file:", "Should show config file path")
	assert.Contains(t, output, "Config directory:", "Should show config directory")
	assert.Contains(t, output, env.ConfigDir, "Should contain the config directory path")
	assert.Contains(t, output, "not created", "Should indicate file doesn't exist yet")

	t.Logf("Config path output:\n%s", output)
}

// TestIntegration_ConfigPathAfterInit tests config path after init shows exists
func TestIntegration_ConfigPathAfterInit(t *testing.T) {
	env := setupIntegrationTest(t)

	// Init config first
	err := izanami.InitConfigFile()
	require.NoError(t, err)
	assert.True(t, env.ConfigFileExists())

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "path"})
	err = cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()

	assert.Contains(t, output, "Status: exists", "Should indicate file exists")

	t.Log("Config path shows file exists after init")
}

// TestIntegration_ConfigSetGlobalKey tests setting a global config key
func TestIntegration_ConfigSetGlobalKey(t *testing.T) {
	env := setupIntegrationTest(t)

	// Init config first
	err := izanami.InitConfigFile()
	require.NoError(t, err)
	assert.True(t, env.ConfigFileExists())

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "set", "timeout", "60"})
	err = cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()

	assert.Contains(t, output, "Set timeout = 60", "Should confirm setting")

	// Verify the value was persisted
	configValue, err := izanami.GetConfigValue("timeout")
	require.NoError(t, err)
	assert.Equal(t, "60", configValue.Value, "Timeout should be set to 60")

	t.Logf("Config set output:\n%s", output)
}

// TestIntegration_ConfigSetOutputFormat tests setting output-format config
func TestIntegration_ConfigSetOutputFormat(t *testing.T) {
	env := setupIntegrationTest(t)

	// Init config first
	err := izanami.InitConfigFile()
	require.NoError(t, err)
	assert.True(t, env.ConfigFileExists())

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "set", "output-format", "json"})
	err = cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()

	assert.Contains(t, output, "Set output-format = json", "Should confirm setting")

	// Verify the value was persisted
	configValue, err := izanami.GetConfigValue("output-format")
	require.NoError(t, err)
	assert.Equal(t, "json", configValue.Value)

	t.Log("Config set output-format completed successfully")
}

// TestIntegration_ConfigSetVerbose tests setting verbose config
func TestIntegration_ConfigSetVerbose(t *testing.T) {
	env := setupIntegrationTest(t)

	err := izanami.InitConfigFile()
	require.NoError(t, err)
	assert.True(t, env.ConfigFileExists())

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "set", "verbose", "true"})
	err = cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()

	assert.Contains(t, output, "Set verbose = true", "Should confirm setting")

	t.Log("Config set verbose completed successfully")
}

// TestIntegration_ConfigSetColor tests setting color config
func TestIntegration_ConfigSetColor(t *testing.T) {
	env := setupIntegrationTest(t)

	err := izanami.InitConfigFile()
	require.NoError(t, err)
	assert.True(t, env.ConfigFileExists())

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "set", "color", "never"})
	err = cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()

	assert.Contains(t, output, "Set color = never", "Should confirm setting")

	t.Log("Config set color completed successfully")
}

// TestIntegration_ConfigSetInvalidKey tests setting an invalid config key
func TestIntegration_ConfigSetInvalidKey(t *testing.T) {
	env := setupIntegrationTest(t)

	err := izanami.InitConfigFile()
	require.NoError(t, err)
	assert.True(t, env.ConfigFileExists())

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "set", "invalid-key", "value"})
	err = cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.Error(t, err)
	output := buf.String()

	// Should show valid keys list
	assert.Contains(t, output, "Global configuration keys", "Should show valid keys")

	t.Log("Config set correctly fails for invalid key")
}

// TestIntegration_ConfigSetProfileKey tests config set rejects profile-specific keys
func TestIntegration_ConfigSetProfileKey(t *testing.T) {
	env := setupIntegrationTest(t)

	err := izanami.InitConfigFile()
	require.NoError(t, err)
	assert.True(t, env.ConfigFileExists())

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	// Try to set a profile-specific key
	cmd.SetArgs([]string{"config", "set", "base-url", "http://localhost:9000"})
	err = cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.Error(t, err)
	// Should suggest using 'profiles set' instead
	assert.Contains(t, err.Error(), "profile-specific", "Should indicate it's a profile setting")
	assert.Contains(t, err.Error(), "profiles set", "Should suggest profiles set command")

	t.Log("Config set correctly rejects profile-specific keys with helpful message")
}

// TestIntegration_ConfigSetNoArgs tests config set with no arguments
func TestIntegration_ConfigSetNoArgs(t *testing.T) {
	_ = setupIntegrationTest(t)

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "set"})
	err := cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.Error(t, err)
	output := buf.String()

	// Should show valid keys list when no args provided
	assert.Contains(t, output, "Global configuration keys", "Should show global keys")
	assert.Contains(t, output, "Profile-specific keys", "Should show profile keys")

	t.Log("Config set correctly shows help when no args provided")
}

// TestIntegration_ConfigSetMissingValue tests config set with missing value
func TestIntegration_ConfigSetMissingValue(t *testing.T) {
	_ = setupIntegrationTest(t)

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "set", "timeout"})
	err := cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing value", "Should indicate missing value")

	t.Log("Config set correctly fails when value is missing")
}

// TestIntegration_ConfigSetTooManyArgs tests config set with too many arguments
func TestIntegration_ConfigSetTooManyArgs(t *testing.T) {
	_ = setupIntegrationTest(t)

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "set", "timeout", "60", "extra"})
	err := cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many arguments", "Should indicate too many arguments")

	t.Log("Config set correctly fails with too many arguments")
}

// TestIntegration_ConfigGet tests getting a config value
func TestIntegration_ConfigGet(t *testing.T) {
	env := setupIntegrationTest(t)

	// Init config and set a value
	err := izanami.InitConfigFile()
	require.NoError(t, err)
	assert.True(t, env.ConfigFileExists())

	err = izanami.SetConfigValue("timeout", "45")
	require.NoError(t, err)

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "get", "timeout"})
	err = cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()

	assert.Contains(t, output, "timeout: 45", "Should show timeout value")
	assert.Contains(t, output, "source: file", "Should show source is file")

	t.Logf("Config get output:\n%s", output)
}

// TestIntegration_ConfigGetNotSet tests getting a config value that isn't set
func TestIntegration_ConfigGetNotSet(t *testing.T) {
	env := setupIntegrationTest(t)

	// Init config but don't set the value
	err := izanami.InitConfigFile()
	require.NoError(t, err)
	assert.True(t, env.ConfigFileExists())

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "get", "project"})
	err = cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()

	assert.Contains(t, output, "(not set)", "Should indicate value is not set")

	t.Logf("Config get not set output:\n%s", output)
}

// TestIntegration_ConfigGetDefault tests getting a config value with default
func TestIntegration_ConfigGetDefault(t *testing.T) {
	env := setupIntegrationTest(t)

	err := izanami.InitConfigFile()
	require.NoError(t, err)
	assert.True(t, env.ConfigFileExists())

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	// timeout has a default value
	cmd.SetArgs([]string{"config", "get", "timeout"})
	err = cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()

	// Should show timeout with default source
	assert.Contains(t, output, "timeout:", "Should show timeout key")

	t.Logf("Config get default output:\n%s", output)
}

// TestIntegration_ConfigUnset tests unsetting a config value
func TestIntegration_ConfigUnset(t *testing.T) {
	env := setupIntegrationTest(t)

	// Init config and set a value
	err := izanami.InitConfigFile()
	require.NoError(t, err)
	assert.True(t, env.ConfigFileExists())

	err = izanami.SetConfigValue("timeout", "99")
	require.NoError(t, err)

	// Verify it's set
	configValue, err := izanami.GetConfigValue("timeout")
	require.NoError(t, err)
	assert.Equal(t, "99", configValue.Value)

	// Unset it
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "unset", "timeout"})
	err = cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()

	assert.Contains(t, output, "Removed timeout from config file", "Should confirm removal")

	t.Logf("Config unset output:\n%s", output)
}

// TestIntegration_ConfigList tests listing all config values
func TestIntegration_ConfigList(t *testing.T) {
	env := setupIntegrationTest(t)

	// Init config and set some values
	err := izanami.InitConfigFile()
	require.NoError(t, err)
	assert.True(t, env.ConfigFileExists())

	err = izanami.SetConfigValue("timeout", "30")
	require.NoError(t, err)
	err = izanami.SetConfigValue("output-format", "table")
	require.NoError(t, err)

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "list"})
	err = cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()

	// Should show table with keys
	assert.Contains(t, output, "KEY", "Should have KEY header")
	assert.Contains(t, output, "VALUE", "Should have VALUE header")
	assert.Contains(t, output, "SOURCE", "Should have SOURCE header")
	assert.Contains(t, output, "timeout", "Should show timeout key")
	assert.Contains(t, output, "output-format", "Should show output-format key")
	assert.Contains(t, output, "Client keys are profile-specific", "Should show note about client keys")

	t.Logf("Config list output:\n%s", output)
}

// TestIntegration_ConfigListShowSecrets tests config list with --show-secrets
func TestIntegration_ConfigListShowSecrets(t *testing.T) {
	env := setupIntegrationTest(t)

	err := izanami.InitConfigFile()
	require.NoError(t, err)
	assert.True(t, env.ConfigFileExists())

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "list", "--show-secrets"})
	err = cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.NoError(t, err)

	// Command should succeed - the actual secret display behavior depends on whether secrets are set
	t.Log("Config list --show-secrets completed successfully")
}

// TestIntegration_ConfigValidate tests config validation for global settings
func TestIntegration_ConfigValidate(t *testing.T) {
	env := setupIntegrationTest(t)

	// Init config - fresh config with default values should be valid
	err := izanami.InitConfigFile()
	require.NoError(t, err)
	assert.True(t, env.ConfigFileExists())

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "validate"})
	err = cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()

	assert.Contains(t, output, "Configuration is valid", "Should indicate config is valid")

	t.Logf("Config validate output:\n%s", output)
}

// TestIntegration_ConfigValidateWithErrors tests config validation returns errors for invalid global settings
func TestIntegration_ConfigValidateWithErrors(t *testing.T) {
	env := setupIntegrationTest(t)

	// Init config
	err := izanami.InitConfigFile()
	require.NoError(t, err)
	assert.True(t, env.ConfigFileExists())

	// Set an invalid value for output-format (only 'table' or 'json' are valid)
	err = izanami.SetConfigValue("output-format", "invalid-format")
	require.NoError(t, err)

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "validate"})
	err = cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	// Should fail because output-format is invalid
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error(s)", "Should indicate configuration has errors")

	output := buf.String()
	assert.Contains(t, output, "output-format", "Should mention the invalid field")

	t.Logf("Config validate with errors output:\n%s", output)
}

// TestIntegration_ConfigValidateNoFile tests config validate when no file exists
func TestIntegration_ConfigValidateNoFile(t *testing.T) {
	env := setupIntegrationTest(t)

	// Don't init config - file doesn't exist
	assert.False(t, env.ConfigFileExists(), "Config file should not exist")

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "validate"})
	err := cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no configuration file found", "Should return error about missing config")

	output := buf.String()
	assert.Contains(t, output, "No configuration file found", "Should indicate no config file")

	t.Logf("Config validate no file output:\n%s", output)
}

// TestIntegration_ConfigResetWithConfirmation tests config reset with confirmation
func TestIntegration_ConfigResetWithConfirmation(t *testing.T) {
	env := setupIntegrationTest(t)

	// Init config first
	err := izanami.InitConfigFile()
	require.NoError(t, err)
	assert.True(t, env.ConfigFileExists())

	var buf bytes.Buffer
	input := bytes.NewBufferString("y\n") // Confirm yes

	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(input)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	// Set input on the reset subcommand
	for _, subCmd := range configCmd.Commands() {
		if subCmd.Use == "reset" {
			subCmd.SetIn(input)
		}
	}

	cmd.SetArgs([]string{"config", "reset"})
	err = cmd.Execute()

	// Reset command state
	for _, subCmd := range configCmd.Commands() {
		if subCmd.Use == "reset" {
			subCmd.SetIn(nil)
		}
	}
	configCmd.SetIn(nil)
	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()

	assert.Contains(t, output, "Configuration file deleted", "Should confirm deletion")

	// Verify config file was deleted
	assert.False(t, env.ConfigFileExists(), "Config file should be deleted after reset")

	t.Logf("Config reset output:\n%s", output)
}

// TestIntegration_ConfigResetNoFile tests config reset when no file exists
func TestIntegration_ConfigResetNoFile(t *testing.T) {
	env := setupIntegrationTest(t)

	// Don't init config - file doesn't exist
	assert.False(t, env.ConfigFileExists(), "Config file should not exist")

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "reset"})
	err := cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "config file does not exist", "Should indicate no config file")

	t.Log("Config reset correctly fails when no config file exists")
}

// TestIntegration_ConfigResetCancelled tests config reset when user cancels
func TestIntegration_ConfigResetCancelled(t *testing.T) {
	env := setupIntegrationTest(t)

	// Init config first
	err := izanami.InitConfigFile()
	require.NoError(t, err)
	assert.True(t, env.ConfigFileExists())

	var buf bytes.Buffer
	input := bytes.NewBufferString("n\n") // Cancel

	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(input)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	// Set input on the reset subcommand
	for _, subCmd := range configCmd.Commands() {
		if subCmd.Use == "reset" {
			subCmd.SetIn(input)
		}
	}

	cmd.SetArgs([]string{"config", "reset"})
	err = cmd.Execute()

	// Reset command state
	for _, subCmd := range configCmd.Commands() {
		if subCmd.Use == "reset" {
			subCmd.SetIn(nil)
		}
	}
	configCmd.SetIn(nil)
	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()

	assert.Contains(t, output, "Cancelled", "Should show cancelled message")

	// Verify config file still exists
	assert.True(t, env.ConfigFileExists(), "Config file should still exist after cancel")

	t.Log("Config reset correctly cancelled")
}

// TestIntegration_ConfigSetThenGet tests setting and then getting a value
func TestIntegration_ConfigSetThenGet(t *testing.T) {
	env := setupIntegrationTest(t)

	// Init config
	err := izanami.InitConfigFile()
	require.NoError(t, err)
	assert.True(t, env.ConfigFileExists())

	// Set a value
	var setBuf bytes.Buffer
	setCmd := &cobra.Command{Use: "iz"}
	setCmd.AddCommand(configCmd)
	setCmd.SetOut(&setBuf)
	setCmd.SetErr(&setBuf)
	configCmd.SetOut(&setBuf)
	configCmd.SetErr(&setBuf)

	setCmd.SetArgs([]string{"config", "set", "timeout", "120"})
	err = setCmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.NoError(t, err)

	// Get the value
	var getBuf bytes.Buffer
	getCmd := &cobra.Command{Use: "iz"}
	getCmd.AddCommand(configCmd)
	getCmd.SetOut(&getBuf)
	getCmd.SetErr(&getBuf)
	configCmd.SetOut(&getBuf)
	configCmd.SetErr(&getBuf)

	getCmd.SetArgs([]string{"config", "get", "timeout"})
	err = getCmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.NoError(t, err)
	output := getBuf.String()

	assert.Contains(t, output, "timeout: 120", "Should show the value we set")
	assert.Contains(t, output, "source: file", "Should show source is file")

	t.Log("Config set then get completed successfully")
}

// TestIntegration_ConfigAfterLogin tests config commands work with login
func TestIntegration_ConfigAfterLogin(t *testing.T) {
	env := setupIntegrationTest(t)

	// Login first (creates config and session)
	env.Login(t)

	// Should be able to set config values
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "set", "verbose", "true"})
	err := cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()

	assert.Contains(t, output, "Set verbose = true", "Should set verbose after login")

	t.Log("Config commands work after login")
}

// TestIntegration_ConfigListAfterLogin tests config list shows profile settings
func TestIntegration_ConfigListAfterLogin(t *testing.T) {
	env := setupIntegrationTest(t)

	// Login first
	env.Login(t)

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "list"})
	err := cmd.Execute()

	configCmd.SetOut(nil)
	configCmd.SetErr(nil)

	require.NoError(t, err)
	output := buf.String()

	// Should show config values including those from profile
	assert.Contains(t, output, "base-url", "Should show base-url key")

	t.Logf("Config list after login output:\n%s", output)
}
