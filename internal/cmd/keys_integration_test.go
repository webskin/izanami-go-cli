package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// ============================================================================
// TempAPIKey - Temporary API key management for integration tests
// ============================================================================

// TempAPIKey manages a temporary API key for integration tests
type TempAPIKey struct {
	Name         string
	ClientID     string
	ClientSecret string // Returned only on creation
	Description  string
	Tenant       string
	Projects     []string
	Enabled      bool
	Admin        bool
	client       *izanami.Client
	ctx          context.Context
	created      bool
}

// NewTempAPIKey creates a new temporary API key helper with auto-generated unique name
func NewTempAPIKey(t *testing.T, client *izanami.Client, tenant string) *TempAPIKey {
	t.Helper()
	name := fmt.Sprintf("testkey%d", time.Now().UnixNano())
	return &TempAPIKey{
		Name:        name,
		Description: "Test API key created by integration test",
		Tenant:      tenant,
		Projects:    []string{},
		Enabled:     true,
		Admin:       false,
		client:      client,
		ctx:         context.Background(),
		created:     false,
	}
}

// WithName sets a custom name for the API key
func (tk *TempAPIKey) WithName(name string) *TempAPIKey {
	tk.Name = name
	return tk
}

// WithDescription sets a custom description for the API key
func (tk *TempAPIKey) WithDescription(desc string) *TempAPIKey {
	tk.Description = desc
	return tk
}

// WithProjects sets the projects for the API key
func (tk *TempAPIKey) WithProjects(projects []string) *TempAPIKey {
	tk.Projects = projects
	return tk
}

// WithEnabled sets the enabled state for the API key
func (tk *TempAPIKey) WithEnabled(enabled bool) *TempAPIKey {
	tk.Enabled = enabled
	return tk
}

// WithAdmin sets the admin state for the API key
func (tk *TempAPIKey) WithAdmin(admin bool) *TempAPIKey {
	tk.Admin = admin
	return tk
}

// Create creates the API key on the server
func (tk *TempAPIKey) Create(t *testing.T) (*izanami.APIKey, error) {
	t.Helper()
	data := map[string]interface{}{
		"name":        tk.Name,
		"description": tk.Description,
		"enabled":     tk.Enabled,
		"admin":       tk.Admin,
	}
	if len(tk.Projects) > 0 {
		data["projects"] = tk.Projects
	}

	result, err := tk.client.CreateAPIKey(tk.ctx, tk.Tenant, data)
	if err == nil {
		tk.created = true
		tk.ClientID = result.ClientID
		tk.ClientSecret = result.ClientSecret
		t.Logf("TempAPIKey created: %s (clientID: %s, tenant: %s)", tk.Name, tk.ClientID, tk.Tenant)
	}
	return result, err
}

// MustCreate creates the API key and fails the test on error
func (tk *TempAPIKey) MustCreate(t *testing.T) *TempAPIKey {
	t.Helper()
	_, err := tk.Create(t)
	require.NoError(t, err, "Failed to create temp API key %s", tk.Name)
	return tk
}

// Delete removes the API key from server
func (tk *TempAPIKey) Delete(t *testing.T) {
	t.Helper()
	if !tk.created || tk.ClientID == "" {
		return
	}
	err := tk.client.DeleteAPIKey(tk.ctx, tk.Tenant, tk.ClientID)
	if err != nil {
		t.Logf("Warning: failed to delete temp API key %s: %v", tk.ClientID, err)
	} else {
		t.Logf("TempAPIKey deleted: %s", tk.ClientID)
		tk.created = false
	}
}

// Cleanup registers automatic deletion via t.Cleanup()
func (tk *TempAPIKey) Cleanup(t *testing.T) *TempAPIKey {
	t.Helper()
	t.Cleanup(func() {
		tk.Delete(t)
	})
	return tk
}

// MarkCreated marks the API key as created (for when creation happens outside TempAPIKey)
func (tk *TempAPIKey) MarkCreated(clientID string) *TempAPIKey {
	tk.created = true
	tk.ClientID = clientID
	return tk
}

// ============================================================================
// Test Setup Helpers
// ============================================================================

// setupKeysTest sets up the test environment and logs in
func setupKeysTest(t *testing.T, env *IntegrationTestEnv) func() {
	t.Helper()
	env.Login(t)

	// Get JWT token from session
	token := env.GetJwtToken(t)

	// Save original values
	origCfg := cfg
	origOutputFormat := outputFormat
	origTenant := tenant

	// Set up config
	cfg = &izanami.Config{
		BaseURL:  env.BaseURL,
		Username: env.Username,
		JwtToken: token,
		Timeout:  30,
	}
	outputFormat = "table"
	tenant = "" // Will be set per-test

	// Reset keys-specific flags to defaults
	keyName = ""
	keyDescription = ""
	keyProjects = []string{}
	keyEnabled = true
	keyAdmin = false
	keysDeleteForce = false

	return func() {
		cfg = origCfg
		outputFormat = origOutputFormat
		tenant = origTenant
		keyName = ""
		keyDescription = ""
		keyProjects = []string{}
		keyEnabled = true
		keyAdmin = false
		keysDeleteForce = false
	}
}

// executeKeysCommand executes a keys command with proper setup
func executeKeysCommand(t *testing.T, args []string) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	keysCmd.SetOut(&buf)
	keysCmd.SetErr(&buf)
	keysListCmd.SetOut(&buf)
	keysListCmd.SetErr(&buf)
	keysGetCmd.SetOut(&buf)
	keysGetCmd.SetErr(&buf)
	keysCreateCmd.SetOut(&buf)
	keysCreateCmd.SetErr(&buf)
	keysUpdateCmd.SetOut(&buf)
	keysUpdateCmd.SetErr(&buf)
	keysDeleteCmd.SetOut(&buf)
	keysDeleteCmd.SetErr(&buf)

	fullArgs := append([]string{"admin", "keys"}, args...)
	cmd.SetArgs(fullArgs)
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	keysCmd.SetOut(nil)
	keysCmd.SetErr(nil)
	keysListCmd.SetOut(nil)
	keysListCmd.SetErr(nil)
	keysGetCmd.SetOut(nil)
	keysGetCmd.SetErr(nil)
	keysCreateCmd.SetOut(nil)
	keysCreateCmd.SetErr(nil)
	keysUpdateCmd.SetOut(nil)
	keysUpdateCmd.SetErr(nil)
	keysDeleteCmd.SetOut(nil)
	keysDeleteCmd.SetErr(nil)

	return buf.String(), err
}

// executeKeysCommandWithInput executes a keys command with stdin input
func executeKeysCommandWithInput(t *testing.T, args []string, input string) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	inputBuf := bytes.NewBufferString(input)

	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(inputBuf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminCmd.SetIn(inputBuf)
	keysCmd.SetOut(&buf)
	keysCmd.SetErr(&buf)
	keysCmd.SetIn(inputBuf)
	keysDeleteCmd.SetOut(&buf)
	keysDeleteCmd.SetErr(&buf)
	keysDeleteCmd.SetIn(inputBuf)

	fullArgs := append([]string{"admin", "keys"}, args...)
	cmd.SetArgs(fullArgs)
	err := cmd.Execute()

	adminCmd.SetIn(nil)
	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	keysCmd.SetIn(nil)
	keysCmd.SetOut(nil)
	keysCmd.SetErr(nil)
	keysDeleteCmd.SetIn(nil)
	keysDeleteCmd.SetOut(nil)
	keysDeleteCmd.SetErr(nil)

	return buf.String(), err
}

// ============================================================================
// Keys List Tests
// ============================================================================

func TestIntegration_KeysListAfterLogin(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	// Create a key to ensure we have at least one
	tempKey := NewTempAPIKey(t, client, tempTenant.Name).MustCreate(t)
	defer tempKey.Delete(t)

	output, err := executeKeysCommand(t, []string{"list"})
	require.NoError(t, err)

	t.Logf("Keys list output:\n%s", output)

	// Should show the created key
	assert.Contains(t, output, tempKey.Name)
}

func TestIntegration_KeysListJSONOutput(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Keys JSON test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	outputFormat = "json"

	// Create a key
	tempKey := NewTempAPIKey(t, client, tempTenant.Name).
		WithDescription("JSON test key").
		MustCreate(t)
	defer tempKey.Delete(t)

	output, err := executeKeysCommand(t, []string{"list"})
	require.NoError(t, err)

	t.Logf("Keys list JSON output:\n%s", output)

	// Should be valid JSON
	var keys []map[string]interface{}
	err = json.Unmarshal([]byte(output), &keys)
	require.NoError(t, err, "Output should be valid JSON array")

	// Find our key in the response
	var found bool
	for _, key := range keys {
		if key["name"] == tempKey.Name {
			found = true
			assert.Equal(t, "JSON test key", key["description"])
			break
		}
	}
	assert.True(t, found, "Created key should be in the list")
}

func TestIntegration_KeysListEmpty(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant with no keys
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Empty keys test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	output, err := executeKeysCommand(t, []string{"list"})
	require.NoError(t, err)

	t.Logf("Keys list empty output:\n%s", output)
	assert.Contains(t, output, "No API keys found")
}

func TestIntegration_KeysListMissingTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Don't set tenant
	tenant = ""

	_, err := executeKeysCommand(t, []string{"list"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant is required")
}

// ============================================================================
// Keys Get Tests
// ============================================================================

func TestIntegration_KeysGetExisting(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	// Create a key
	tempKey := NewTempAPIKey(t, client, tempTenant.Name).
		WithDescription("Key to get").
		MustCreate(t)
	defer tempKey.Delete(t)

	output, err := executeKeysCommand(t, []string{"get", tempKey.ClientID})
	require.NoError(t, err)

	t.Logf("Keys get output:\n%s", output)

	// Should show the key details
	assert.Contains(t, output, tempKey.Name)
	assert.Contains(t, output, tempKey.ClientID)
}

func TestIntegration_KeysGetJSONOutput(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	outputFormat = "json"

	// Create a key
	tempKey := NewTempAPIKey(t, client, tempTenant.Name).
		WithDescription("Get JSON test").
		MustCreate(t)
	defer tempKey.Delete(t)

	output, err := executeKeysCommand(t, []string{"get", tempKey.ClientID})
	require.NoError(t, err)

	t.Logf("Keys get JSON output:\n%s", output)

	// Should be valid JSON
	var key map[string]interface{}
	err = json.Unmarshal([]byte(output), &key)
	require.NoError(t, err, "Output should be valid JSON object")

	assert.Equal(t, tempKey.Name, key["name"])
	assert.Equal(t, tempKey.ClientID, key["clientId"])
	assert.Equal(t, "Get JSON test", key["description"])
}

func TestIntegration_KeysGetNotFound(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	_, err := executeKeysCommand(t, []string{"get", "nonexistent-client-id-12345"})
	require.Error(t, err)
	// Should get a 404 or "not found" error
	assert.Contains(t, err.Error(), "not found")
}

func TestIntegration_KeysGetMissingArg(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	_, err := executeKeysCommand(t, []string{"get"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

// ============================================================================
// Keys Create Tests
// ============================================================================

func TestIntegration_KeysCreateBasic(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	keyNameArg := fmt.Sprintf("clikey%d", time.Now().UnixNano())

	// Create a TempAPIKey to track for cleanup (but don't create it yet)
	tempKey := NewTempAPIKey(t, client, tempTenant.Name).WithName(keyNameArg)
	defer tempKey.Delete(t)

	output, err := executeKeysCommand(t, []string{"create", keyNameArg})
	require.NoError(t, err)

	t.Logf("Keys create output:\n%s", output)
	assert.Contains(t, output, "API key created successfully")
	assert.Contains(t, output, keyNameArg)
	assert.Contains(t, output, "Client ID:")
	assert.Contains(t, output, "Client Secret:")
	assert.Contains(t, output, "IMPORTANT: Save the Client Secret")

	// Extract clientID from output for cleanup - use API to find it
	ctx := context.Background()
	keys, err := izanami.ListAPIKeys(client, ctx, tempTenant.Name, izanami.ParseAPIKeys)
	require.NoError(t, err)

	for _, key := range keys {
		if key.Name == keyNameArg {
			tempKey.MarkCreated(key.ClientID)
			break
		}
	}
	require.True(t, tempKey.created, "Created key should be found in list")
}

func TestIntegration_KeysCreateWithDescription(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	keyDescription = "My custom key description"

	keyNameArg := fmt.Sprintf("clikey%d", time.Now().UnixNano())

	// Create a TempAPIKey to track for cleanup
	tempKey := NewTempAPIKey(t, client, tempTenant.Name).WithName(keyNameArg)
	defer tempKey.Delete(t)

	output, err := executeKeysCommand(t, []string{"create", keyNameArg})
	require.NoError(t, err)

	t.Logf("Keys create with description output:\n%s", output)
	assert.Contains(t, output, "API key created successfully")

	// Verify via API
	ctx := context.Background()
	keys, err := izanami.ListAPIKeys(client, ctx, tempTenant.Name, izanami.ParseAPIKeys)
	require.NoError(t, err)

	for _, key := range keys {
		if key.Name == keyNameArg {
			tempKey.MarkCreated(key.ClientID)
			assert.Equal(t, "My custom key description", key.Description)
			break
		}
	}
	require.True(t, tempKey.created, "Created key should be found")
}

func TestIntegration_KeysCreateWithProjects(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	// Create a project to associate with the key
	tempProject := NewTempProject(t, client, tempTenant.Name, "Keys test project").MustCreate(t)
	defer tempProject.Delete(t)

	tenant = tempTenant.Name
	keyProjects = []string{tempProject.Name}

	keyNameArg := fmt.Sprintf("clikey%d", time.Now().UnixNano())

	// Create a TempAPIKey to track for cleanup
	tempKey := NewTempAPIKey(t, client, tempTenant.Name).WithName(keyNameArg)
	defer tempKey.Delete(t)

	output, err := executeKeysCommand(t, []string{"create", keyNameArg})
	require.NoError(t, err)

	t.Logf("Keys create with projects output:\n%s", output)
	assert.Contains(t, output, "API key created successfully")
	assert.Contains(t, output, "Projects:")

	// Verify via API
	ctx := context.Background()
	keys, err := izanami.ListAPIKeys(client, ctx, tempTenant.Name, izanami.ParseAPIKeys)
	require.NoError(t, err)

	for _, key := range keys {
		if key.Name == keyNameArg {
			tempKey.MarkCreated(key.ClientID)
			assert.Contains(t, key.Projects, tempProject.Name)
			break
		}
	}
	require.True(t, tempKey.created, "Created key should be found")
}

func TestIntegration_KeysCreateDisabled(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	keyEnabled = false

	keyNameArg := fmt.Sprintf("clikey%d", time.Now().UnixNano())

	// Create a TempAPIKey to track for cleanup
	tempKey := NewTempAPIKey(t, client, tempTenant.Name).WithName(keyNameArg)
	defer tempKey.Delete(t)

	output, err := executeKeysCommand(t, []string{"create", keyNameArg})
	require.NoError(t, err)

	t.Logf("Keys create disabled output:\n%s", output)
	assert.Contains(t, output, "API key created successfully")
	assert.Contains(t, output, "Enabled:       false")

	// Verify via API
	ctx := context.Background()
	keys, err := izanami.ListAPIKeys(client, ctx, tempTenant.Name, izanami.ParseAPIKeys)
	require.NoError(t, err)

	for _, key := range keys {
		if key.Name == keyNameArg {
			tempKey.MarkCreated(key.ClientID)
			assert.False(t, key.Enabled)
			break
		}
	}
	require.True(t, tempKey.created, "Created key should be found")
}

func TestIntegration_KeysCreateAdmin(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	keyAdmin = true

	keyNameArg := fmt.Sprintf("clikey%d", time.Now().UnixNano())

	// Create a TempAPIKey to track for cleanup
	tempKey := NewTempAPIKey(t, client, tempTenant.Name).WithName(keyNameArg)
	defer tempKey.Delete(t)

	output, err := executeKeysCommand(t, []string{"create", keyNameArg})
	require.NoError(t, err)

	t.Logf("Keys create admin output:\n%s", output)
	assert.Contains(t, output, "API key created successfully")
	assert.Contains(t, output, "Admin:         true")

	// Verify via API
	ctx := context.Background()
	keys, err := izanami.ListAPIKeys(client, ctx, tempTenant.Name, izanami.ParseAPIKeys)
	require.NoError(t, err)

	for _, key := range keys {
		if key.Name == keyNameArg {
			tempKey.MarkCreated(key.ClientID)
			assert.True(t, key.Admin)
			break
		}
	}
	require.True(t, tempKey.created, "Created key should be found")
}

func TestIntegration_KeysCreateJSONOutput(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	outputFormat = "json"

	keyNameArg := fmt.Sprintf("clikey%d", time.Now().UnixNano())

	// Create a TempAPIKey to track for cleanup
	tempKey := NewTempAPIKey(t, client, tempTenant.Name).WithName(keyNameArg)
	defer tempKey.Delete(t)

	output, err := executeKeysCommand(t, []string{"create", keyNameArg})
	require.NoError(t, err)

	t.Logf("Keys create JSON output:\n%s", output)

	// Should be valid JSON with clientSecret
	var key map[string]interface{}
	err = json.Unmarshal([]byte(output), &key)
	require.NoError(t, err, "Output should be valid JSON object")

	assert.Equal(t, keyNameArg, key["name"])
	assert.NotEmpty(t, key["clientId"])
	assert.NotEmpty(t, key["clientSecret"])

	// Mark for cleanup
	tempKey.MarkCreated(key["clientId"].(string))
}

func TestIntegration_KeysCreateMissingTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Don't set tenant
	tenant = ""

	_, err := executeKeysCommand(t, []string{"create", "some-key"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant is required")
}

// ============================================================================
// Keys Update Tests
// ============================================================================

func TestIntegration_KeysUpdateName(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	// Create a key to update
	tempKey := NewTempAPIKey(t, client, tempTenant.Name).MustCreate(t)
	defer tempKey.Delete(t)

	newName := fmt.Sprintf("updatedkey%d", time.Now().UnixNano())
	keyName = newName

	output, err := executeKeysCommand(t, []string{"update", tempKey.ClientID})
	require.NoError(t, err)

	t.Logf("Keys update name output:\n%s", output)
	assert.Contains(t, output, "API key updated successfully")

	// Verify via API
	ctx := context.Background()
	key, err := client.GetAPIKey(ctx, tempTenant.Name, tempKey.ClientID)
	require.NoError(t, err)
	assert.Equal(t, newName, key.Name)
}

func TestIntegration_KeysUpdateDescription(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	// Create a key to update
	tempKey := NewTempAPIKey(t, client, tempTenant.Name).MustCreate(t)
	defer tempKey.Delete(t)

	keyDescription = "Updated description via CLI"

	output, err := executeKeysCommand(t, []string{"update", tempKey.ClientID})
	require.NoError(t, err)

	t.Logf("Keys update description output:\n%s", output)
	assert.Contains(t, output, "API key updated successfully")

	// Verify via API
	ctx := context.Background()
	key, err := client.GetAPIKey(ctx, tempTenant.Name, tempKey.ClientID)
	require.NoError(t, err)
	assert.Equal(t, "Updated description via CLI", key.Description)
}

func TestIntegration_KeysUpdateEnabled(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	// Create an enabled key
	tempKey := NewTempAPIKey(t, client, tempTenant.Name).WithEnabled(true).MustCreate(t)
	defer tempKey.Delete(t)

	// Disable it
	keyEnabled = false

	output, err := executeKeysCommand(t, []string{"update", tempKey.ClientID})
	require.NoError(t, err)

	t.Logf("Keys update enabled output:\n%s", output)
	assert.Contains(t, output, "API key updated successfully")

	// Verify via API
	ctx := context.Background()
	key, err := client.GetAPIKey(ctx, tempTenant.Name, tempKey.ClientID)
	require.NoError(t, err)
	assert.False(t, key.Enabled)
}

func TestIntegration_KeysUpdateNoChanges(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	// Create a key
	tempKey := NewTempAPIKey(t, client, tempTenant.Name).MustCreate(t)
	defer tempKey.Delete(t)

	// Don't set any flags
	keyName = ""
	keyDescription = ""
	keyProjects = []string{}

	_, err := executeKeysCommand(t, []string{"update", tempKey.ClientID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no fields to update")
}

func TestIntegration_KeysUpdateNotFound(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	keyDescription = "New description"

	_, err := executeKeysCommand(t, []string{"update", "nonexistent-client-id-12345"})
	require.Error(t, err)
	// Should get a 404 or error
}

// ============================================================================
// Keys Delete Tests
// ============================================================================

func TestIntegration_KeysDeleteWithForce(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	keysDeleteForce = true

	// Create a key to delete
	tempKey := NewTempAPIKey(t, client, tempTenant.Name).MustCreate(t)
	// Don't defer delete - we're deleting it in the test

	output, err := executeKeysCommand(t, []string{"delete", tempKey.ClientID})
	require.NoError(t, err)

	t.Logf("Keys delete with force output:\n%s", output)
	assert.Contains(t, output, "API key deleted successfully")

	// Verify key no longer exists
	ctx := context.Background()
	_, err = client.GetAPIKey(ctx, tempTenant.Name, tempKey.ClientID)
	require.Error(t, err, "Key should no longer exist")
	assert.Contains(t, err.Error(), "not found")
}

func TestIntegration_KeysDeleteWithConfirmation(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	keysDeleteForce = false

	// Create a key to delete
	tempKey := NewTempAPIKey(t, client, tempTenant.Name).MustCreate(t)
	// Don't defer delete - we're deleting it in the test

	// User types "y" to confirm
	output, err := executeKeysCommandWithInput(t, []string{"delete", tempKey.ClientID}, "y\n")
	require.NoError(t, err)

	t.Logf("Keys delete with confirmation output:\n%s", output)
	assert.Contains(t, output, "API key deleted successfully")

	// Verify key no longer exists
	ctx := context.Background()
	_, err = client.GetAPIKey(ctx, tempTenant.Name, tempKey.ClientID)
	require.Error(t, err, "Key should no longer exist")
}

func TestIntegration_KeysDeleteCancelled(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	keysDeleteForce = false

	// Create a key
	tempKey := NewTempAPIKey(t, client, tempTenant.Name).MustCreate(t)
	defer tempKey.Delete(t) // Will still exist after cancelled delete

	// User types "n" to cancel
	output, err := executeKeysCommandWithInput(t, []string{"delete", tempKey.ClientID}, "n\n")
	require.NoError(t, err)

	t.Logf("Keys delete cancelled output:\n%s", output)
	assert.Contains(t, output, "Cancelled")

	// Verify key still exists
	ctx := context.Background()
	key, err := client.GetAPIKey(ctx, tempTenant.Name, tempKey.ClientID)
	require.NoError(t, err, "Key should still exist after cancelled delete")
	assert.Equal(t, tempKey.ClientID, key.ClientID)
}

func TestIntegration_KeysDeleteNotFound(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	keysDeleteForce = true

	_, err := executeKeysCommand(t, []string{"delete", "nonexistent-client-id-12345"})
	require.Error(t, err)
	// Should get a 404 or "not found" error
}

func TestIntegration_KeysDeleteMissingArg(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupKeysTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	_, err := executeKeysCommand(t, []string{"delete"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

// ============================================================================
// API Direct Tests
// ============================================================================

func TestIntegration_APIListKeys(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant for isolation
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	// Create some keys
	tempKey1 := NewTempAPIKey(t, client, tempTenant.Name).WithDescription("First key").MustCreate(t)
	defer tempKey1.Delete(t)

	tempKey2 := NewTempAPIKey(t, client, tempTenant.Name).WithDescription("Second key").MustCreate(t)
	defer tempKey2.Delete(t)

	// List keys via API
	ctx := context.Background()
	keys, err := izanami.ListAPIKeys(client, ctx, tempTenant.Name, izanami.ParseAPIKeys)
	require.NoError(t, err)

	t.Logf("API listed %d keys", len(keys))

	// Should contain both keys
	var found1, found2 bool
	for _, key := range keys {
		if key.ClientID == tempKey1.ClientID {
			found1 = true
			assert.Equal(t, "First key", key.Description)
		}
		if key.ClientID == tempKey2.ClientID {
			found2 = true
			assert.Equal(t, "Second key", key.Description)
		}
	}
	assert.True(t, found1, "First key should be in list")
	assert.True(t, found2, "Second key should be in list")
}

func TestIntegration_APICreateKey(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant for isolation
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	keyName := fmt.Sprintf("apikey%d", time.Now().UnixNano())

	// Create key via API
	ctx := context.Background()
	result, err := client.CreateAPIKey(ctx, tempTenant.Name, map[string]interface{}{
		"name":        keyName,
		"description": "Created via API",
		"enabled":     true,
		"admin":       false,
	})
	require.NoError(t, err)

	// Cleanup
	defer func() {
		_ = client.DeleteAPIKey(ctx, tempTenant.Name, result.ClientID)
	}()

	// Verify it exists
	key, err := client.GetAPIKey(ctx, tempTenant.Name, result.ClientID)
	require.NoError(t, err)
	assert.Equal(t, keyName, key.Name)
	assert.Equal(t, "Created via API", key.Description)
	assert.NotEmpty(t, result.ClientSecret, "Client secret should be returned on creation")
}

func TestIntegration_APIUpdateKey(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant for isolation
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	// Create a key to update
	tempKey := NewTempAPIKey(t, client, tempTenant.Name).MustCreate(t)
	defer tempKey.Delete(t)

	// Update via API
	ctx := context.Background()
	err := client.UpdateAPIKey(ctx, tempTenant.Name, tempKey.ClientID, map[string]interface{}{
		"description": "Updated via API",
		"enabled":     false,
	})
	require.NoError(t, err)

	// Verify update
	key, err := client.GetAPIKey(ctx, tempTenant.Name, tempKey.ClientID)
	require.NoError(t, err)
	assert.Equal(t, "Updated via API", key.Description)
	assert.False(t, key.Enabled)
}

func TestIntegration_APIDeleteKey(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant for isolation
	tempTenant := NewTempTenant(t, client, "Keys integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	// Create a key to delete
	tempKey := NewTempAPIKey(t, client, tempTenant.Name).MustCreate(t)
	// Don't defer delete - we're testing deletion

	// Delete via API
	ctx := context.Background()
	err := client.DeleteAPIKey(ctx, tempTenant.Name, tempKey.ClientID)
	require.NoError(t, err)

	// Verify it no longer exists
	_, err = client.GetAPIKey(ctx, tempTenant.Name, tempKey.ClientID)
	require.Error(t, err, "Key should no longer exist after deletion")
	assert.Contains(t, err.Error(), "not found")
}
