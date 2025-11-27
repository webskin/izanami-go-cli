package cmd

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// setupTenantsTest sets up the global cfg for tenant command tests
func setupTenantsTest(t *testing.T, env *IntegrationTestEnv) func() {
	t.Helper()

	// Get JWT token from session
	token := env.GetJwtToken(t)

	// Save original values
	origCfg := cfg
	origOutputFormat := outputFormat
	origTenantDesc := tenantDesc
	origTenantData := tenantData
	origDeleteForce := tenantsDeleteForce
	origLogsOrder := logsOrder
	origLogsUsers := logsUsers
	origLogsTypes := logsTypes
	origLogsFeatures := logsFeatures
	origLogsProjects := logsProjects
	origLogsStart := logsStart
	origLogsEnd := logsEnd
	origLogsCursor := logsCursor
	origLogsCount := logsCount
	origLogsTotal := logsTotal

	// Set up config
	cfg = &izanami.Config{
		BaseURL:  env.BaseURL,
		Username: env.Username,
		JwtToken: token,
		Timeout:  30,
	}
	outputFormat = "table"
	tenantDesc = ""
	tenantData = ""
	tenantsDeleteForce = false
	logsOrder = ""
	logsUsers = ""
	logsTypes = ""
	logsFeatures = ""
	logsProjects = ""
	logsStart = ""
	logsEnd = ""
	logsCursor = 0
	logsCount = 50
	logsTotal = false

	return func() {
		cfg = origCfg
		outputFormat = origOutputFormat
		tenantDesc = origTenantDesc
		tenantData = origTenantData
		tenantsDeleteForce = origDeleteForce
		logsOrder = origLogsOrder
		logsUsers = origLogsUsers
		logsTypes = origLogsTypes
		logsFeatures = origLogsFeatures
		logsProjects = origLogsProjects
		logsStart = origLogsStart
		logsEnd = origLogsEnd
		logsCursor = origLogsCursor
		logsCount = origLogsCount
		logsTotal = origLogsTotal
	}
}

// ============================================================================
// TENANTS LIST
// ============================================================================

func TestIntegration_TenantsListAfterLogin(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupTenantsTest(t, env)
	defer cleanup()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminTenantsCmd.SetOut(&buf)
	adminTenantsCmd.SetErr(&buf)
	adminTenantsListCmd.SetOut(&buf)
	adminTenantsListCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "tenants", "list"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminTenantsCmd.SetOut(nil)
	adminTenantsCmd.SetErr(nil)
	adminTenantsListCmd.SetOut(nil)
	adminTenantsListCmd.SetErr(nil)

	require.NoError(t, err, "Tenants list should succeed")

	// Verify via API that we have tenants
	client := env.NewAuthenticatedClient(t)
	ctx := context.Background()
	tenants, err := izanami.ListTenants(client, ctx, nil, izanami.ParseTenants)
	require.NoError(t, err)

	t.Logf("Listed %d tenants via API", len(tenants))
}

func TestIntegration_TenantsListJSONOutput(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupTenantsTest(t, env)
	defer cleanup()

	// Set JSON output format
	outputFormat = "json"

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminTenantsCmd.SetOut(&buf)
	adminTenantsCmd.SetErr(&buf)
	adminTenantsListCmd.SetOut(&buf)
	adminTenantsListCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "tenants", "list"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminTenantsCmd.SetOut(nil)
	adminTenantsCmd.SetErr(nil)
	adminTenantsListCmd.SetOut(nil)
	adminTenantsListCmd.SetErr(nil)

	require.NoError(t, err, "Tenants list JSON should succeed")
	output := buf.String()

	// Should be valid JSON array
	assert.True(t, strings.HasPrefix(strings.TrimSpace(output), "["), "JSON output should start with [")

	t.Logf("Tenants list JSON output length: %d chars", len(output))
}

func TestIntegration_TenantsListVerifyViaAPI(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupTenantsTest(t, env)
	defer cleanup()

	// Set JSON output to capture structured data
	outputFormat = "json"

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminTenantsCmd.SetOut(&buf)
	adminTenantsCmd.SetErr(&buf)
	adminTenantsListCmd.SetOut(&buf)
	adminTenantsListCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "tenants", "list"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminTenantsCmd.SetOut(nil)
	adminTenantsCmd.SetErr(nil)
	adminTenantsListCmd.SetOut(nil)
	adminTenantsListCmd.SetErr(nil)

	require.NoError(t, err)
	cliOutput := buf.String()

	// Verify via API
	client := env.NewAuthenticatedClient(t)
	ctx := context.Background()
	tenants, err := izanami.ListTenants(client, ctx, nil, izanami.ParseTenants)
	require.NoError(t, err, "API list tenants should succeed")

	// Verify CLI JSON output contains tenant names from API
	for _, tenant := range tenants {
		assert.Contains(t, cliOutput, tenant.Name, "CLI JSON output should contain tenant: %s", tenant.Name)
	}

	t.Logf("Verified %d tenants from API appear in CLI JSON output", len(tenants))
}

// ============================================================================
// TENANTS GET
// ============================================================================

func TestIntegration_TenantsGetExisting(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupTenantsTest(t, env)
	defer cleanup()

	// First get a tenant name from the list
	client := env.NewAuthenticatedClient(t)
	ctx := context.Background()
	tenants, err := izanami.ListTenants(client, ctx, nil, izanami.ParseTenants)
	require.NoError(t, err)
	require.NotEmpty(t, tenants, "Need at least one tenant for this test")

	tenantName := tenants[0].Name

	// Set JSON output for reliable assertion
	outputFormat = "json"

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminTenantsCmd.SetOut(&buf)
	adminTenantsCmd.SetErr(&buf)
	adminTenantsGetCmd.SetOut(&buf)
	adminTenantsGetCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "tenants", "get", tenantName})
	err = cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminTenantsCmd.SetOut(nil)
	adminTenantsCmd.SetErr(nil)
	adminTenantsGetCmd.SetOut(nil)
	adminTenantsGetCmd.SetErr(nil)

	require.NoError(t, err, "Tenants get should succeed")
	output := buf.String()

	// Should display tenant name in JSON
	assert.Contains(t, output, tenantName, "Output should contain tenant name")
	assert.True(t, strings.HasPrefix(strings.TrimSpace(output), "{"), "JSON output should start with {")

	t.Logf("Tenants get output for '%s': %d chars", tenantName, len(output))
}

func TestIntegration_TenantsGetNonExistent(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupTenantsTest(t, env)
	defer cleanup()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminTenantsCmd.SetOut(&buf)
	adminTenantsCmd.SetErr(&buf)
	adminTenantsGetCmd.SetOut(&buf)
	adminTenantsGetCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "tenants", "get", "non-existent-tenant-12345"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminTenantsCmd.SetOut(nil)
	adminTenantsCmd.SetErr(nil)
	adminTenantsGetCmd.SetOut(nil)
	adminTenantsGetCmd.SetErr(nil)

	require.Error(t, err, "Getting non-existent tenant should fail")

	t.Logf("Expected error for non-existent tenant: %v", err)
}

func TestIntegration_TenantsGetMissingArg(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupTenantsTest(t, env)
	defer cleanup()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminTenantsCmd.SetOut(&buf)
	adminTenantsCmd.SetErr(&buf)
	adminTenantsGetCmd.SetOut(&buf)
	adminTenantsGetCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "tenants", "get"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminTenantsCmd.SetOut(nil)
	adminTenantsCmd.SetErr(nil)
	adminTenantsGetCmd.SetOut(nil)
	adminTenantsGetCmd.SetErr(nil)

	require.Error(t, err, "Get without tenant name should fail")
	assert.Contains(t, err.Error(), "accepts 1 arg", "Error should mention argument requirement")

	t.Logf("Expected error for missing arg: %v", err)
}

// ============================================================================
// TENANTS CREATE
// ============================================================================

func TestIntegration_TenantsCreateAndDelete(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupTenantsTest(t, env)
	defer cleanup()

	// Generate unique tenant name
	tenantName := fmt.Sprintf("test-tenant-%d", time.Now().UnixNano())
	tenantDescription := "Integration test tenant"

	// Set description flag
	tenantDesc = tenantDescription

	// Create tenant
	var createBuf bytes.Buffer
	createCmd := &cobra.Command{Use: "iz"}
	createCmd.AddCommand(adminCmd)
	createCmd.SetOut(&createBuf)
	createCmd.SetErr(&createBuf)
	adminCmd.SetOut(&createBuf)
	adminCmd.SetErr(&createBuf)
	adminTenantsCmd.SetOut(&createBuf)
	adminTenantsCmd.SetErr(&createBuf)
	adminTenantsCreateCmd.SetOut(&createBuf)
	adminTenantsCreateCmd.SetErr(&createBuf)

	createCmd.SetArgs([]string{"admin", "tenants", "create", tenantName})
	err := createCmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminTenantsCmd.SetOut(nil)
	adminTenantsCmd.SetErr(nil)
	adminTenantsCreateCmd.SetOut(nil)
	adminTenantsCreateCmd.SetErr(nil)

	require.NoError(t, err, "Tenant create should succeed")
	createOutput := createBuf.String()
	assert.Contains(t, createOutput, "created successfully", "Should confirm creation")
	assert.Contains(t, createOutput, tenantName, "Should mention tenant name")

	t.Logf("Tenant created: %s", tenantName)

	// Verify via API
	client := env.NewAuthenticatedClient(t)
	ctx := context.Background()
	tenant, err := izanami.GetTenant(client, ctx, tenantName, izanami.ParseTenant)
	require.NoError(t, err, "Should be able to get created tenant via API")
	assert.Equal(t, tenantName, tenant.Name, "Tenant name should match")
	assert.Equal(t, tenantDescription, tenant.Description, "Tenant description should match")

	// Cleanup: Delete tenant
	tenantsDeleteForce = true

	var deleteBuf bytes.Buffer
	deleteCmd := &cobra.Command{Use: "iz"}
	deleteCmd.AddCommand(adminCmd)
	deleteCmd.SetOut(&deleteBuf)
	deleteCmd.SetErr(&deleteBuf)
	adminCmd.SetOut(&deleteBuf)
	adminCmd.SetErr(&deleteBuf)
	adminTenantsCmd.SetOut(&deleteBuf)
	adminTenantsCmd.SetErr(&deleteBuf)
	adminTenantsDeleteCmd.SetOut(&deleteBuf)
	adminTenantsDeleteCmd.SetErr(&deleteBuf)

	deleteCmd.SetArgs([]string{"admin", "tenants", "delete", tenantName})
	err = deleteCmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminTenantsCmd.SetOut(nil)
	adminTenantsCmd.SetErr(nil)
	adminTenantsDeleteCmd.SetOut(nil)
	adminTenantsDeleteCmd.SetErr(nil)

	require.NoError(t, err, "Tenant delete should succeed")
	deleteOutput := deleteBuf.String()
	assert.Contains(t, deleteOutput, "deleted successfully", "Should confirm deletion")

	t.Logf("Tenant deleted: %s", tenantName)
}

func TestIntegration_TenantsCreateWithJSONData(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupTenantsTest(t, env)
	defer cleanup()

	// Generate unique tenant name
	tenantName := fmt.Sprintf("test-tenant-json-%d", time.Now().UnixNano())
	jsonData := fmt.Sprintf(`{"name":"%s","description":"Created with JSON data"}`, tenantName)

	// Set data flag
	tenantData = jsonData

	// Create tenant with JSON data
	var createBuf bytes.Buffer
	createCmd := &cobra.Command{Use: "iz"}
	createCmd.AddCommand(adminCmd)
	createCmd.SetOut(&createBuf)
	createCmd.SetErr(&createBuf)
	adminCmd.SetOut(&createBuf)
	adminCmd.SetErr(&createBuf)
	adminTenantsCmd.SetOut(&createBuf)
	adminTenantsCmd.SetErr(&createBuf)
	adminTenantsCreateCmd.SetOut(&createBuf)
	adminTenantsCreateCmd.SetErr(&createBuf)

	// Mark the flag as changed
	adminTenantsCreateCmd.Flags().Set("data", jsonData)

	createCmd.SetArgs([]string{"admin", "tenants", "create", tenantName})
	err := createCmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminTenantsCmd.SetOut(nil)
	adminTenantsCmd.SetErr(nil)
	adminTenantsCreateCmd.SetOut(nil)
	adminTenantsCreateCmd.SetErr(nil)

	require.NoError(t, err, "Tenant create with JSON should succeed")

	// Verify via API
	client := env.NewAuthenticatedClient(t)
	ctx := context.Background()
	tenant, err := izanami.GetTenant(client, ctx, tenantName, izanami.ParseTenant)
	require.NoError(t, err, "Should be able to get created tenant via API")
	assert.Equal(t, "Created with JSON data", tenant.Description, "Description from JSON should match")

	t.Logf("Tenant created with JSON data: %s", tenantName)

	// Cleanup
	err = client.DeleteTenant(ctx, tenantName)
	require.NoError(t, err, "Cleanup delete should succeed")
}

func TestIntegration_TenantsCreateDuplicate(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupTenantsTest(t, env)
	defer cleanup()

	// Generate unique tenant name
	tenantName := fmt.Sprintf("test-tenant-dup-%d", time.Now().UnixNano())

	// Create tenant first time via API
	client := env.NewAuthenticatedClient(t)
	ctx := context.Background()
	err := client.CreateTenant(ctx, map[string]interface{}{
		"name":        tenantName,
		"description": "First creation",
	})
	require.NoError(t, err, "First creation should succeed")

	// Set description for CLI create
	tenantDesc = "Duplicate"

	// Try to create duplicate via CLI
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminTenantsCmd.SetOut(&buf)
	adminTenantsCmd.SetErr(&buf)
	adminTenantsCreateCmd.SetOut(&buf)
	adminTenantsCreateCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "tenants", "create", tenantName})
	err = cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminTenantsCmd.SetOut(nil)
	adminTenantsCmd.SetErr(nil)
	adminTenantsCreateCmd.SetOut(nil)
	adminTenantsCreateCmd.SetErr(nil)

	require.Error(t, err, "Creating duplicate tenant should fail")

	t.Logf("Expected error for duplicate tenant: %v", err)

	// Cleanup
	err = client.DeleteTenant(ctx, tenantName)
	require.NoError(t, err, "Cleanup delete should succeed")
}

// ============================================================================
// TENANTS UPDATE
// ============================================================================

func TestIntegration_TenantsUpdateDescription(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupTenantsTest(t, env)
	defer cleanup()

	// Generate unique tenant name
	tenantName := fmt.Sprintf("test-tenant-upd-%d", time.Now().UnixNano())
	originalDesc := "Original description"
	updatedDesc := "Updated description"

	// Create tenant first via API
	client := env.NewAuthenticatedClient(t)
	ctx := context.Background()
	err := client.CreateTenant(ctx, map[string]interface{}{
		"name":        tenantName,
		"description": originalDesc,
	})
	require.NoError(t, err, "Tenant creation should succeed")

	// Set description flag for update
	tenantDesc = updatedDesc

	// Mark the flag as changed
	adminTenantsUpdateCmd.Flags().Set("description", updatedDesc)

	// Update tenant
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminTenantsCmd.SetOut(&buf)
	adminTenantsCmd.SetErr(&buf)
	adminTenantsUpdateCmd.SetOut(&buf)
	adminTenantsUpdateCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "tenants", "update", tenantName})
	err = cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminTenantsCmd.SetOut(nil)
	adminTenantsCmd.SetErr(nil)
	adminTenantsUpdateCmd.SetOut(nil)
	adminTenantsUpdateCmd.SetErr(nil)

	require.NoError(t, err, "Tenant update should succeed")
	output := buf.String()
	assert.Contains(t, output, "updated successfully", "Should confirm update")

	// Verify via API
	tenant, err := izanami.GetTenant(client, ctx, tenantName, izanami.ParseTenant)
	require.NoError(t, err, "Should be able to get updated tenant")
	assert.Equal(t, updatedDesc, tenant.Description, "Description should be updated")

	t.Logf("Tenant updated: %s (description: '%s' -> '%s')", tenantName, originalDesc, updatedDesc)

	// Cleanup
	err = client.DeleteTenant(ctx, tenantName)
	require.NoError(t, err, "Cleanup delete should succeed")
}

func TestIntegration_TenantsUpdateViaAPI(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupTenantsTest(t, env)
	defer cleanup()

	// Generate unique tenant name
	tenantName := fmt.Sprintf("test-tenant-updapi-%d", time.Now().UnixNano())

	// Create tenant first via API
	client := env.NewAuthenticatedClient(t)
	ctx := context.Background()
	err := client.CreateTenant(ctx, map[string]interface{}{
		"name":        tenantName,
		"description": "Original",
	})
	require.NoError(t, err, "Tenant creation should succeed")

	// Update via API directly (testing the underlying client method)
	err = client.UpdateTenant(ctx, tenantName, map[string]interface{}{
		"name":        tenantName,
		"description": "Updated via API",
	})
	require.NoError(t, err, "Tenant update via API should succeed")

	// Verify via API
	tenant, err := izanami.GetTenant(client, ctx, tenantName, izanami.ParseTenant)
	require.NoError(t, err, "Should be able to get updated tenant")
	assert.Equal(t, "Updated via API", tenant.Description, "Description should be updated")

	t.Logf("Tenant updated via API: %s", tenantName)

	// Cleanup
	err = client.DeleteTenant(ctx, tenantName)
	require.NoError(t, err, "Cleanup delete should succeed")
}

// ============================================================================
// TENANTS DELETE
// ============================================================================

func TestIntegration_TenantsDeleteWithForce(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupTenantsTest(t, env)
	defer cleanup()

	// Generate unique tenant name
	tenantName := fmt.Sprintf("test-tenant-del-%d", time.Now().UnixNano())

	// Create tenant first via API
	client := env.NewAuthenticatedClient(t)
	ctx := context.Background()
	err := client.CreateTenant(ctx, map[string]interface{}{
		"name":        tenantName,
		"description": "To be deleted",
	})
	require.NoError(t, err, "Tenant creation should succeed")

	// Set force flag
	tenantsDeleteForce = true

	// Delete with force flag
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminTenantsCmd.SetOut(&buf)
	adminTenantsCmd.SetErr(&buf)
	adminTenantsDeleteCmd.SetOut(&buf)
	adminTenantsDeleteCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "tenants", "delete", tenantName})
	err = cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminTenantsCmd.SetOut(nil)
	adminTenantsCmd.SetErr(nil)
	adminTenantsDeleteCmd.SetOut(nil)
	adminTenantsDeleteCmd.SetErr(nil)

	require.NoError(t, err, "Tenant delete should succeed")
	output := buf.String()
	assert.Contains(t, output, "deleted successfully", "Should confirm deletion")

	// Verify tenant no longer exists
	_, err = izanami.GetTenant(client, ctx, tenantName, izanami.ParseTenant)
	require.Error(t, err, "Tenant should no longer exist")

	t.Logf("Tenant deleted: %s", tenantName)
}

func TestIntegration_TenantsDeleteWithConfirmation(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupTenantsTest(t, env)
	defer cleanup()

	// Generate unique tenant name
	tenantName := fmt.Sprintf("test-tenant-delconf-%d", time.Now().UnixNano())

	// Create tenant first via API
	client := env.NewAuthenticatedClient(t)
	ctx := context.Background()
	err := client.CreateTenant(ctx, map[string]interface{}{
		"name":        tenantName,
		"description": "To be deleted with confirmation",
	})
	require.NoError(t, err, "Tenant creation should succeed")

	// Force flag is false (default from cleanup setup)
	tenantsDeleteForce = false

	// Delete with confirmation input "y"
	var buf bytes.Buffer
	input := bytes.NewBufferString("y\n")
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(input)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminCmd.SetIn(input)
	adminTenantsCmd.SetOut(&buf)
	adminTenantsCmd.SetErr(&buf)
	adminTenantsCmd.SetIn(input)
	adminTenantsDeleteCmd.SetOut(&buf)
	adminTenantsDeleteCmd.SetErr(&buf)
	adminTenantsDeleteCmd.SetIn(input)

	cmd.SetArgs([]string{"admin", "tenants", "delete", tenantName})
	err = cmd.Execute()

	adminCmd.SetIn(nil)
	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminTenantsCmd.SetIn(nil)
	adminTenantsCmd.SetOut(nil)
	adminTenantsCmd.SetErr(nil)
	adminTenantsDeleteCmd.SetIn(nil)
	adminTenantsDeleteCmd.SetOut(nil)
	adminTenantsDeleteCmd.SetErr(nil)

	require.NoError(t, err, "Tenant delete with confirmation should succeed")
	output := buf.String()
	assert.Contains(t, output, "deleted successfully", "Should confirm deletion")

	// Verify tenant no longer exists
	_, err = izanami.GetTenant(client, ctx, tenantName, izanami.ParseTenant)
	require.Error(t, err, "Tenant should no longer exist")

	t.Logf("Tenant deleted with confirmation: %s", tenantName)
}

func TestIntegration_TenantsDeleteCancelled(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupTenantsTest(t, env)
	defer cleanup()

	// Generate unique tenant name
	tenantName := fmt.Sprintf("test-tenant-delcan-%d", time.Now().UnixNano())

	// Create tenant first via API
	client := env.NewAuthenticatedClient(t)
	ctx := context.Background()
	err := client.CreateTenant(ctx, map[string]interface{}{
		"name":        tenantName,
		"description": "Should not be deleted",
	})
	require.NoError(t, err, "Tenant creation should succeed")

	// Force flag is false
	tenantsDeleteForce = false

	// Try to delete with "n" confirmation
	var buf bytes.Buffer
	input := bytes.NewBufferString("n\n")
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(input)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminCmd.SetIn(input)
	adminTenantsCmd.SetOut(&buf)
	adminTenantsCmd.SetErr(&buf)
	adminTenantsCmd.SetIn(input)
	adminTenantsDeleteCmd.SetOut(&buf)
	adminTenantsDeleteCmd.SetErr(&buf)
	adminTenantsDeleteCmd.SetIn(input)

	cmd.SetArgs([]string{"admin", "tenants", "delete", tenantName})
	err = cmd.Execute()

	adminCmd.SetIn(nil)
	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminTenantsCmd.SetIn(nil)
	adminTenantsCmd.SetOut(nil)
	adminTenantsCmd.SetErr(nil)
	adminTenantsDeleteCmd.SetIn(nil)
	adminTenantsDeleteCmd.SetOut(nil)
	adminTenantsDeleteCmd.SetErr(nil)

	require.NoError(t, err, "Cancelled delete should not error")

	// Verify tenant still exists
	tenant, err := izanami.GetTenant(client, ctx, tenantName, izanami.ParseTenant)
	require.NoError(t, err, "Tenant should still exist")
	assert.Equal(t, tenantName, tenant.Name)

	t.Logf("Tenant deletion cancelled: %s still exists", tenantName)

	// Cleanup
	err = client.DeleteTenant(ctx, tenantName)
	require.NoError(t, err, "Cleanup delete should succeed")
}

func TestIntegration_TenantsDeleteNonExistent(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupTenantsTest(t, env)
	defer cleanup()

	// Set force flag
	tenantsDeleteForce = true

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminTenantsCmd.SetOut(&buf)
	adminTenantsCmd.SetErr(&buf)
	adminTenantsDeleteCmd.SetOut(&buf)
	adminTenantsDeleteCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "tenants", "delete", "non-existent-tenant-99999"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminTenantsCmd.SetOut(nil)
	adminTenantsCmd.SetErr(nil)
	adminTenantsDeleteCmd.SetOut(nil)
	adminTenantsDeleteCmd.SetErr(nil)

	// Delete of non-existent may succeed (idempotent) or fail depending on server
	// Either is acceptable behavior
	t.Logf("Delete non-existent tenant result: err=%v", err)
}

// ============================================================================
// TENANTS LOGS
// ============================================================================

func TestIntegration_TenantsLogsViaAPI(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupTenantsTest(t, env)
	defer cleanup()

	// Get an existing tenant for the logs
	client := env.NewAuthenticatedClient(t)
	ctx := context.Background()
	tenants, err := izanami.ListTenants(client, ctx, nil, izanami.ParseTenants)
	require.NoError(t, err)
	require.NotEmpty(t, tenants, "Need at least one tenant for this test")

	tenantName := tenants[0].Name

	// Test the logs API directly
	opts := &izanami.LogsRequest{
		Count: 10,
	}
	logs, err := izanami.ListTenantLogs(client, ctx, tenantName, opts, izanami.ParseLogsResponse)
	require.NoError(t, err, "Listing tenant logs via API should succeed")

	t.Logf("Retrieved %d log events for tenant '%s'", len(logs.Events), tenantName)
}

func TestIntegration_TenantsLogsViaAPIWithFilters(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupTenantsTest(t, env)
	defer cleanup()

	// Get an existing tenant
	client := env.NewAuthenticatedClient(t)
	ctx := context.Background()
	tenants, err := izanami.ListTenants(client, ctx, nil, izanami.ParseTenants)
	require.NoError(t, err)
	require.NotEmpty(t, tenants, "Need at least one tenant for this test")

	tenantName := tenants[0].Name

	// Test with various filter options
	opts := &izanami.LogsRequest{
		Order: "desc",
		Count: 5,
	}
	logs, err := izanami.ListTenantLogs(client, ctx, tenantName, opts, izanami.ParseLogsResponse)
	require.NoError(t, err, "Listing tenant logs with filters via API should succeed")

	t.Logf("Retrieved %d log events with filters for tenant '%s'", len(logs.Events), tenantName)
}

func TestIntegration_TenantsLogsRawJSON(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupTenantsTest(t, env)
	defer cleanup()

	// Get an existing tenant
	client := env.NewAuthenticatedClient(t)
	ctx := context.Background()
	tenants, err := izanami.ListTenants(client, ctx, nil, izanami.ParseTenants)
	require.NoError(t, err)
	require.NotEmpty(t, tenants, "Need at least one tenant for this test")

	tenantName := tenants[0].Name

	// Test raw JSON output using Identity mapper
	opts := &izanami.LogsRequest{
		Count: 5,
	}
	raw, err := izanami.ListTenantLogs(client, ctx, tenantName, opts, izanami.Identity)
	require.NoError(t, err, "Listing tenant logs as raw JSON should succeed")

	// Verify it's valid JSON
	trimmed := strings.TrimSpace(string(raw))
	assert.True(t, strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "["),
		"Raw output should be valid JSON")

	t.Logf("Retrieved raw JSON logs for tenant '%s' (%d bytes)", tenantName, len(raw))
}

func TestIntegration_TenantsLogsMissingTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)
	cleanup := setupTenantsTest(t, env)
	defer cleanup()

	// Make sure cfg.Tenant is empty
	cfg.Tenant = ""

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminTenantsCmd.SetOut(&buf)
	adminTenantsCmd.SetErr(&buf)
	adminTenantsLogsCmd.SetOut(&buf)
	adminTenantsLogsCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "tenants", "logs"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminTenantsCmd.SetOut(nil)
	adminTenantsCmd.SetErr(nil)
	adminTenantsLogsCmd.SetOut(nil)
	adminTenantsLogsCmd.SetErr(nil)

	require.Error(t, err, "Logs without tenant should fail")

	t.Logf("Expected error for missing tenant: %v", err)
}

// ============================================================================
// AUTH ERROR CASES
// ============================================================================

func TestIntegration_TenantsListWithoutLogin(t *testing.T) {
	_ = setupIntegrationTest(t) // No login

	// Save and clear cfg
	origCfg := cfg
	cfg = nil
	defer func() { cfg = origCfg }()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminTenantsCmd.SetOut(&buf)
	adminTenantsCmd.SetErr(&buf)
	adminTenantsListCmd.SetOut(&buf)
	adminTenantsListCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "tenants", "list"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminTenantsCmd.SetOut(nil)
	adminTenantsCmd.SetErr(nil)
	adminTenantsListCmd.SetOut(nil)
	adminTenantsListCmd.SetErr(nil)

	require.Error(t, err, "Tenants list without login should fail")

	t.Logf("Expected error without login: %v", err)
}
