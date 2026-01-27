package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"gopkg.in/yaml.v3"
)

// Test helper: Setup command with proper I/O streams for config commands
func setupConfigCommand(buf *bytes.Buffer, input *bytes.Buffer, args []string) (*cobra.Command, func()) {
	cmd := &cobra.Command{Use: "test"}
	cmd.AddCommand(configCmd)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if input != nil {
		cmd.SetIn(input)
		configResetCmd.SetIn(input)
	}
	configCmd.SetOut(buf)
	configCmd.SetErr(buf)
	cmd.SetArgs(args)

	// Return cleanup function to reset command streams
	cleanup := func() {
		configCmd.SetIn(nil)
		configCmd.SetOut(nil)
		configCmd.SetErr(nil)
		configResetCmd.SetIn(nil)
	}

	return cmd, cleanup
}

// Test helper: Create a test config file with profiles for config tests
func createConfigTestFile(t *testing.T, configPath string, profiles map[string]*izanami.Profile, activeProfile string) {
	t.Helper()

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	err := os.MkdirAll(dir, 0700)
	require.NoError(t, err, "Failed to create config directory")

	// Build config structure
	config := map[string]interface{}{
		"timeout":       30,
		"verbose":       false,
		"output-format": "table",
		"color":         "auto",
	}

	if activeProfile != "" {
		config["active_profile"] = activeProfile
	}

	if len(profiles) > 0 {
		profilesMap := make(map[string]interface{})
		for name, profile := range profiles {
			profileMap := make(map[string]interface{})
			if profile.Session != "" {
				profileMap["session"] = profile.Session
			}
			if profile.BaseURL != "" {
				profileMap["base-url"] = profile.BaseURL
			}
			if profile.ClientBaseURL != "" {
				profileMap["client-base-url"] = profile.ClientBaseURL
			}
			if profile.Tenant != "" {
				profileMap["tenant"] = profile.Tenant
			}
			if profile.Project != "" {
				profileMap["project"] = profile.Project
			}
			if profile.Context != "" {
				profileMap["context"] = profile.Context
			}
			if profile.ClientID != "" {
				profileMap["client-id"] = profile.ClientID
			}
			if profile.ClientSecret != "" {
				profileMap["client-secret"] = profile.ClientSecret
			}
			if profile.PersonalAccessToken != "" {
				profileMap["personal-access-token"] = profile.PersonalAccessToken
			}
			if profile.PersonalAccessTokenUsername != "" {
				profileMap["personal-access-token-username"] = profile.PersonalAccessTokenUsername
			}
			if profile.ClientKeys != nil && len(profile.ClientKeys) > 0 {
				profileMap["client-keys"] = profile.ClientKeys
			}
			profilesMap[name] = profileMap
		}
		config["profiles"] = profilesMap
	}

	// Write YAML
	data, err := yaml.Marshal(config)
	require.NoError(t, err, "Failed to marshal config")

	err = os.WriteFile(configPath, data, 0600)
	require.NoError(t, err, "Failed to write config file")
}

// TestConfigListCmd_NoActiveProfile tests config list when no active profile is set
func TestConfigListCmd_NoActiveProfile(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Create config without active profile
	createConfigTestFile(t, paths.configPath, nil, "")

	var buf bytes.Buffer
	cmd, cleanup := setupConfigCommand(&buf, nil, []string{"config", "list"})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Should show Global Settings section
	assert.Contains(t, output, "=== Global Settings ===", "Should have Global Settings header")
	assert.Contains(t, output, "KEY", "Should have KEY header")
	assert.Contains(t, output, "VALUE", "Should have VALUE header")
	assert.Contains(t, output, "SOURCE", "Should have SOURCE header")
	// Should show global keys
	assert.Contains(t, output, "timeout", "Should show timeout key")
	assert.Contains(t, output, "output-format", "Should show output-format key")
	assert.Contains(t, output, "color", "Should show color key")
	assert.Contains(t, output, "verbose", "Should show verbose key")
	// Should show no active profile message
	assert.Contains(t, output, "No active profile set", "Should show no active profile message")

	t.Logf("Config list (no profile) output:\n%s", output)
}

// TestConfigListCmd_WithActiveProfile tests config list when an active profile is set
func TestConfigListCmd_WithActiveProfile(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Create config with active profile
	profiles := map[string]*izanami.Profile{
		"sandbox": {
			BaseURL:       "http://localhost:9000",
			ClientBaseURL: "http://worker.localhost:9000",
			Tenant:        "dev-tenant",
			Project:       "test-project",
			Context:       "dev",
		},
	}
	createConfigTestFile(t, paths.configPath, profiles, "sandbox")

	var buf bytes.Buffer
	cmd, cleanup := setupConfigCommand(&buf, nil, []string{"config", "list"})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Should show Global Settings section
	assert.Contains(t, output, "=== Global Settings ===", "Should have Global Settings header")
	// Should show Active Profile section
	assert.Contains(t, output, "=== Active Profile: sandbox ===", "Should have Active Profile header")
	// Should show profile-specific keys
	assert.Contains(t, output, "base-url", "Should show base-url in profile section")
	assert.Contains(t, output, "client-base-url", "Should show client-base-url in profile section")
	assert.Contains(t, output, "http://localhost:9000", "Should show base-url value")
	assert.Contains(t, output, "http://worker.localhost:9000", "Should show client-base-url value")
	assert.Contains(t, output, "tenant", "Should show tenant key")
	assert.Contains(t, output, "dev-tenant", "Should show tenant value")
	assert.Contains(t, output, "project", "Should show project key")
	assert.Contains(t, output, "test-project", "Should show project value")
	// Should show helpful footer
	assert.Contains(t, output, "iz config set", "Should show config set hint")
	assert.Contains(t, output, "iz profiles set", "Should show profiles set hint")
	assert.Contains(t, output, "iz profiles client-keys list|add", "Should show client-keys hint")
	assert.Contains(t, output, "iz profiles show sandbox", "Should show profiles show hint")

	t.Logf("Config list (with profile) output:\n%s", output)
}

// TestConfigListCmd_WithClientKeys tests config list shows client-keys count
func TestConfigListCmd_WithClientKeys(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Create config with client-keys
	profiles := map[string]*izanami.Profile{
		"prod": {
			BaseURL: "https://izanami.prod.com",
			Tenant:  "production",
			ClientKeys: map[string]izanami.TenantClientKeysConfig{
				"tenant1": {
					ClientID:     "client1",
					ClientSecret: "secret1",
				},
				"tenant2": {
					ClientID:     "client2",
					ClientSecret: "secret2",
				},
			},
		},
	}
	createConfigTestFile(t, paths.configPath, profiles, "prod")

	var buf bytes.Buffer
	cmd, cleanup := setupConfigCommand(&buf, nil, []string{"config", "list"})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Should show client-keys count
	assert.Contains(t, output, "client-keys", "Should show client-keys key")
	assert.Contains(t, output, "2 tenant(s) configured", "Should show client-keys count")

	t.Logf("Config list (with client-keys) output:\n%s", output)
}

// TestConfigListCmd_SensitiveValuesRedacted tests that sensitive values are redacted
func TestConfigListCmd_SensitiveValuesRedacted(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Create config with sensitive values
	profiles := map[string]*izanami.Profile{
		"test": {
			BaseURL:             "http://localhost:9000",
			PersonalAccessToken: "my-personal-token",
		},
	}
	createConfigTestFile(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupConfigCommand(&buf, nil, []string{"config", "list"})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Sensitive values should be redacted
	assert.Contains(t, output, "<redacted>", "Should show redacted for sensitive values")
	assert.NotContains(t, output, "my-personal-token", "Should NOT show personal-access-token value")

	t.Logf("Config list (secrets redacted) output:\n%s", output)
}

// TestConfigListCmd_ShowSecrets tests --show-secrets flag
func TestConfigListCmd_ShowSecrets(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Create config with sensitive values
	profiles := map[string]*izanami.Profile{
		"test": {
			BaseURL:             "http://localhost:9000",
			PersonalAccessToken: "my-personal-token",
		},
	}
	createConfigTestFile(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupConfigCommand(&buf, nil, []string{"config", "list", "--show-secrets"})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Sensitive values should be shown
	assert.Contains(t, output, "my-personal-token", "Should show personal-access-token value with --show-secrets")
	assert.NotContains(t, output, "<redacted>", "Should NOT show redacted with --show-secrets")

	t.Logf("Config list (show secrets) output:\n%s", output)
}

// TestConfigListCmd_BaseURLFromSession tests that base-url is resolved from session
func TestConfigListCmd_BaseURLFromSession(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Create session with URL
	sessions := map[string]*izanami.Session{
		"my-session": {
			URL:      "http://session-url.example.com",
			Username: "testuser",
		},
	}
	createTestSessions(t, paths.sessionsPath, sessions)

	// Create profile that references session but has no base-url
	profiles := map[string]*izanami.Profile{
		"test": {
			Session: "my-session",
			Tenant:  "test-tenant",
		},
	}
	createConfigTestFile(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupConfigCommand(&buf, nil, []string{"config", "list"})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Should show URL from session and source as "session"
	assert.Contains(t, output, "http://session-url.example.com", "Should show base-url from session")
	assert.Contains(t, output, "session", "Should show 'session' as source for base-url")
	assert.Contains(t, output, "my-session", "Should show session name")

	t.Logf("Config list (base-url from session) output:\n%s", output)
}

// TestConfigListCmd_NoClientIdClientSecret tests that client-id and client-secret are not shown
func TestConfigListCmd_NoClientIdClientSecret(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Create config with client-id and client-secret (these are legacy fields)
	profiles := map[string]*izanami.Profile{
		"test": {
			BaseURL:      "http://localhost:9000",
			ClientID:     "legacy-client-id",
			ClientSecret: "legacy-client-secret",
		},
	}
	createConfigTestFile(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupConfigCommand(&buf, nil, []string{"config", "list"})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// client-id and client-secret should NOT be shown in config list
	// (they are legacy fields, client-keys is preferred)
	assert.NotContains(t, output, "client-id\t", "Should NOT show client-id row")
	assert.NotContains(t, output, "legacy-client-id", "Should NOT show client-id value")
	assert.NotContains(t, output, "client-secret\t", "Should NOT show client-secret row")

	t.Logf("Config list (no client-id/client-secret) output:\n%s", output)
}

// TestFormatClientKeysCount tests the formatClientKeysCount helper function
func TestFormatClientKeysCount(t *testing.T) {
	tests := []struct {
		name     string
		keys     map[string]izanami.TenantClientKeysConfig
		expected string
	}{
		{
			name:     "nil map",
			keys:     nil,
			expected: "",
		},
		{
			name:     "empty map",
			keys:     map[string]izanami.TenantClientKeysConfig{},
			expected: "",
		},
		{
			name: "one tenant",
			keys: map[string]izanami.TenantClientKeysConfig{
				"tenant1": {ClientID: "id1"},
			},
			expected: "1 tenant(s) configured",
		},
		{
			name: "multiple tenants",
			keys: map[string]izanami.TenantClientKeysConfig{
				"tenant1": {ClientID: "id1"},
				"tenant2": {ClientID: "id2"},
				"tenant3": {ClientID: "id3"},
			},
			expected: "3 tenant(s) configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatClientKeysCount(tt.keys)
			assert.Equal(t, tt.expected, result)
		})
	}
}
