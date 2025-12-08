package cmd

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// Database connection singleton for cleanup operations
var (
	testDB     *sql.DB
	testDBOnce sync.Once
	testDBErr  error
)

// getTestDB returns a singleton database connection for test cleanup operations.
// The connection is lazily initialized on first call.
func getTestDB(dsn string) (*sql.DB, error) {
	testDBOnce.Do(func() {
		testDB, testDBErr = sql.Open("postgres", dsn)
		if testDBErr != nil {
			return
		}
		// Verify connection
		testDBErr = testDB.Ping()
		if testDBErr != nil {
			testDB.Close()
			testDB = nil
			return
		}
		// Set conservative pool settings for test use
		testDB.SetMaxOpenConns(1)
		testDB.SetMaxIdleConns(1)
	})
	return testDB, testDBErr
}

// cleanupPostgresConnections terminates lingering database connections after tenant deletion.
// This prevents connection pool exhaustion. Terminates:
// - LISTEN connections (except the main "izanami" listener)
// - Idle vertx-pg-client connections
// If IZ_TEST_DB_DSN is not set or connection fails, it logs a warning but does not fail the test.
func cleanupPostgresListeners(t *testing.T) {
	t.Helper()

	dsn := os.Getenv("IZ_TEST_DB_DSN")
	if dsn == "" {
		// DSN not configured - skip cleanup silently
		return
	}

	db, err := getTestDB(dsn)
	if err != nil {
		t.Logf("Warning: PostgreSQL cleanup skipped - connection failed: %v", err)
		return
	}

	query := `
		SELECT pg_terminate_backend(pid)
		FROM pg_stat_activity
		WHERE pid <> pg_backend_pid()
			AND backend_type = 'client backend'
			AND ((query LIKE 'LISTEN%' AND query <> 'LISTEN "izanami"')
				OR application_name = 'vertx-pg-client')
	`

	result, err := db.ExecContext(context.Background(), query)
	if err != nil {
		t.Logf("Warning: PostgreSQL connection cleanup failed: %v", err)
		return
	}

	if rows, _ := result.RowsAffected(); rows > 0 {
		t.Logf("PostgreSQL cleanup: terminated %d lingering connections", rows)
	}
}

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

// Delete removes the tenant from server and cleans up any lingering database connections
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

		// Clean up any lingering PostgreSQL LISTEN connections
		// This prevents connection pool exhaustion after tenant deletion
		cleanupPostgresListeners(t)
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

// ============================================================================
// TempProject - Temporary project management for integration tests
// ============================================================================

// TempProject manages a temporary project for integration tests
type TempProject struct {
	Name        string
	Description string
	Tenant      string
	client      *izanami.Client
	ctx         context.Context
	created     bool
}

// NewTempProject creates a new temporary project helper with auto-generated unique name
func NewTempProject(t *testing.T, client *izanami.Client, tenant, description string) *TempProject {
	t.Helper()
	return &TempProject{
		Name:        fmt.Sprintf("test-project-%d", time.Now().UnixNano()),
		Description: description,
		Tenant:      tenant,
		client:      client,
		ctx:         context.Background(),
		created:     false,
	}
}

// WithName sets a custom name for the project (for specific test scenarios)
func (tp *TempProject) WithName(name string) *TempProject {
	tp.Name = name
	return tp
}

// Create creates the project on the server
func (tp *TempProject) Create(t *testing.T) error {
	t.Helper()
	err := tp.client.CreateProject(tp.ctx, tp.Tenant, map[string]interface{}{
		"name":        tp.Name,
		"description": tp.Description,
	})
	if err == nil {
		tp.created = true
		t.Logf("TempProject created: %s/%s", tp.Tenant, tp.Name)
	}
	return err
}

// MustCreate creates the project and fails the test on error
func (tp *TempProject) MustCreate(t *testing.T) *TempProject {
	t.Helper()
	err := tp.Create(t)
	require.NoError(t, err, "Failed to create temp project %s/%s", tp.Tenant, tp.Name)
	return tp
}

// Get retrieves the current project state from server
func (tp *TempProject) Get(t *testing.T) *izanami.Project {
	t.Helper()
	project, err := izanami.GetProject(tp.client, tp.ctx, tp.Tenant, tp.Name, izanami.ParseProject)
	require.NoError(t, err, "Failed to get temp project %s/%s", tp.Tenant, tp.Name)
	return project
}

// Delete removes the project from server
func (tp *TempProject) Delete(t *testing.T) {
	t.Helper()
	if !tp.created {
		return
	}
	err := tp.client.DeleteProject(tp.ctx, tp.Tenant, tp.Name)
	if err != nil {
		t.Logf("Warning: failed to delete temp project %s/%s: %v", tp.Tenant, tp.Name, err)
	} else {
		t.Logf("TempProject deleted: %s/%s", tp.Tenant, tp.Name)
		tp.created = false
	}
}

// Cleanup registers automatic deletion via t.Cleanup()
func (tp *TempProject) Cleanup(t *testing.T) *TempProject {
	t.Helper()
	t.Cleanup(func() {
		tp.Delete(t)
	})
	return tp
}

// MarkCreated marks the project as created (for when creation happens outside TempProject)
func (tp *TempProject) MarkCreated() *TempProject {
	tp.created = true
	return tp
}
