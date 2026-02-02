package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// Note: TempAPIKey is defined in keys_integration_test.go

// ============================================================================
// Test Setup Helpers for Features Check Commands
// ============================================================================

// setupFeaturesCheckTest sets up the test environment for features check tests
// and returns a cleanup function
func setupFeaturesCheckTest(t *testing.T, env *IntegrationTestEnv) func() {
	t.Helper()
	env.Login(t)
	token := env.GetJwtToken(t)

	// Save original values
	origCfg := cfg
	origOutputFormat := outputFormat
	origTenant := tenant
	origFeatureUser := featureUser
	origFeatureContextStr := featureContextStr
	origFeatureData := featureData
	origCheckClientID := checkClientID
	origCheckClientSecret := checkClientSecret
	origCheckFeatures := checkFeatures
	origCheckProjects := checkProjects
	origCheckConditions := checkConditions
	origCheckDate := checkDate
	origCheckOneTagIn := checkOneTagIn
	origCheckAllTagsIn := checkAllTagsIn
	origCheckNoTagIn := checkNoTagIn

	// Set up config
	cfg = &izanami.ResolvedConfig{
		LeaderURL: env.LeaderURL,
		Username:  env.Username,
		JwtToken:  token,
		Timeout:   30,
	}
	outputFormat = "table"
	tenant = ""

	// Reset command-specific flags to defaults
	featureUser = ""
	featureContextStr = ""
	featureData = ""
	checkClientID = ""
	checkClientSecret = ""
	checkFeatures = []string{}
	checkProjects = []string{}
	checkConditions = false
	checkDate = ""
	checkOneTagIn = []string{}
	checkAllTagsIn = []string{}
	checkNoTagIn = []string{}

	return func() {
		cfg = origCfg
		outputFormat = origOutputFormat
		tenant = origTenant
		featureUser = origFeatureUser
		featureContextStr = origFeatureContextStr
		featureData = origFeatureData
		checkClientID = origCheckClientID
		checkClientSecret = origCheckClientSecret
		checkFeatures = origCheckFeatures
		checkProjects = origCheckProjects
		checkConditions = origCheckConditions
		checkDate = origCheckDate
		checkOneTagIn = origCheckOneTagIn
		checkAllTagsIn = origCheckAllTagsIn
		checkNoTagIn = origCheckNoTagIn
	}
}

// executeFeaturesCheckCommand executes 'iz features check' command with proper output capture
func executeFeaturesCheckCommand(t *testing.T, args []string) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(rootFeaturesCmd)

	// Set Out/Err on ALL commands in hierarchy
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	rootFeaturesCmd.SetOut(&buf)
	rootFeaturesCmd.SetErr(&buf)
	featuresCheckCmd.SetOut(&buf)
	featuresCheckCmd.SetErr(&buf)

	fullArgs := append([]string{"features", "check"}, args...)
	cmd.SetArgs(fullArgs)
	err := cmd.Execute()

	// Reset all commands
	rootFeaturesCmd.SetOut(nil)
	rootFeaturesCmd.SetErr(nil)
	featuresCheckCmd.SetOut(nil)
	featuresCheckCmd.SetErr(nil)

	return buf.String(), err
}

// executeFeaturesCheckBulkCommand executes 'iz features check-bulk' command with proper output capture
func executeFeaturesCheckBulkCommand(t *testing.T, args []string) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(rootFeaturesCmd)

	// Set Out/Err on ALL commands in hierarchy
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	rootFeaturesCmd.SetOut(&buf)
	rootFeaturesCmd.SetErr(&buf)
	featuresCheckBulkCmd.SetOut(&buf)
	featuresCheckBulkCmd.SetErr(&buf)

	fullArgs := append([]string{"features", "check-bulk"}, args...)
	cmd.SetArgs(fullArgs)
	err := cmd.Execute()

	// Reset all commands
	rootFeaturesCmd.SetOut(nil)
	rootFeaturesCmd.SetErr(nil)
	featuresCheckBulkCmd.SetOut(nil)
	featuresCheckBulkCmd.SetErr(nil)

	return buf.String(), err
}

// ============================================================================
// Features Check Tests (Single Feature)
// ============================================================================

func TestIntegration_FeaturesCheckByUUID(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesCheckTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp tenant, project, and feature
	tempTenant := NewTempTenant(t, client, "features check by UUID test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features check project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)

	// Create a temporary API key with project access for the temp tenant
	// (env credentials won't work for a newly created temp tenant)
	tempAPIKey := NewTempAPIKey(t, client, tempTenant.Name).
		WithProjects([]string{tempProject.Name}).
		MustCreate(t).Cleanup(t)
	checkClientID = tempAPIKey.ClientID
	checkClientSecret = tempAPIKey.ClientSecret

	// Check feature by UUID
	featureUser = "test-user"
	output, err := executeFeaturesCheckCommand(t, []string{tempFeature.ID})

	require.NoError(t, err, "features check by UUID should succeed")
	assert.Contains(t, output, tempFeature.Name, "Output should contain feature name")

	t.Logf("Features check by UUID output:\n%s", output)
}

func TestIntegration_FeaturesCheckByName(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesCheckTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp tenant, project, and feature
	tempTenant := NewTempTenant(t, client, "features check by name test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features check project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)

	// Set tenant (required for name resolution)
	tenant = tempTenant.Name
	cfg.Tenant = tempTenant.Name

	// Create a temporary API key with project access for the temp tenant
	tempAPIKey := NewTempAPIKey(t, client, tempTenant.Name).
		WithProjects([]string{tempProject.Name}).
		MustCreate(t).Cleanup(t)
	checkClientID = tempAPIKey.ClientID
	checkClientSecret = tempAPIKey.ClientSecret

	// Check feature by name
	featureUser = "test-user"
	output, err := executeFeaturesCheckCommand(t, []string{tempFeature.Name})

	require.NoError(t, err, "features check by name should succeed")
	assert.Contains(t, output, tempFeature.Name, "Output should contain feature name")

	t.Logf("Features check by name output:\n%s", output)
}

func TestIntegration_FeaturesCheckByNameWithProject(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesCheckTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp tenant, project, and feature
	tempTenant := NewTempTenant(t, client, "features check by name with project test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features check project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)

	// Set tenant and project (for disambiguation)
	tenant = tempTenant.Name
	cfg.Tenant = tempTenant.Name
	cfg.Project = tempProject.Name

	// Create a temporary API key with project access for the temp tenant
	tempAPIKey := NewTempAPIKey(t, client, tempTenant.Name).
		WithProjects([]string{tempProject.Name}).
		MustCreate(t).Cleanup(t)
	checkClientID = tempAPIKey.ClientID
	checkClientSecret = tempAPIKey.ClientSecret

	// Check feature by name with project
	featureUser = "test-user"
	output, err := executeFeaturesCheckCommand(t, []string{tempFeature.Name})

	require.NoError(t, err, "features check by name with project should succeed")
	assert.Contains(t, output, tempFeature.Name, "Output should contain feature name")

	t.Logf("Features check by name with project output:\n%s", output)
}

func TestIntegration_FeaturesCheckJSONOutput(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesCheckTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp tenant, project, and feature
	tempTenant := NewTempTenant(t, client, "features check JSON output test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features check project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)

	// Create a temporary API key with project access for the temp tenant
	tempAPIKey := NewTempAPIKey(t, client, tempTenant.Name).
		WithProjects([]string{tempProject.Name}).
		MustCreate(t).Cleanup(t)
	checkClientID = tempAPIKey.ClientID
	checkClientSecret = tempAPIKey.ClientSecret

	// Check feature with JSON output
	featureUser = "test-user"
	outputFormat = "json"
	output, err := executeFeaturesCheckCommand(t, []string{tempFeature.ID})

	require.NoError(t, err, "features check with JSON output should succeed")

	// Verify JSON is valid
	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "Output should be valid JSON")
	assert.Contains(t, result, "name", "JSON should contain name field")
	assert.Contains(t, result, "active", "JSON should contain active field")

	t.Logf("Features check JSON output:\n%s", output)
}

func TestIntegration_FeaturesCheckMissingClientCredentials(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesCheckTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp tenant, project, and feature
	tempTenant := NewTempTenant(t, client, "features check missing creds test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features check project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)

	// Do NOT set client credentials - should fail
	checkClientID = ""
	checkClientSecret = ""

	// Check feature - should fail without credentials
	featureUser = "test-user"
	_, err := executeFeaturesCheckCommand(t, []string{tempFeature.ID})

	require.Error(t, err, "features check without client credentials should fail")
}

func TestIntegration_FeaturesCheckFeatureNotFound(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesCheckTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp tenant and project for name resolution
	tempTenant := NewTempTenant(t, client, "features check not found test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features check not found project").MustCreate(t).Cleanup(t)

	// Set tenant for name resolution
	tenant = tempTenant.Name
	cfg.Tenant = tempTenant.Name

	// Create a temporary API key with project access for the temp tenant
	tempAPIKey := NewTempAPIKey(t, client, tempTenant.Name).
		WithProjects([]string{tempProject.Name}).
		MustCreate(t).Cleanup(t)
	checkClientID = tempAPIKey.ClientID
	checkClientSecret = tempAPIKey.ClientSecret

	// Check non-existent feature by name
	featureUser = "test-user"
	_, err := executeFeaturesCheckCommand(t, []string{"non-existent-feature"})

	require.Error(t, err, "features check for non-existent feature should fail")
	assert.Contains(t, err.Error(), "no feature named", "Error should mention feature not found")
}

func TestIntegration_FeaturesCheckNameRequiresTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesCheckTest(t, env)
	defer cleanup()

	// This test validates CLI behavior - error happens at CLI level before any API call
	// So no API credentials or tenant/project setup is needed

	// Do NOT set tenant - should fail for name resolution
	tenant = ""

	// Check feature by name without tenant - error should occur at CLI level before API call
	featureUser = "test-user"
	_, err := executeFeaturesCheckCommand(t, []string{"some-feature-name"})

	require.Error(t, err, "features check by name without tenant should fail")
	assert.Contains(t, err.Error(), "tenant", "Error should mention tenant is required")
}

// ============================================================================
// Features Check-Bulk Tests (Multiple Features)
// ============================================================================

func TestIntegration_FeaturesCheckBulkByProject(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesCheckTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp tenant, project, and features
	tempTenant := NewTempTenant(t, client, "features check-bulk by project test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features check-bulk project").MustCreate(t).Cleanup(t)
	tempFeature1 := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)
	tempFeature2 := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(false).MustCreate(t).Cleanup(t)

	// Create a temporary API key with project access for the temp tenant
	tempAPIKey := NewTempAPIKey(t, client, tempTenant.Name).
		WithProjects([]string{tempProject.Name}).
		MustCreate(t).Cleanup(t)
	checkClientID = tempAPIKey.ClientID
	checkClientSecret = tempAPIKey.ClientSecret

	// Get project ID by fetching the project
	project, err := izanami.GetProject(client, context.Background(), tempTenant.Name, tempProject.Name, izanami.ParseProject)
	require.NoError(t, err, "Should get project")

	// Check features by project UUID
	checkProjects = []string{project.ID}
	featureUser = "test-user"
	output, err := executeFeaturesCheckBulkCommand(t, []string{})

	require.NoError(t, err, "features check-bulk by project should succeed")
	assert.Contains(t, output, tempFeature1.Name, "Output should contain first feature name")
	assert.Contains(t, output, tempFeature2.Name, "Output should contain second feature name")

	t.Logf("Features check-bulk by project output:\n%s", output)
}

func TestIntegration_FeaturesCheckBulkByProjectName(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesCheckTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp tenant, project, and features
	tempTenant := NewTempTenant(t, client, "features check-bulk by project name test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features check-bulk project").MustCreate(t).Cleanup(t)
	tempFeature1 := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)
	tempFeature2 := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(false).MustCreate(t).Cleanup(t)

	// Set tenant for name resolution
	tenant = tempTenant.Name
	cfg.Tenant = tempTenant.Name

	// Create a temporary API key with project access for the temp tenant
	tempAPIKey := NewTempAPIKey(t, client, tempTenant.Name).
		WithProjects([]string{tempProject.Name}).
		MustCreate(t).Cleanup(t)
	checkClientID = tempAPIKey.ClientID
	checkClientSecret = tempAPIKey.ClientSecret

	// Check features by project NAME (requires tenant for resolution)
	checkProjects = []string{tempProject.Name}
	featureUser = "test-user"
	output, err := executeFeaturesCheckBulkCommand(t, []string{})

	require.NoError(t, err, "features check-bulk by project name should succeed")
	assert.Contains(t, output, tempFeature1.Name, "Output should contain first feature name")
	assert.Contains(t, output, tempFeature2.Name, "Output should contain second feature name")

	t.Logf("Features check-bulk by project name output:\n%s", output)
}

func TestIntegration_FeaturesCheckBulkByFeatureIDs(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesCheckTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp tenant, project, and features
	tempTenant := NewTempTenant(t, client, "features check-bulk by feature IDs test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features check-bulk project").MustCreate(t).Cleanup(t)
	tempFeature1 := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)
	tempFeature2 := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(false).MustCreate(t).Cleanup(t)

	// Create a temporary API key with project access for the temp tenant
	tempAPIKey := NewTempAPIKey(t, client, tempTenant.Name).
		WithProjects([]string{tempProject.Name}).
		MustCreate(t).Cleanup(t)
	checkClientID = tempAPIKey.ClientID
	checkClientSecret = tempAPIKey.ClientSecret

	// Check specific features by UUID
	checkFeatures = []string{tempFeature1.ID, tempFeature2.ID}
	featureUser = "test-user"
	output, err := executeFeaturesCheckBulkCommand(t, []string{})

	require.NoError(t, err, "features check-bulk by feature IDs should succeed")
	assert.Contains(t, output, tempFeature1.Name, "Output should contain first feature name")
	assert.Contains(t, output, tempFeature2.Name, "Output should contain second feature name")

	t.Logf("Features check-bulk by feature IDs output:\n%s", output)
}

func TestIntegration_FeaturesCheckBulkByFeatureNames(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesCheckTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp tenant, project, and features
	tempTenant := NewTempTenant(t, client, "features check-bulk by feature names test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features check-bulk project").MustCreate(t).Cleanup(t)
	tempFeature1 := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)
	tempFeature2 := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(false).MustCreate(t).Cleanup(t)

	// Set tenant for name resolution
	tenant = tempTenant.Name
	cfg.Tenant = tempTenant.Name

	// Create a temporary API key with project access for the temp tenant
	tempAPIKey := NewTempAPIKey(t, client, tempTenant.Name).
		WithProjects([]string{tempProject.Name}).
		MustCreate(t).Cleanup(t)
	checkClientID = tempAPIKey.ClientID
	checkClientSecret = tempAPIKey.ClientSecret

	// Check features by NAME (requires tenant for resolution)
	checkFeatures = []string{tempFeature1.Name, tempFeature2.Name}
	featureUser = "test-user"
	output, err := executeFeaturesCheckBulkCommand(t, []string{})

	require.NoError(t, err, "features check-bulk by feature names should succeed")
	assert.Contains(t, output, tempFeature1.Name, "Output should contain first feature name")
	assert.Contains(t, output, tempFeature2.Name, "Output should contain second feature name")

	t.Logf("Features check-bulk by feature names output:\n%s", output)
}

func TestIntegration_FeaturesCheckBulkJSONOutput(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesCheckTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp tenant, project, and feature
	tempTenant := NewTempTenant(t, client, "features check-bulk JSON output test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features check-bulk project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)

	// Create a temporary API key with project access for the temp tenant
	tempAPIKey := NewTempAPIKey(t, client, tempTenant.Name).
		WithProjects([]string{tempProject.Name}).
		MustCreate(t).Cleanup(t)
	checkClientID = tempAPIKey.ClientID
	checkClientSecret = tempAPIKey.ClientSecret

	// Check features with JSON output
	checkFeatures = []string{tempFeature.ID}
	featureUser = "test-user"
	outputFormat = "json"
	output, err := executeFeaturesCheckBulkCommand(t, []string{})

	require.NoError(t, err, "features check-bulk with JSON output should succeed")

	// Verify JSON is valid - it's a map of feature ID to activation result
	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "Output should be valid JSON")
	assert.Contains(t, result, tempFeature.ID, "JSON should contain feature ID as key")

	t.Logf("Features check-bulk JSON output:\n%s", output)
}

func TestIntegration_FeaturesCheckBulkWithConditions(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesCheckTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp tenant, project, and feature
	tempTenant := NewTempTenant(t, client, "features check-bulk with conditions test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features check-bulk project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)

	// Create a temporary API key with project access for the temp tenant
	tempAPIKey := NewTempAPIKey(t, client, tempTenant.Name).
		WithProjects([]string{tempProject.Name}).
		MustCreate(t).Cleanup(t)
	checkClientID = tempAPIKey.ClientID
	checkClientSecret = tempAPIKey.ClientSecret

	// Check features with conditions
	checkFeatures = []string{tempFeature.ID}
	checkConditions = true
	featureUser = "test-user"
	outputFormat = "json"
	output, err := executeFeaturesCheckBulkCommand(t, []string{})

	require.NoError(t, err, "features check-bulk with conditions should succeed")

	// Verify JSON is valid and contains conditions
	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "Output should be valid JSON")

	// The result should have conditions info when --conditions is set
	featureResult, ok := result[tempFeature.ID].(map[string]interface{})
	if ok {
		// Conditions may or may not be present depending on feature setup
		t.Logf("Feature result contains conditions field: %v", featureResult["conditions"] != nil)
	}

	t.Logf("Features check-bulk with conditions output:\n%s", output)
}

func TestIntegration_FeaturesCheckBulkMissingFilter(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesCheckTest(t, env)
	defer cleanup()

	// No client credentials needed - error happens at CLI validation level before API call
	// Do NOT set any filter - should fail
	checkFeatures = []string{}
	checkProjects = []string{}
	featureUser = "test-user"

	_, err := executeFeaturesCheckBulkCommand(t, []string{})

	require.Error(t, err, "features check-bulk without filters should fail")
	assert.True(t, strings.Contains(err.Error(), "features") || strings.Contains(err.Error(), "projects"),
		"Error should mention that features or projects filter is required, got: %s", err.Error())
}

func TestIntegration_FeaturesCheckBulkProjectNameRequiresTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesCheckTest(t, env)
	defer cleanup()

	// No client credentials needed - error happens at CLI validation level before API call
	// Do NOT set tenant - should fail for project name resolution
	tenant = ""
	checkProjects = []string{"some-project-name"}
	featureUser = "test-user"

	_, err := executeFeaturesCheckBulkCommand(t, []string{})

	require.Error(t, err, "features check-bulk with project name but no tenant should fail")
	assert.Contains(t, err.Error(), "tenant", "Error should mention tenant is required")
}

func TestIntegration_FeaturesCheckBulkFeatureNameRequiresTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesCheckTest(t, env)
	defer cleanup()

	// No client credentials needed - error happens at CLI validation level before API call
	// Do NOT set tenant - should fail for feature name resolution
	tenant = ""
	checkFeatures = []string{"some-feature-name"}
	featureUser = "test-user"

	_, err := executeFeaturesCheckBulkCommand(t, []string{})

	require.Error(t, err, "features check-bulk with feature name but no tenant should fail")
	assert.Contains(t, err.Error(), "tenant", "Error should mention tenant is required")
}

// ============================================================================
// API Direct Tests
// ============================================================================

func TestIntegration_APICheckFeature(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create temp tenant, project, and feature
	tempTenant := NewTempTenant(t, client, "API check feature test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "API check feature project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)

	// Create a temporary API key with project access for the temp tenant
	tempAPIKey := NewTempAPIKey(t, client, tempTenant.Name).
		WithProjects([]string{tempProject.Name}).
		MustCreate(t).Cleanup(t)
	clientID := tempAPIKey.ClientID
	clientSecret := tempAPIKey.ClientSecret

	// Create a feature check client with client credentials
	clientConfig := &izanami.ResolvedConfig{
		LeaderURL:    env.LeaderURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Timeout:      30,
	}
	checkClient, err := izanami.NewFeatureCheckClient(clientConfig)
	require.NoError(t, err, "Should create feature check client with credentials")

	// Check the feature
	result, err := izanami.CheckFeature(checkClient, context.Background(), tempFeature.ID, "test-user", "", "", izanami.ParseFeatureCheckResult)
	require.NoError(t, err, "CheckFeature API should succeed")

	assert.Equal(t, tempFeature.Name, result.Name, "Feature name should match")
	// Active is interface{}, check as bool
	active, ok := result.Active.(bool)
	require.True(t, ok, "Active should be a boolean")
	assert.True(t, active, "Feature should be active (enabled)")

	t.Logf("API CheckFeature result: name=%s, active=%v", result.Name, result.Active)
}

func TestIntegration_APICheckFeatures(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create temp tenant, project, and features
	tempTenant := NewTempTenant(t, client, "API check features test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "API check features project").MustCreate(t).Cleanup(t)
	tempFeature1 := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)
	tempFeature2 := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(false).MustCreate(t).Cleanup(t)

	// Create a temporary API key with project access for the temp tenant
	tempAPIKey := NewTempAPIKey(t, client, tempTenant.Name).
		WithProjects([]string{tempProject.Name}).
		MustCreate(t).Cleanup(t)
	clientID := tempAPIKey.ClientID
	clientSecret := tempAPIKey.ClientSecret

	// Create a feature check client with client credentials
	clientConfig := &izanami.ResolvedConfig{
		LeaderURL:    env.LeaderURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Timeout:      30,
	}
	checkClient, err := izanami.NewFeatureCheckClient(clientConfig)
	require.NoError(t, err, "Should create feature check client with credentials")

	// Check the features
	request := izanami.CheckFeaturesRequest{
		User:     "test-user",
		Features: []string{tempFeature1.ID, tempFeature2.ID},
	}
	results, err := izanami.CheckFeatures(checkClient, context.Background(), request, izanami.ParseActivationsWithConditions)
	require.NoError(t, err, "CheckFeatures API should succeed")

	// ActivationsWithConditions is a map[string]ActivationWithConditions
	assert.Len(t, results, 2, "Should return 2 feature results")

	// Check that both features are in results (keyed by feature ID)
	activation1, found1 := results[tempFeature1.ID]
	activation2, found2 := results[tempFeature2.ID]

	assert.True(t, found1, "Should find feature 1 in results")
	assert.True(t, found2, "Should find feature 2 in results")

	if found1 {
		assert.Equal(t, tempFeature1.Name, activation1.Name, "Feature 1 name should match")
		active1, ok := activation1.Active.(bool)
		if ok {
			assert.True(t, active1, "Feature 1 should be active")
		}
	}
	if found2 {
		assert.Equal(t, tempFeature2.Name, activation2.Name, "Feature 2 name should match")
		active2, ok := activation2.Active.(bool)
		if ok {
			assert.False(t, active2, "Feature 2 should not be active")
		}
	}

	t.Logf("API CheckFeatures returned %d results", len(results))
}

func TestIntegration_APICheckFeaturesByProject(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create temp tenant, project, and features
	tempTenant := NewTempTenant(t, client, "API check features by project test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "API check features project").MustCreate(t).Cleanup(t)
	uniqueName := fmt.Sprintf("api-test-feature-%d", time.Now().UnixNano())
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithName(uniqueName).WithEnabled(true).MustCreate(t).Cleanup(t)

	// Create a temporary API key with project access for the temp tenant
	tempAPIKey := NewTempAPIKey(t, client, tempTenant.Name).
		WithProjects([]string{tempProject.Name}).
		MustCreate(t).Cleanup(t)
	clientID := tempAPIKey.ClientID
	clientSecret := tempAPIKey.ClientSecret

	// Create a feature check client with client credentials
	clientConfig := &izanami.ResolvedConfig{
		LeaderURL:    env.LeaderURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Timeout:      30,
	}
	checkClient, err := izanami.NewFeatureCheckClient(clientConfig)
	require.NoError(t, err, "Should create feature check client with credentials")

	// Get project ID by fetching the project
	project, err := izanami.GetProject(client, context.Background(), tempTenant.Name, tempProject.Name, izanami.ParseProject)
	require.NoError(t, err, "Should get project")

	// Check features by project
	request := izanami.CheckFeaturesRequest{
		User:     "test-user",
		Projects: []string{project.ID},
	}
	results, err := izanami.CheckFeatures(checkClient, context.Background(), request, izanami.ParseActivationsWithConditions)
	require.NoError(t, err, "CheckFeatures by project API should succeed")

	// ActivationsWithConditions is a map
	assert.GreaterOrEqual(t, len(results), 1, "Should return at least 1 feature result")

	// Find our specific feature (keyed by feature ID)
	activation, found := results[tempFeature.ID]
	assert.True(t, found, "Should find our test feature in results")

	if found {
		assert.Equal(t, tempFeature.Name, activation.Name, "Feature name should match")
		active, ok := activation.Active.(bool)
		if ok {
			assert.True(t, active, "Feature should be active")
		}
	}

	t.Logf("API CheckFeatures by project returned %d results", len(results))
}
