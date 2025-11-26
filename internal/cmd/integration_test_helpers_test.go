package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// IntegrationTestEnv holds the test environment configuration
type IntegrationTestEnv struct {
	TempDir      string
	ConfigDir    string
	ConfigPath   string
	SessionsPath string
	BaseURL      string
	Username     string
	Password     string
	ClientID     string
	ClientSecret string
}

// setupIntegrationTest creates an isolated test environment and returns config from env vars.
// Tests are skipped if IZ_TEST_BASE_URL is not set.
func setupIntegrationTest(t *testing.T) *IntegrationTestEnv {
	t.Helper()

	// Skip if env vars not set
	baseURL := os.Getenv("IZ_TEST_BASE_URL")
	if baseURL == "" {
		t.Skip("IZ_TEST_BASE_URL not set, skipping integration test")
	}

	// Create temp dir: /tmp/TestXXX<random>/
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".config", "iz")
	configPath := filepath.Join(configDir, "config.yaml")
	sessionsPath := filepath.Join(tempDir, ".izsessions")

	// Override path functions to use temp directory
	origConfigDir := izanami.GetConfigDir
	origSessionsPath := izanami.GetSessionsPath

	izanami.SetGetConfigDirFunc(func() string { return configDir })
	izanami.SetGetSessionsPathFunc(func() string { return sessionsPath })

	t.Cleanup(func() {
		izanami.SetGetConfigDirFunc(origConfigDir)
		izanami.SetGetSessionsPathFunc(origSessionsPath)
	})

	return &IntegrationTestEnv{
		TempDir:      tempDir,
		ConfigDir:    configDir,
		ConfigPath:   configPath,
		SessionsPath: sessionsPath,
		BaseURL:      baseURL,
		Username:     os.Getenv("IZ_TEST_USERNAME"),
		Password:     os.Getenv("IZ_TEST_PASSWORD"),
		ClientID:     os.Getenv("IZ_TEST_CLIENT_ID"),
		ClientSecret: os.Getenv("IZ_TEST_CLIENT_SECRET"),
	}
}

// SessionsFileExists checks if the sessions file exists
func (env *IntegrationTestEnv) SessionsFileExists() bool {
	_, err := os.Stat(env.SessionsPath)
	return err == nil
}

// ConfigFileExists checks if the config file exists
func (env *IntegrationTestEnv) ConfigFileExists() bool {
	_, err := os.Stat(env.ConfigPath)
	return err == nil
}

// ReadSessionsFile reads the sessions file content
func (env *IntegrationTestEnv) ReadSessionsFile(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(env.SessionsPath)
	if err != nil {
		t.Fatalf("Failed to read sessions file: %v", err)
	}
	return string(content)
}

// ReadConfigFile reads the config file content
func (env *IntegrationTestEnv) ReadConfigFile(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(env.ConfigPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}
	return string(content)
}

// Login performs login and sets up profile/session for authenticated tests.
// Returns the JWT token. Skips the test if credentials are not configured.
func (env *IntegrationTestEnv) Login(t *testing.T) string {
	t.Helper()

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

	cmd.SetArgs([]string{"login", env.BaseURL, env.Username, "--password", env.Password})
	err := cmd.Execute()

	// Cleanup command state
	loginCmd.SetIn(nil)
	loginCmd.SetOut(nil)
	loginCmd.SetErr(nil)

	require.NoError(t, err, "Login should succeed: %s", buf.String())

	// Return the JWT token
	return env.GetJwtToken(t)
}

// GetJwtToken retrieves the JWT token from the saved session
func (env *IntegrationTestEnv) GetJwtToken(t *testing.T) string {
	t.Helper()

	sessions, err := izanami.LoadSessions()
	require.NoError(t, err, "Should load sessions")

	for _, session := range sessions.Sessions {
		if session.URL == env.BaseURL && session.Username == env.Username {
			return session.JwtToken
		}
	}

	t.Fatalf("No session found for %s@%s", env.Username, env.BaseURL)
	return ""
}

// NewAuthenticatedClient creates an Izanami client with JWT authentication
func (env *IntegrationTestEnv) NewAuthenticatedClient(t *testing.T) *izanami.Client {
	t.Helper()

	token := env.GetJwtToken(t)

	config := &izanami.Config{
		BaseURL:  env.BaseURL,
		Username: env.Username,
		JwtToken: token,
		Timeout:  30,
	}

	client, err := izanami.NewClient(config)
	require.NoError(t, err, "Should create authenticated client")

	return client
}
