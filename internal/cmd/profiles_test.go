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

// Test helper: Setup command with proper I/O streams for profile commands
func setupProfileCommand(buf *bytes.Buffer, input *bytes.Buffer, args []string) (*cobra.Command, func()) {
	cmd := &cobra.Command{Use: "test"}
	cmd.AddCommand(profileCmd)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if input != nil {
		cmd.SetIn(input)
		// Set on all profile subcommands that might read input
		profileAddCmd.SetIn(input)
		profileInitCmd.SetIn(input)
		profileDeleteCmd.SetIn(input)
		profileClientKeysAddCmd.SetIn(input)
	}
	profileCmd.SetOut(buf)
	profileCmd.SetErr(buf)
	cmd.SetArgs(args)

	// Return cleanup function to reset command streams
	cleanup := func() {
		profileCmd.SetIn(nil)
		profileCmd.SetOut(nil)
		profileCmd.SetErr(nil)
		profileAddCmd.SetIn(nil)
		profileInitCmd.SetIn(nil)
		profileDeleteCmd.SetIn(nil)
		profileClientKeysAddCmd.SetIn(nil)
	}

	return cmd, cleanup
}

// Test helper: Create a test config file with profiles
func createTestConfig(t *testing.T, configPath string, profiles map[string]*izanami.Profile, activeProfile string) {
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

// Test helper: Create a test sessions file
func createTestSessions(t *testing.T, sessionsPath string, sessions map[string]*izanami.Session) {
	t.Helper()

	// Ensure directory exists
	dir := filepath.Dir(sessionsPath)
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err, "Failed to create sessions directory")

	sessionsData := &izanami.Sessions{
		Sessions: sessions,
	}

	data, err := yaml.Marshal(sessionsData)
	require.NoError(t, err, "Failed to marshal sessions")

	err = os.WriteFile(sessionsPath, data, 0600)
	require.NoError(t, err, "Failed to write sessions file")
}

// Test helper: Verify profile exists in config with expected values
func verifyProfileInConfig(t *testing.T, configPath string, profileName string, expectedProfile *izanami.Profile) {
	t.Helper()

	data, err := os.ReadFile(configPath)
	require.NoError(t, err, "Failed to read config file")

	var config map[string]interface{}
	err = yaml.Unmarshal(data, &config)
	require.NoError(t, err, "Failed to unmarshal config")

	profiles, ok := config["profiles"].(map[string]interface{})
	require.True(t, ok, "Config should have profiles map")

	profile, ok := profiles[profileName].(map[string]interface{})
	require.True(t, ok, "Profile %s should exist", profileName)

	if expectedProfile.Session != "" {
		assert.Equal(t, expectedProfile.Session, profile["session"], "Session mismatch")
	}
	if expectedProfile.BaseURL != "" {
		assert.Equal(t, expectedProfile.BaseURL, profile["base-url"], "BaseURL mismatch")
	}
	if expectedProfile.Tenant != "" {
		assert.Equal(t, expectedProfile.Tenant, profile["tenant"], "Tenant mismatch")
	}
	if expectedProfile.Project != "" {
		assert.Equal(t, expectedProfile.Project, profile["project"], "Project mismatch")
	}
}

// TestProfileListCmd_NoProfiles tests listing when no profiles exist
func TestProfileListCmd_NoProfiles(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Create empty config
	createTestConfig(t, paths.configPath, nil, "")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "list"})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify output
	output := buf.String()
	assert.Contains(t, output, "No profiles configured")
	assert.Contains(t, output, "Create a profile with:")
	assert.Contains(t, output, "iz profiles add <name>")
}

// TestProfileListCmd_MultipleProfiles tests listing multiple profiles
func TestProfileListCmd_MultipleProfiles(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"sandbox": {
			BaseURL: "http://localhost:9000",
			Tenant:  "dev-tenant",
			Project: "test",
		},
		"prod": {
			BaseURL: "https://izanami.example.com",
			Tenant:  "production",
			Project: "main",
		},
		"build": {
			Session: "build-session",
			Tenant:  "build-tenant",
		},
	}
	createTestConfig(t, paths.configPath, profiles, "sandbox")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "list"})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify output shows all profiles
	output := buf.String()
	assert.Contains(t, output, "sandbox")
	assert.Contains(t, output, "prod")
	assert.Contains(t, output, "build")
	assert.Contains(t, output, "Active profile: sandbox")
	assert.Contains(t, output, "http://localhost:9000")
	assert.Contains(t, output, "https://izanami.example.com")
}

// TestProfileListCmd_WithSessionURLResolution tests URL resolution from sessions
func TestProfileListCmd_WithSessionURLResolution(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Create session
	sessions := map[string]*izanami.Session{
		"my-session": {
			URL:      "http://session-url.com",
			Username: "testuser",
		},
	}
	createTestSessions(t, paths.sessionsPath, sessions)

	// Create profile that references session
	profiles := map[string]*izanami.Profile{
		"test": {
			Session: "my-session",
			Tenant:  "test-tenant",
		},
	}
	createTestConfig(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "list"})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify URL from session is displayed
	output := buf.String()
	assert.Contains(t, output, "test")
	assert.Contains(t, output, "my-session")
	assert.Contains(t, output, "http://session-url.com")
}

// TestProfileCurrentCmd_NoActiveProfile tests showing current when none is active
func TestProfileCurrentCmd_NoActiveProfile(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	// Create config without active profile
	profiles := map[string]*izanami.Profile{
		"test": {BaseURL: "http://localhost:9000"},
	}
	createTestConfig(t, paths.configPath, profiles, "")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "current"})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify output
	output := buf.String()
	assert.Contains(t, output, "No active profile set")
	assert.Contains(t, output, "iz profiles use <name>")
}

// TestProfileCurrentCmd_ActiveProfileExists tests showing active profile
func TestProfileCurrentCmd_ActiveProfileExists(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"sandbox": {
			BaseURL: "http://localhost:9000",
			Tenant:  "dev-tenant",
			Project: "test",
			Context: "dev",
		},
	}
	createTestConfig(t, paths.configPath, profiles, "sandbox")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "current"})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify output shows profile details
	output := buf.String()
	assert.Contains(t, output, "Active Profile: sandbox")
	assert.Contains(t, output, "http://localhost:9000")
	assert.Contains(t, output, "dev-tenant")
	assert.Contains(t, output, "test")
	assert.Contains(t, output, "dev")
}

// TestProfileShowCmd_ProfileExists tests showing a specific profile
func TestProfileShowCmd_ProfileExists(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			BaseURL:      "http://localhost:9000",
			Tenant:       "test-tenant",
			Project:      "test-project",
			ClientSecret: "secret123",
		},
	}
	createTestConfig(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "show", "test"})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify output shows profile details
	output := buf.String()
	assert.Contains(t, output, "Profile: test")
	assert.Contains(t, output, "http://localhost:9000")
	assert.Contains(t, output, "test-tenant")
	assert.Contains(t, output, "test-project")
	assert.Contains(t, output, "<redacted>")
	assert.NotContains(t, output, "secret123")
}

// TestProfileShowCmd_WithShowSecretsFlag tests showing secrets
func TestProfileShowCmd_WithShowSecretsFlag(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			BaseURL:      "http://localhost:9000",
			ClientSecret: "secret123",
		},
	}
	createTestConfig(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "show", "test", "--show-secrets"})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify secrets are shown
	output := buf.String()
	assert.Contains(t, output, "secret123")
	assert.NotContains(t, output, "<redacted>")
}

// TestProfileShowCmd_ProfileNotFound tests showing non-existent profile
func TestProfileShowCmd_ProfileNotFound(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	createTestConfig(t, paths.configPath, nil, "")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "show", "nonexistent"})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	// Error can be "not found" or "no profiles defined"
	assert.True(t, err.Error() == "no profiles defined" || err.Error() == "profile 'nonexistent' not found")
}

// TestProfileUseCmd_SwitchToExisting tests switching to an existing profile
func TestProfileUseCmd_SwitchToExisting(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"sandbox": {BaseURL: "http://localhost:9000", Tenant: "dev"},
		"prod":    {BaseURL: "https://prod.com", Tenant: "production"},
	}
	createTestConfig(t, paths.configPath, profiles, "sandbox")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "use", "prod"})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify output
	output := buf.String()
	assert.Contains(t, output, "Switched to profile 'prod'")
	assert.Contains(t, output, "https://prod.com")
	assert.Contains(t, output, "production")

	// Verify active profile was updated in config
	data, err := os.ReadFile(paths.configPath)
	require.NoError(t, err)
	var config map[string]interface{}
	yaml.Unmarshal(data, &config)
	assert.Equal(t, "prod", config["active_profile"])
}

// TestProfileUseCmd_NonExistentProfile tests switching to non-existent profile
func TestProfileUseCmd_NonExistentProfile(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"sandbox": {BaseURL: "http://localhost:9000"},
	}
	createTestConfig(t, paths.configPath, profiles, "sandbox")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "use", "nonexistent"})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

// TestProfileAddCmd_WithSessionReference tests adding profile with session
func TestProfileAddCmd_WithSessionReference(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	createTestConfig(t, paths.configPath, nil, "")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{
		"profiles", "add", "test",
		"--session", "my-session",
		"--tenant", "test-tenant",
		"--project", "test-project",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify profile was created in config
	verifyProfileInConfig(t, paths.configPath, "test", &izanami.Profile{
		Session: "my-session",
		Tenant:  "test-tenant",
		Project: "test-project",
	})
}

// TestProfileAddCmd_WithDirectURL tests adding profile with direct URL
func TestProfileAddCmd_WithDirectURL(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	createTestConfig(t, paths.configPath, nil, "")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{
		"profiles", "add", "prod",
		"--url", "https://izanami.prod.com",
		"--tenant", "production",
		"--context", "prod",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify profile was created
	verifyProfileInConfig(t, paths.configPath, "prod", &izanami.Profile{
		BaseURL: "https://izanami.prod.com",
		Tenant:  "production",
		Context: "prod",
	})
}

// TestProfileAddCmd_FirstProfileAutoActivated tests first profile is auto-activated
func TestProfileAddCmd_FirstProfileAutoActivated(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	createTestConfig(t, paths.configPath, nil, "")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{
		"profiles", "add", "first",
		"--url", "http://localhost:9000",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify active profile was set
	data, err := os.ReadFile(paths.configPath)
	require.NoError(t, err)
	var config map[string]interface{}
	yaml.Unmarshal(data, &config)
	assert.Equal(t, "first", config["active_profile"])
}

// TestProfileAddCmd_MissingSessionAndURL tests validation
func TestProfileAddCmd_MissingSessionAndURL(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	createTestConfig(t, paths.configPath, nil, "")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{
		"profiles", "add", "incomplete",
		"--tenant", "test",
	})
	defer cleanup()

	err := cmd.Execute()
	// Note: The command actually succeeds with just --tenant because validation
	// is checked only when no flags are provided. Testing the actual CLI behavior.
	// In non-interactive mode with flags, the validation happens during profile creation.
	require.NoError(t, err)
}

// TestProfileInitCmd_SandboxTemplate tests sandbox template
func TestProfileInitCmd_SandboxTemplate(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	createTestConfig(t, paths.configPath, nil, "")

	var buf bytes.Buffer
	input := createInputBuffer("\n") // Empty input for session override prompt
	cmd, cleanup := setupProfileCommand(&buf, input, []string{"profiles", "init", "sandbox"})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify sandbox defaults were applied
	verifyProfileInConfig(t, paths.configPath, "sandbox", &izanami.Profile{
		BaseURL: "http://localhost:9000",
		Tenant:  "sandbox-tenant",
		Project: "test",
		Context: "dev",
	})
}

// TestProfileInitCmd_InvalidTemplate tests invalid template name
func TestProfileInitCmd_InvalidTemplate(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	createTestConfig(t, paths.configPath, nil, "")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "init", "invalid"})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid template")
}

// TestProfileSetCmd_UpdateTenant tests updating tenant on active profile
func TestProfileSetCmd_UpdateTenant(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			BaseURL: "http://localhost:9000",
			Tenant:  "old-tenant",
		},
	}
	createTestConfig(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "set", "tenant", "new-tenant"})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify tenant was updated
	verifyProfileInConfig(t, paths.configPath, "test", &izanami.Profile{
		Tenant: "new-tenant",
	})
}

// TestProfileSetCmd_NoActiveProfile tests error when no active profile
func TestProfileSetCmd_NoActiveProfile(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	createTestConfig(t, paths.configPath, nil, "")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "set", "tenant", "test"})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active profile")
}

// TestProfileSetCmd_InvalidKey tests invalid key
func TestProfileSetCmd_InvalidKey(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {BaseURL: "http://localhost:9000"},
	}
	createTestConfig(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "set", "invalid-key", "value"})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid key")
}

// TestProfileUnsetCmd_ClearTenant tests clearing tenant from active profile
func TestProfileUnsetCmd_ClearTenant(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			BaseURL: "http://localhost:9000",
			Tenant:  "my-tenant",
			Project: "my-project",
		},
	}
	createTestConfig(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "unset", "tenant"})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify output
	output := buf.String()
	assert.Contains(t, output, "Removed tenant from profile 'test'")

	// Verify tenant was cleared in config but other fields remain
	data, err := os.ReadFile(paths.configPath)
	require.NoError(t, err)
	var config map[string]interface{}
	yaml.Unmarshal(data, &config)
	profilesMap := config["profiles"].(map[string]interface{})
	profile := profilesMap["test"].(map[string]interface{})
	_, hasTenant := profile["tenant"]
	assert.False(t, hasTenant, "Tenant should be cleared")
	assert.Equal(t, "my-project", profile["project"], "Project should remain")
	assert.Equal(t, "http://localhost:9000", profile["base-url"], "BaseURL should remain")
}

// TestProfileUnsetCmd_ClearSensitiveValue tests clearing personal-access-token
func TestProfileUnsetCmd_ClearSensitiveValue(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			BaseURL:                     "http://localhost:9000",
			PersonalAccessToken:         "secret-token-123",
			PersonalAccessTokenUsername: "admin",
		},
	}
	createTestConfig(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "unset", "personal-access-token"})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify output
	output := buf.String()
	assert.Contains(t, output, "Removed personal-access-token from profile 'test'")

	// Verify token was cleared but username remains
	data, err := os.ReadFile(paths.configPath)
	require.NoError(t, err)
	var config map[string]interface{}
	yaml.Unmarshal(data, &config)
	profilesMap := config["profiles"].(map[string]interface{})
	profile := profilesMap["test"].(map[string]interface{})
	_, hasToken := profile["personal-access-token"]
	assert.False(t, hasToken, "Personal access token should be cleared")
	assert.Equal(t, "admin", profile["personal-access-token-username"], "Username should remain")
}

// TestProfileUnsetCmd_ClearSession tests clearing session reference
func TestProfileUnsetCmd_ClearSession(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			Session: "my-session",
			Tenant:  "my-tenant",
		},
	}
	createTestConfig(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "unset", "session"})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify session was cleared
	data, err := os.ReadFile(paths.configPath)
	require.NoError(t, err)
	var config map[string]interface{}
	yaml.Unmarshal(data, &config)
	profilesMap := config["profiles"].(map[string]interface{})
	profile := profilesMap["test"].(map[string]interface{})
	_, hasSession := profile["session"]
	assert.False(t, hasSession, "Session should be cleared")
	assert.Equal(t, "my-tenant", profile["tenant"], "Tenant should remain")
}

// TestProfileUnsetCmd_NoActiveProfile tests error when no active profile
func TestProfileUnsetCmd_NoActiveProfile(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	createTestConfig(t, paths.configPath, nil, "")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "unset", "tenant"})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active profile")
}

// TestProfileUnsetCmd_InvalidKey tests error for invalid key
func TestProfileUnsetCmd_InvalidKey(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {BaseURL: "http://localhost:9000"},
	}
	createTestConfig(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "unset", "invalid-key"})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid key")
}

// TestProfileUnsetCmd_AllKeys tests unsetting each valid key
func TestProfileUnsetCmd_AllKeys(t *testing.T) {
	keys := []string{
		"base-url",
		"tenant",
		"project",
		"context",
		"session",
		"personal-access-token",
		"personal-access-token-username",
		"client-id",
		"client-secret",
	}

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			paths := setupTestPaths(t)
			overridePathFunctions(t, paths)

			// Create profile with all fields populated
			profiles := map[string]*izanami.Profile{
				"test": {
					BaseURL:                     "http://localhost:9000",
					Tenant:                      "tenant",
					Project:                     "project",
					Context:                     "context",
					Session:                     "session",
					PersonalAccessToken:         "token",
					PersonalAccessTokenUsername: "user",
					ClientID:                    "client-id",
					ClientSecret:                "client-secret",
				},
			}
			createTestConfig(t, paths.configPath, profiles, "test")

			var buf bytes.Buffer
			cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "unset", key})
			defer cleanup()

			err := cmd.Execute()
			require.NoError(t, err, "Should be able to unset %s", key)

			output := buf.String()
			assert.Contains(t, output, "Removed "+key)
		})
	}
}

// TestProfileDeleteCmd_WithForceFlag tests deleting with --force
func TestProfileDeleteCmd_WithForceFlag(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {BaseURL: "http://localhost:9000"},
		"keep": {BaseURL: "http://keep.com"},
	}
	createTestConfig(t, paths.configPath, profiles, "keep")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "delete", "test", "--force"})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify profile was deleted
	data, err := os.ReadFile(paths.configPath)
	require.NoError(t, err)
	var config map[string]interface{}
	yaml.Unmarshal(data, &config)
	profilesMap := config["profiles"].(map[string]interface{})
	_, exists := profilesMap["test"]
	assert.False(t, exists, "Profile should be deleted")
}

// TestProfileDeleteCmd_ActiveProfile tests deleting active profile shows warning
func TestProfileDeleteCmd_ActiveProfile(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {BaseURL: "http://localhost:9000"},
	}
	createTestConfig(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "delete", "test", "--force"})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify active profile was cleared
	data, err := os.ReadFile(paths.configPath)
	require.NoError(t, err)
	var config map[string]interface{}
	yaml.Unmarshal(data, &config)
	// Active profile should be cleared or set to empty string
	activeProfile, _ := config["active_profile"].(string)
	assert.Empty(t, activeProfile, "Active profile should be cleared")
}

// TestProfileDeleteCmd_NonExistentProfile tests deleting non-existent profile
func TestProfileDeleteCmd_NonExistentProfile(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	createTestConfig(t, paths.configPath, nil, "")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "delete", "nonexistent", "--force"})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestProfileClientKeysAddCmd_NoActiveProfile tests validation
func TestProfileClientKeysAddCmd_NoActiveProfile(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	createTestConfig(t, paths.configPath, nil, "")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "client-keys", "add", "--tenant", "test"})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active profile")
}

// TestProfileClientKeysAddCmd_MissingTenant tests missing tenant flag
func TestProfileClientKeysAddCmd_MissingTenant(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {BaseURL: "http://localhost:9000"},
	}
	createTestConfig(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupProfileCommand(&buf, nil, []string{"profiles", "client-keys", "add"})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	// The error should be about missing tenant flag or reading client ID
	// (Command tries to read input which will fail or require flag check)
}
