package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

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

// ============================================================================
// TempTenant - Temporary tenant management for integration tests
// ============================================================================

// TempTenant manages a temporary tenant for integration tests
type TempTenant struct {
	Name        string
	Description string
	client      *izanami.Client
	ctx         context.Context
	created     bool
}

// NewTempTenant creates a new temporary tenant helper with auto-generated unique name
func NewTempTenant(t *testing.T, client *izanami.Client, description string) *TempTenant {
	t.Helper()
	return &TempTenant{
		Name:        fmt.Sprintf("test-tenant-%d", time.Now().UnixNano()),
		Description: description,
		client:      client,
		ctx:         context.Background(),
		created:     false,
	}
}

// WithName sets a custom name for the tenant (for specific test scenarios)
func (tt *TempTenant) WithName(name string) *TempTenant {
	tt.Name = name
	return tt
}

// Create creates the tenant on the server
func (tt *TempTenant) Create(t *testing.T) error {
	t.Helper()
	err := tt.client.CreateTenant(tt.ctx, map[string]interface{}{
		"name":        tt.Name,
		"description": tt.Description,
	})
	if err == nil {
		tt.created = true
		t.Logf("TempTenant created: %s", tt.Name)
	}
	return err
}

// MustCreate creates the tenant and fails the test on error
func (tt *TempTenant) MustCreate(t *testing.T) *TempTenant {
	t.Helper()
	err := tt.Create(t)
	require.NoError(t, err, "Failed to create temp tenant %s", tt.Name)
	return tt
}

// Get retrieves the current tenant state from server
func (tt *TempTenant) Get(t *testing.T) *izanami.Tenant {
	t.Helper()
	tenant, err := izanami.GetTenant(tt.client, tt.ctx, tt.Name, izanami.ParseTenant)
	require.NoError(t, err, "Failed to get temp tenant %s", tt.Name)
	return tenant
}

// Update updates the tenant description
func (tt *TempTenant) Update(t *testing.T, description string) *TempTenant {
	t.Helper()
	err := tt.client.UpdateTenant(tt.ctx, tt.Name, map[string]interface{}{
		"name":        tt.Name,
		"description": description,
	})
	require.NoError(t, err, "Failed to update temp tenant %s", tt.Name)
	tt.Description = description
	return tt
}

// Delete removes the tenant from server
func (tt *TempTenant) Delete(t *testing.T) {
	t.Helper()
	if !tt.created {
		return
	}
	err := tt.client.DeleteTenant(tt.ctx, tt.Name)
	if err != nil {
		t.Logf("Warning: failed to delete temp tenant %s: %v", tt.Name, err)
	} else {
		t.Logf("TempTenant deleted: %s", tt.Name)
		tt.created = false
	}
}

// Cleanup registers automatic deletion via t.Cleanup()
func (tt *TempTenant) Cleanup(t *testing.T) *TempTenant {
	t.Helper()
	t.Cleanup(func() {
		tt.Delete(t)
	})
	return tt
}

// MarkCreated marks the tenant as created (for when creation happens outside TempTenant)
func (tt *TempTenant) MarkCreated() *TempTenant {
	tt.created = true
	return tt
}
