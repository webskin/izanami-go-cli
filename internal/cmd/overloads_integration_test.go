//go:build integration

package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// ============================================================================
// Test Setup Helpers
// ============================================================================

// setupOverloadsTest sets up the test environment for overloads tests
func setupOverloadsTest(t *testing.T, env *IntegrationTestEnv) func() {
	t.Helper()
	env.Login(t)

	// Get JWT token from session
	token := env.GetJwtToken(t)

	// Save original values
	origCfg := cfg
	origOutputFormat := outputFormat
	origTenant := tenant
	origProject := project
	origCompactJSON := compactJSON

	// Set up config
	cfg = &izanami.ResolvedConfig{
		LeaderURL: env.LeaderURL,
		Username:  env.Username,
		JwtToken:  token,
		Timeout:   30,
	}
	outputFormat = "table"
	tenant = ""
	project = ""
	compactJSON = false

	// Reset overload-specific flags to defaults
	overloadContext = ""
	overloadEnabled = true
	overloadData = ""
	overloadPreserveProtect = false
	overloadDeleteForce = false

	return func() {
		cfg = origCfg
		outputFormat = origOutputFormat
		tenant = origTenant
		project = origProject
		compactJSON = origCompactJSON
		overloadContext = ""
		overloadEnabled = true
		overloadData = ""
		overloadPreserveProtect = false
		overloadDeleteForce = false
	}
}

// executeOverloadsCommand executes an overloads command with proper output capture
func executeOverloadsCommand(t *testing.T, args []string) (string, error) {
	t.Helper()

	// Reset global flag variables to defaults before each test
	// This prevents flag state from persisting between tests
	overloadContext = ""
	overloadEnabled = true
	overloadData = ""
	overloadPreserveProtect = false
	overloadDeleteForce = false

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.PersistentFlags().StringVar(&project, "project", "", "Default project")
	cmd.AddCommand(adminCmd)

	// Set Out/Err on ALL commands in hierarchy
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	overloadsCmd.SetOut(&buf)
	overloadsCmd.SetErr(&buf)
	overloadsSetCmd.SetOut(&buf)
	overloadsSetCmd.SetErr(&buf)
	overloadsGetCmd.SetOut(&buf)
	overloadsGetCmd.SetErr(&buf)
	overloadsDeleteCmd.SetOut(&buf)
	overloadsDeleteCmd.SetErr(&buf)

	fullArgs := append([]string{"admin", "overloads"}, args...)
	cmd.SetArgs(fullArgs)
	err := cmd.Execute()

	// Reset all commands
	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	overloadsCmd.SetOut(nil)
	overloadsCmd.SetErr(nil)
	overloadsSetCmd.SetOut(nil)
	overloadsSetCmd.SetErr(nil)
	overloadsGetCmd.SetOut(nil)
	overloadsGetCmd.SetErr(nil)
	overloadsDeleteCmd.SetOut(nil)
	overloadsDeleteCmd.SetErr(nil)

	// Reset global flag variables after execution
	overloadContext = ""
	overloadEnabled = true
	overloadData = ""
	overloadPreserveProtect = false
	overloadDeleteForce = false

	return buf.String(), err
}

// executeOverloadsCommandWithInput executes an overloads command with stdin input
func executeOverloadsCommandWithInput(t *testing.T, args []string, input string) (string, error) {
	t.Helper()

	// Reset global flag variables to defaults before each test
	overloadContext = ""
	overloadEnabled = true
	overloadData = ""
	overloadPreserveProtect = false
	overloadDeleteForce = false

	var buf bytes.Buffer
	inputBuf := bytes.NewBufferString(input)

	cmd := &cobra.Command{Use: "iz"}
	cmd.PersistentFlags().StringVar(&project, "project", "", "Default project")
	cmd.AddCommand(adminCmd)

	// Set Out/Err/In on ALL commands
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(inputBuf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminCmd.SetIn(inputBuf)
	overloadsCmd.SetOut(&buf)
	overloadsCmd.SetErr(&buf)
	overloadsCmd.SetIn(inputBuf)
	overloadsDeleteCmd.SetOut(&buf)
	overloadsDeleteCmd.SetErr(&buf)
	overloadsDeleteCmd.SetIn(inputBuf)

	fullArgs := append([]string{"admin", "overloads"}, args...)
	cmd.SetArgs(fullArgs)
	err := cmd.Execute()

	// Reset all
	adminCmd.SetIn(nil)
	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	overloadsCmd.SetIn(nil)
	overloadsCmd.SetOut(nil)
	overloadsCmd.SetErr(nil)
	overloadsDeleteCmd.SetIn(nil)
	overloadsDeleteCmd.SetOut(nil)
	overloadsDeleteCmd.SetErr(nil)

	// Reset global flag variables after execution
	overloadContext = ""
	overloadEnabled = true
	overloadData = ""
	overloadPreserveProtect = false
	overloadDeleteForce = false

	return buf.String(), err
}

// ============================================================================
// Overloads Set Tests
// ============================================================================

func TestIntegration_OverloadsSetSimple(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupOverloadsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp resources
	tempTenant := NewTempTenant(t, client, "overloads set test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "overloads test project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(false).MustCreate(t).Cleanup(t)
	tempContext := NewTempContext(t, client, tempTenant.Name).WithProject(tempProject.Name).MustCreate(t).Cleanup(t)

	// Set tenant global variable
	tenant = tempTenant.Name

	// Test: Set overload with --enabled
	output, err := executeOverloadsCommand(t, []string{
		"set", tempFeature.Name,
		"--context", tempContext.Name,
		"--project", tempProject.Name,
		"--enabled",
	})

	require.NoError(t, err, "overloads set should succeed")
	assert.Contains(t, output, "Overload set successfully")
	assert.Contains(t, output, tempFeature.Name)
	assert.Contains(t, output, tempContext.Name)

	t.Logf("Overloads set output:\n%s", output)
}

func TestIntegration_OverloadsSetWithData(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupOverloadsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp resources
	tempTenant := NewTempTenant(t, client, "overloads set data test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "overloads data test project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(false).MustCreate(t).Cleanup(t)
	tempContext := NewTempContext(t, client, tempTenant.Name).WithProject(tempProject.Name).MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name

	// Test: Set overload with JSON data (user list condition)
	// Note: resultType is required by the API
	strategyJSON := `{"enabled":true,"resultType":"boolean","conditions":[{"rule":{"type":"UserList","users":["Bob","Alice"]}}]}`
	output, err := executeOverloadsCommand(t, []string{
		"set", tempFeature.Name,
		"--context", tempContext.Name,
		"--project", tempProject.Name,
		"--data", strategyJSON,
	})

	require.NoError(t, err, "overloads set with data should succeed")
	assert.Contains(t, output, "Overload set successfully")

	t.Logf("Overloads set with data output:\n%s", output)
}

func TestIntegration_OverloadsSetMissingContext(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupOverloadsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "overloads missing context test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "overloads missing context project").MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name

	// Test: Set overload without --context should fail
	_, err := executeOverloadsCommand(t, []string{
		"set", "some-feature",
		"--project", tempProject.Name,
	})

	require.Error(t, err, "overloads set without --context should fail")
	assert.Contains(t, err.Error(), "context")
}

func TestIntegration_OverloadsSetMissingProject(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupOverloadsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "overloads missing project test").MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name

	// Test: Set overload without --project should fail
	_, err := executeOverloadsCommand(t, []string{
		"set", "some-feature",
		"--context", "PROD",
	})

	require.Error(t, err, "overloads set without --project should fail")
	assert.Contains(t, err.Error(), "project")
}

func TestIntegration_OverloadsSetMissingTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupOverloadsTest(t, env)
	defer cleanup()

	// Don't set tenant

	// Test: Set overload without tenant should fail
	_, err := executeOverloadsCommand(t, []string{
		"set", "some-feature",
		"--context", "PROD",
		"--project", "some-project",
	})

	require.Error(t, err, "overloads set without tenant should fail")
	assert.Contains(t, err.Error(), "tenant")
}

// ============================================================================
// Overloads Get Tests
// ============================================================================

func TestIntegration_OverloadsGetExisting(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupOverloadsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp resources
	tempTenant := NewTempTenant(t, client, "overloads get test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "overloads get project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(false).MustCreate(t).Cleanup(t)
	tempContext := NewTempContext(t, client, tempTenant.Name).WithProject(tempProject.Name).MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name

	// First, set an overload
	ctx := context.Background()
	strategy := map[string]interface{}{
		"enabled":    true,
		"resultType": "boolean",
	}
	err := client.SetOverload(ctx, tempTenant.Name, tempProject.Name, tempContext.Name, tempFeature.Name, strategy, false)
	require.NoError(t, err, "SetOverload should succeed")

	// Test: Get the overload
	outputFormat = "json"
	output, err := executeOverloadsCommand(t, []string{
		"get", tempFeature.Name,
		"--context", tempContext.Name,
		"--project", tempProject.Name,
	})

	require.NoError(t, err, "overloads get should succeed")
	assert.Contains(t, output, "enabled")

	t.Logf("Overloads get output:\n%s", output)
}

func TestIntegration_OverloadsGetJSONOutput(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupOverloadsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp resources
	tempTenant := NewTempTenant(t, client, "overloads get json test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "overloads get json project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(false).MustCreate(t).Cleanup(t)
	tempContext := NewTempContext(t, client, tempTenant.Name).WithProject(tempProject.Name).MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name
	outputFormat = "json"

	// First, set an overload with conditions
	ctx := context.Background()
	strategy := map[string]interface{}{
		"enabled":    true,
		"resultType": "boolean",
		"conditions": []map[string]interface{}{
			{"rule": map[string]interface{}{"type": "UserList", "users": []string{"TestUser"}}},
		},
	}
	err := client.SetOverload(ctx, tempTenant.Name, tempProject.Name, tempContext.Name, tempFeature.Name, strategy, false)
	require.NoError(t, err, "SetOverload should succeed")

	// Test: Get the overload in JSON format
	output, err := executeOverloadsCommand(t, []string{
		"get", tempFeature.Name,
		"--context", tempContext.Name,
		"--project", tempProject.Name,
	})

	require.NoError(t, err, "overloads get -o json should succeed")
	assert.Contains(t, output, "enabled")
	assert.Contains(t, output, "true")

	t.Logf("Overloads get JSON output:\n%s", output)
}

func TestIntegration_OverloadsGetNoOverloadExists(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupOverloadsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp resources (no overload created)
	tempTenant := NewTempTenant(t, client, "overloads get none test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "overloads get none project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(false).MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name

	// Test: Get overload for non-existent context should fail
	_, err := executeOverloadsCommand(t, []string{
		"get", tempFeature.Name,
		"--context", "nonexistent-context",
		"--project", tempProject.Name,
	})

	require.Error(t, err, "overloads get for non-existent context should fail")
	assert.Contains(t, err.Error(), "no overload")
}

func TestIntegration_OverloadsGetMissingContext(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupOverloadsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "overloads get missing ctx test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "overloads get missing ctx project").MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name

	// Test: Get without --context should fail
	_, err := executeOverloadsCommand(t, []string{
		"get", "some-feature",
		"--project", tempProject.Name,
	})

	require.Error(t, err, "overloads get without --context should fail")
	assert.Contains(t, err.Error(), "context")
}

// ============================================================================
// Overloads Delete Tests
// ============================================================================

func TestIntegration_OverloadsDeleteWithForce(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupOverloadsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp resources
	tempTenant := NewTempTenant(t, client, "overloads delete test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "overloads delete project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(false).MustCreate(t).Cleanup(t)
	tempContext := NewTempContext(t, client, tempTenant.Name).WithProject(tempProject.Name).MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name

	// First, set an overload
	ctx := context.Background()
	strategy := map[string]interface{}{"enabled": true, "resultType": "boolean"}
	err := client.SetOverload(ctx, tempTenant.Name, tempProject.Name, tempContext.Name, tempFeature.Name, strategy, false)
	require.NoError(t, err, "SetOverload should succeed")

	// Test: Delete with --force
	output, err := executeOverloadsCommand(t, []string{
		"delete", tempFeature.Name,
		"--context", tempContext.Name,
		"--project", tempProject.Name,
		"--force",
	})

	require.NoError(t, err, "overloads delete --force should succeed")
	assert.Contains(t, output, "Overload deleted successfully")

	t.Logf("Overloads delete output:\n%s", output)
}

func TestIntegration_OverloadsDeleteWithConfirmation(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupOverloadsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp resources
	tempTenant := NewTempTenant(t, client, "overloads delete confirm test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "overloads delete confirm project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(false).MustCreate(t).Cleanup(t)
	tempContext := NewTempContext(t, client, tempTenant.Name).WithProject(tempProject.Name).MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name

	// First, set an overload
	ctx := context.Background()
	strategy := map[string]interface{}{"enabled": true, "resultType": "boolean"}
	err := client.SetOverload(ctx, tempTenant.Name, tempProject.Name, tempContext.Name, tempFeature.Name, strategy, false)
	require.NoError(t, err, "SetOverload should succeed")

	// Test: Delete with "y" confirmation
	output, err := executeOverloadsCommandWithInput(t, []string{
		"delete", tempFeature.Name,
		"--context", tempContext.Name,
		"--project", tempProject.Name,
	}, "y\n")

	require.NoError(t, err, "overloads delete with 'y' should succeed")
	assert.Contains(t, output, "Overload deleted successfully")

	t.Logf("Overloads delete with confirmation output:\n%s", output)
}

func TestIntegration_OverloadsDeleteCancelled(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupOverloadsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp resources
	tempTenant := NewTempTenant(t, client, "overloads delete cancel test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "overloads delete cancel project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(false).MustCreate(t).Cleanup(t)
	tempContext := NewTempContext(t, client, tempTenant.Name).WithProject(tempProject.Name).MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name

	// First, set an overload
	ctx := context.Background()
	strategy := map[string]interface{}{"enabled": true, "resultType": "boolean"}
	err := client.SetOverload(ctx, tempTenant.Name, tempProject.Name, tempContext.Name, tempFeature.Name, strategy, false)
	require.NoError(t, err, "SetOverload should succeed")

	// Test: Delete with "n" cancellation
	output, err := executeOverloadsCommandWithInput(t, []string{
		"delete", tempFeature.Name,
		"--context", tempContext.Name,
		"--project", tempProject.Name,
	}, "n\n")

	require.NoError(t, err, "overloads delete with 'n' should not error")
	assert.Contains(t, output, "Cancelled")

	t.Logf("Overloads delete cancelled output:\n%s", output)
}

func TestIntegration_OverloadsDeleteMissingContext(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupOverloadsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "overloads delete missing ctx test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "overloads delete missing ctx project").MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name

	// Test: Delete without --context should fail
	_, err := executeOverloadsCommand(t, []string{
		"delete", "some-feature",
		"--project", tempProject.Name,
		"--force",
	})

	require.Error(t, err, "overloads delete without --context should fail")
	assert.Contains(t, err.Error(), "context")
}

// ============================================================================
// Direct API Tests
// ============================================================================

func TestIntegration_APISetOverload(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)
	ctx := context.Background()

	// Create temp resources
	tempTenant := NewTempTenant(t, client, "api set overload test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "api set overload project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(false).MustCreate(t).Cleanup(t)
	tempContext := NewTempContext(t, client, tempTenant.Name).WithProject(tempProject.Name).MustCreate(t).Cleanup(t)

	// Test: Set overload via API
	strategy := map[string]interface{}{
		"enabled":    true,
		"resultType": "boolean",
		"conditions": []map[string]interface{}{
			{"rule": map[string]interface{}{"type": "UserList", "users": []string{"Bob"}}},
		},
	}

	err := client.SetOverload(ctx, tempTenant.Name, tempProject.Name, tempContext.Name, tempFeature.Name, strategy, false)
	require.NoError(t, err, "SetOverload should succeed")

	t.Logf("API SetOverload succeeded for feature %s in context %s", tempFeature.Name, tempContext.Name)
}

func TestIntegration_APIGetOverload(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)
	ctx := context.Background()

	// Create temp resources
	tempTenant := NewTempTenant(t, client, "api get overload test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "api get overload project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(false).MustCreate(t).Cleanup(t)
	tempContext := NewTempContext(t, client, tempTenant.Name).WithProject(tempProject.Name).MustCreate(t).Cleanup(t)

	// Set an overload first
	strategy := map[string]interface{}{
		"enabled":    true,
		"resultType": "boolean",
	}
	err := client.SetOverload(ctx, tempTenant.Name, tempProject.Name, tempContext.Name, tempFeature.Name, strategy, false)
	require.NoError(t, err, "SetOverload should succeed")

	// Test: Get overload via API
	raw, err := izanami.GetOverload(client, ctx, tempTenant.Name, tempProject.Name, tempFeature.Name, tempContext.Name, izanami.Identity)
	require.NoError(t, err, "GetOverload should succeed")
	require.NotEmpty(t, raw, "Overload data should not be empty")

	t.Logf("API GetOverload returned: %s", string(raw))
}

func TestIntegration_APIDeleteOverload(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)
	ctx := context.Background()

	// Create temp resources
	tempTenant := NewTempTenant(t, client, "api delete overload test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "api delete overload project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(false).MustCreate(t).Cleanup(t)
	tempContext := NewTempContext(t, client, tempTenant.Name).WithProject(tempProject.Name).MustCreate(t).Cleanup(t)

	// Set an overload first
	strategy := map[string]interface{}{
		"enabled":    true,
		"resultType": "boolean",
	}
	err := client.SetOverload(ctx, tempTenant.Name, tempProject.Name, tempContext.Name, tempFeature.Name, strategy, false)
	require.NoError(t, err, "SetOverload should succeed")

	// Test: Delete overload via API
	err = client.DeleteOverload(ctx, tempTenant.Name, tempProject.Name, tempContext.Name, tempFeature.Name, false)
	require.NoError(t, err, "DeleteOverload should succeed")

	// Verify it's gone
	_, err = izanami.GetOverload(client, ctx, tempTenant.Name, tempProject.Name, tempFeature.Name, tempContext.Name, izanami.Identity)
	require.Error(t, err, "GetOverload should fail after deletion")
	assert.Contains(t, err.Error(), "no overload")

	t.Logf("API DeleteOverload succeeded for feature %s in context %s", tempFeature.Name, tempContext.Name)
}

func TestIntegration_OverloadsSetNestedContext(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupOverloadsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp resources with nested context
	tempTenant := NewTempTenant(t, client, "overloads nested context test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "overloads nested project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(false).MustCreate(t).Cleanup(t)

	// Create parent context
	parentCtx := NewTempContext(t, client, tempTenant.Name).WithName("PROD").WithProject(tempProject.Name).MustCreate(t).Cleanup(t)

	// Create child context
	childCtx := NewTempContext(t, client, tempTenant.Name).WithName("mobile").WithParent("PROD").WithProject(tempProject.Name).MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name

	// Test: Set overload on nested context path (PROD/mobile)
	output, err := executeOverloadsCommand(t, []string{
		"set", tempFeature.Name,
		"--context", childCtx.Path, // "PROD/mobile"
		"--project", tempProject.Name,
		"--enabled",
	})

	require.NoError(t, err, "overloads set on nested context should succeed")
	assert.Contains(t, output, "Overload set successfully")
	assert.Contains(t, output, parentCtx.Name+"/"+childCtx.Name)

	t.Logf("Overloads set nested context output:\n%s", output)
}
