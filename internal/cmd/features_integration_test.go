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

// ============================================================================
// TempFeature - Temporary feature management for integration tests
// ============================================================================

// TempFeature manages a temporary feature for integration tests
type TempFeature struct {
	ID          string
	Name        string
	Description string
	Tenant      string
	Project     string
	Enabled     bool
	Tags        []string
	client      *izanami.AdminClient
	ctx         context.Context
	created     bool
}

// NewTempFeature creates a new temporary feature helper with auto-generated unique name
func NewTempFeature(t *testing.T, client *izanami.AdminClient, tenant, project string) *TempFeature {
	t.Helper()
	name := fmt.Sprintf("test-feature-%d", time.Now().UnixNano())
	return &TempFeature{
		Name:        name,
		Description: "Integration test feature",
		Tenant:      tenant,
		Project:     project,
		Enabled:     false,
		Tags:        []string{},
		client:      client,
		ctx:         context.Background(),
		created:     false,
	}
}

// WithName sets a custom name for the feature
func (tf *TempFeature) WithName(name string) *TempFeature {
	tf.Name = name
	return tf
}

// WithDescription sets the feature description
func (tf *TempFeature) WithDescription(desc string) *TempFeature {
	tf.Description = desc
	return tf
}

// WithEnabled sets the enabled state
func (tf *TempFeature) WithEnabled(enabled bool) *TempFeature {
	tf.Enabled = enabled
	return tf
}

// WithTags sets the tags
func (tf *TempFeature) WithTags(tags []string) *TempFeature {
	tf.Tags = tags
	return tf
}

// Create creates the feature on the server
func (tf *TempFeature) Create(t *testing.T) error {
	t.Helper()
	payload := map[string]interface{}{
		"name":        tf.Name,
		"description": tf.Description,
		"enabled":     tf.Enabled,
		"resultType":  "boolean",
		"conditions":  []interface{}{},
		"metadata":    map[string]interface{}{},
	}
	if len(tf.Tags) > 0 {
		payload["tags"] = tf.Tags
	}

	created, err := tf.client.CreateFeature(tf.ctx, tf.Tenant, tf.Project, payload)
	if err == nil {
		tf.ID = created.ID
		tf.created = true
		t.Logf("TempFeature created: %s (ID: %s, tenant: %s, project: %s)", tf.Name, tf.ID, tf.Tenant, tf.Project)
	}
	return err
}

// MustCreate creates the feature and fails the test on error
func (tf *TempFeature) MustCreate(t *testing.T) *TempFeature {
	t.Helper()
	err := tf.Create(t)
	require.NoError(t, err, "Failed to create temp feature %s", tf.Name)
	return tf
}

// Get retrieves the current feature state from server
func (tf *TempFeature) Get(t *testing.T) *izanami.FeatureWithOverloads {
	t.Helper()
	feature, err := izanami.GetFeature(tf.client, tf.ctx, tf.Tenant, tf.ID, izanami.ParseFeature)
	require.NoError(t, err, "Failed to get temp feature %s", tf.Name)
	return feature
}

// Delete removes the feature from server
func (tf *TempFeature) Delete(t *testing.T) {
	t.Helper()
	if !tf.created {
		return
	}
	err := tf.client.DeleteFeature(tf.ctx, tf.Tenant, tf.ID)
	if err != nil {
		t.Logf("Warning: failed to delete temp feature %s: %v", tf.Name, err)
	} else {
		t.Logf("TempFeature deleted: %s", tf.Name)
		tf.created = false
	}
}

// Cleanup registers automatic deletion via t.Cleanup()
func (tf *TempFeature) Cleanup(t *testing.T) *TempFeature {
	t.Helper()
	t.Cleanup(func() {
		tf.Delete(t)
	})
	return tf
}

// MarkCreated marks the feature as created (for when creation happens outside TempFeature)
func (tf *TempFeature) MarkCreated() *TempFeature {
	tf.created = true
	return tf
}

// ============================================================================
// Test Setup Helpers
// ============================================================================

// setupFeaturesTest sets up the test environment and logs in
func setupFeaturesTest(t *testing.T, env *IntegrationTestEnv) func() {
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

	// Reset feature-specific flags to defaults
	featureTag = ""
	featureTags = []string{}
	featureName = ""
	featureDesc = ""
	featureEnabled = false
	featuresDeleteForce = false
	featureTestDate = "now"
	featureTestFeatures = []string{}
	featureTestProjects = []string{}
	featureTestOneTagIn = []string{}
	featureTestAllTagsIn = []string{}
	featureTestNoTagIn = []string{}
	featureData = ""
	featureUser = ""
	featureContextStr = ""

	return func() {
		cfg = origCfg
		outputFormat = origOutputFormat
		tenant = origTenant
		project = origProject
		compactJSON = origCompactJSON
		featureTag = ""
		featureTags = []string{}
		featureName = ""
		featureDesc = ""
		featureEnabled = false
		featuresDeleteForce = false
		featureTestDate = "now"
		featureTestFeatures = []string{}
		featureTestProjects = []string{}
		featureTestOneTagIn = []string{}
		featureTestAllTagsIn = []string{}
		featureTestNoTagIn = []string{}
		featureData = ""
		featureUser = ""
		featureContextStr = ""
	}
}

// executeFeaturesCommand executes a features command with proper setup
func executeFeaturesCommand(t *testing.T, args []string) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.PersistentFlags().StringVar(&project, "project", "", "Default project")
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	featuresCmd.SetOut(&buf)
	featuresCmd.SetErr(&buf)
	featuresListCmd.SetOut(&buf)
	featuresListCmd.SetErr(&buf)
	featuresGetCmd.SetOut(&buf)
	featuresGetCmd.SetErr(&buf)
	featuresCreateCmd.SetOut(&buf)
	featuresCreateCmd.SetErr(&buf)
	featuresUpdateCmd.SetOut(&buf)
	featuresUpdateCmd.SetErr(&buf)
	featuresDeleteCmd.SetOut(&buf)
	featuresDeleteCmd.SetErr(&buf)
	featuresPatchCmd.SetOut(&buf)
	featuresPatchCmd.SetErr(&buf)
	featuresTestCmd.SetOut(&buf)
	featuresTestCmd.SetErr(&buf)
	featuresTestDefinitionCmd.SetOut(&buf)
	featuresTestDefinitionCmd.SetErr(&buf)
	featuresTestBulkCmd.SetOut(&buf)
	featuresTestBulkCmd.SetErr(&buf)

	fullArgs := append([]string{"admin", "features"}, args...)
	cmd.SetArgs(fullArgs)
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	featuresCmd.SetOut(nil)
	featuresCmd.SetErr(nil)
	featuresListCmd.SetOut(nil)
	featuresListCmd.SetErr(nil)
	featuresGetCmd.SetOut(nil)
	featuresGetCmd.SetErr(nil)
	featuresCreateCmd.SetOut(nil)
	featuresCreateCmd.SetErr(nil)
	featuresUpdateCmd.SetOut(nil)
	featuresUpdateCmd.SetErr(nil)
	featuresDeleteCmd.SetOut(nil)
	featuresDeleteCmd.SetErr(nil)
	featuresPatchCmd.SetOut(nil)
	featuresPatchCmd.SetErr(nil)
	featuresTestCmd.SetOut(nil)
	featuresTestCmd.SetErr(nil)
	featuresTestDefinitionCmd.SetOut(nil)
	featuresTestDefinitionCmd.SetErr(nil)
	featuresTestBulkCmd.SetOut(nil)
	featuresTestBulkCmd.SetErr(nil)

	return buf.String(), err
}

// executeFeaturesCommandWithInput executes a features command with stdin input
func executeFeaturesCommandWithInput(t *testing.T, args []string, input string) (string, error) {
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
	featuresCmd.SetOut(&buf)
	featuresCmd.SetErr(&buf)
	featuresCmd.SetIn(inputBuf)
	featuresDeleteCmd.SetOut(&buf)
	featuresDeleteCmd.SetErr(&buf)
	featuresDeleteCmd.SetIn(inputBuf)

	fullArgs := append([]string{"admin", "features"}, args...)
	cmd.SetArgs(fullArgs)
	err := cmd.Execute()

	adminCmd.SetIn(nil)
	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	featuresCmd.SetIn(nil)
	featuresCmd.SetOut(nil)
	featuresCmd.SetErr(nil)
	featuresDeleteCmd.SetIn(nil)
	featuresDeleteCmd.SetOut(nil)
	featuresDeleteCmd.SetErr(nil)

	return buf.String(), err
}

// ============================================================================
// Features List Tests
// ============================================================================

func TestIntegration_FeaturesListBasic(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp tenant and project with a feature
	tempTenant := NewTempTenant(t, client, "features list test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features list project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).MustCreate(t).Cleanup(t)

	// Set both the global tenant variable and cfg.Tenant to ensure proper state
	tenant = tempTenant.Name
	cfg.Tenant = tempTenant.Name

	output, err := executeFeaturesCommand(t, []string{"list"})

	require.NoError(t, err, "features list should succeed")
	assert.Contains(t, output, tempFeature.Name, "Output should contain feature name")

	t.Logf("Features list output:\n%s", output)
}

func TestIntegration_FeaturesListWithProjectFilter(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create temp tenant with two projects
	tempTenant := NewTempTenant(t, client, "features list project filter test").MustCreate(t).Cleanup(t)
	tempProject1 := NewTempProject(t, client, tempTenant.Name, "features project 1").MustCreate(t).Cleanup(t)
	tempProject2 := NewTempProject(t, client, tempTenant.Name, "features project 2").MustCreate(t).Cleanup(t)

	feature1 := NewTempFeature(t, client, tempTenant.Name, tempProject1.Name).WithName("feature-in-proj1").MustCreate(t).Cleanup(t)
	feature2 := NewTempFeature(t, client, tempTenant.Name, tempProject2.Name).WithName("feature-in-proj2").MustCreate(t).Cleanup(t)

	// Set tenant for global state
	tenant = tempTenant.Name
	cfg.Tenant = tempTenant.Name

	// Pass --project flag to filter by project 1
	output, err := executeFeaturesCommand(t, []string{"list", "--project", tempProject1.Name})

	require.NoError(t, err, "features list with --project should succeed")
	assert.Contains(t, output, feature1.Name, "Output should contain feature from project 1")
	assert.NotContains(t, output, feature2.Name, "Output should not contain feature from project 2")

	t.Logf("Features list with project filter output:\n%s", output)
}

func TestIntegration_FeaturesListJSONOutput(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "features list json test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features json project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name
	outputFormat = "json"

	output, err := executeFeaturesCommand(t, []string{"list"})

	require.NoError(t, err, "features list with JSON output should succeed")
	assert.Contains(t, output, tempFeature.Name, "JSON output should contain feature name")
	assert.Contains(t, output, "\"name\"", "Output should be valid JSON")

	t.Logf("Features list JSON output:\n%s", output)
}

func TestIntegration_FeaturesListMissingTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesTest(t, env)
	defer cleanup()

	// Don't set tenant
	_, err := executeFeaturesCommand(t, []string{"list"})

	require.Error(t, err, "features list without tenant should fail")
	assert.Contains(t, err.Error(), "tenant", "Error should mention tenant is required")
}

// ============================================================================
// Features Get Tests
// ============================================================================

func TestIntegration_FeaturesGetExisting(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "features get test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features get project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name

	output, err := executeFeaturesCommand(t, []string{"get", tempFeature.ID})

	require.NoError(t, err, "features get should succeed")
	assert.Contains(t, output, tempFeature.Name, "Output should contain feature name")

	t.Logf("Features get output:\n%s", output)
}

func TestIntegration_FeaturesGetNotFound(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "features get not found test").MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name

	_, err := executeFeaturesCommand(t, []string{"get", "nonexistent-feature-id"})

	require.Error(t, err, "features get for nonexistent feature should fail")
}

// ============================================================================
// Features Create Tests
// ============================================================================

func TestIntegration_FeaturesCreateSimple(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "features create test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features create project").MustCreate(t).Cleanup(t)

	featureName := fmt.Sprintf("cli-created-feature-%d", time.Now().UnixNano())

	// Register cleanup for the feature we're about to create
	t.Cleanup(func() {
		features, _ := izanami.ListFeatures(client, context.Background(), tempTenant.Name, "", izanami.ParseFeatures)
		for _, f := range features {
			if f.Name == featureName {
				_ = client.DeleteFeature(context.Background(), tempTenant.Name, f.ID)
				break
			}
		}
	})

	// Set tenant for global state
	tenant = tempTenant.Name
	cfg.Tenant = tempTenant.Name

	// Pass --project flag explicitly
	output, err := executeFeaturesCommand(t, []string{"create", featureName, "--project", tempProject.Name, "--description", "Test feature", "--enabled"})

	require.NoError(t, err, "features create should succeed")
	assert.Contains(t, output, "created successfully", "Output should confirm creation")
	assert.Contains(t, output, featureName, "Output should contain feature name")

	// Verify via API
	features, err := izanami.ListFeatures(client, context.Background(), tempTenant.Name, "", izanami.ParseFeatures)
	require.NoError(t, err, "Should list features via API")

	found := false
	for _, f := range features {
		if f.Name == featureName {
			found = true
			assert.True(t, f.Enabled, "Feature should be enabled")
			break
		}
	}
	assert.True(t, found, "Created feature should be found in list")

	t.Logf("Features create output:\n%s", output)
}

func TestIntegration_FeaturesCreateWithJSON(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "features create json test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features create json project").MustCreate(t).Cleanup(t)

	featureName := fmt.Sprintf("cli-json-feature-%d", time.Now().UnixNano())

	// Register cleanup
	t.Cleanup(func() {
		features, _ := izanami.ListFeatures(client, context.Background(), tempTenant.Name, "", izanami.ParseFeatures)
		for _, f := range features {
			if f.Name == featureName {
				_ = client.DeleteFeature(context.Background(), tempTenant.Name, f.ID)
				break
			}
		}
	})

	jsonData := fmt.Sprintf(`{"name":"%s","enabled":true,"description":"JSON created"}`, featureName)

	// Set both global variables and cfg fields to ensure proper state after preRunSetup
	tenant = tempTenant.Name
	cfg.Tenant = tempTenant.Name

	output, err := executeFeaturesCommand(t, []string{"create", featureName, "--project", tempProject.Name, "--data", jsonData})

	require.NoError(t, err, "features create with --data should succeed")
	assert.Contains(t, output, "created successfully", "Output should confirm creation")

	t.Logf("Features create with JSON output:\n%s", output)
}

func TestIntegration_FeaturesCreateMissingProject(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "features create missing project test").MustCreate(t).Cleanup(t)

	// Set tenant but no project to test that project is required
	tenant = tempTenant.Name
	cfg.Tenant = tempTenant.Name

	_, err := executeFeaturesCommand(t, []string{"create", "some-feature"})

	require.Error(t, err, "features create without project should fail")
	assert.Contains(t, err.Error(), "project", "Error should mention project is required")
}

// ============================================================================
// Features Delete Tests
// ============================================================================

func TestIntegration_FeaturesDeleteWithForce(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "features delete test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features delete project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).MustCreate(t)
	// Don't register cleanup - we're deleting it

	tenant = tempTenant.Name
	featuresDeleteForce = true

	output, err := executeFeaturesCommand(t, []string{"delete", tempFeature.ID, "--force"})

	require.NoError(t, err, "features delete --force should succeed")
	assert.Contains(t, output, "deleted successfully", "Output should confirm deletion")

	// Verify via API
	features, err := izanami.ListFeatures(client, context.Background(), tempTenant.Name, "", izanami.ParseFeatures)
	require.NoError(t, err, "Should list features via API")

	for _, f := range features {
		assert.NotEqual(t, tempFeature.ID, f.ID, "Deleted feature should not be in list")
	}

	t.Logf("Features delete output:\n%s", output)
}

func TestIntegration_FeaturesDeleteWithoutForceAbort(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "features delete abort test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features delete abort project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name

	output, err := executeFeaturesCommandWithInput(t, []string{"delete", tempFeature.ID}, "n\n")

	require.NoError(t, err, "features delete aborted should not error")
	assert.Contains(t, output, "Cancelled", "Output should indicate deletion was cancelled")

	// Verify feature still exists
	feature := tempFeature.Get(t)
	assert.NotNil(t, feature, "Feature should still exist after aborted deletion")

	t.Logf("Features delete aborted output:\n%s", output)
}

// ============================================================================
// Features Patch Tests
// ============================================================================

func TestIntegration_FeaturesPatchEnableDisable(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "features patch test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features patch project").MustCreate(t).Cleanup(t)
	feature1 := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)
	feature2 := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(false).MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name

	// Patch to disable feature1 and enable feature2
	patchData := fmt.Sprintf(`[{"op":"replace","path":"/%s/enabled","value":false},{"op":"replace","path":"/%s/enabled","value":true}]`, feature1.ID, feature2.ID)
	featureData = patchData

	output, err := executeFeaturesCommand(t, []string{"patch", "--data", patchData})

	require.NoError(t, err, "features patch should succeed")
	assert.Contains(t, output, "patched successfully", "Output should confirm patch")

	// Verify via API
	f1 := feature1.Get(t)
	f2 := feature2.Get(t)

	assert.False(t, f1.Enabled, "Feature 1 should now be disabled")
	assert.True(t, f2.Enabled, "Feature 2 should now be enabled")

	t.Logf("Features patch output:\n%s", output)
}

func TestIntegration_FeaturesPatchMissingData(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "features patch missing data test").MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name

	_, err := executeFeaturesCommand(t, []string{"patch"})

	require.Error(t, err, "features patch without --data should fail")
	assert.Contains(t, err.Error(), "data", "Error should mention data is required")
}

// ============================================================================
// Features Test Tests
// ============================================================================

func TestIntegration_FeaturesTestExisting(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "features test test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features test project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name

	output, err := executeFeaturesCommand(t, []string{"test", tempFeature.ID})

	require.NoError(t, err, "features test should succeed")
	assert.Contains(t, output, tempFeature.Name, "Output should contain feature name")
	assert.Contains(t, output, "true", "Output should show feature is active")

	t.Logf("Features test output:\n%s", output)
}

func TestIntegration_FeaturesTestWithUser(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "features test user test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features test user project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name
	featureUser = "test-user-123"

	output, err := executeFeaturesCommand(t, []string{"test", tempFeature.ID, "--user", "test-user-123"})

	require.NoError(t, err, "features test with --user should succeed")
	assert.Contains(t, output, tempFeature.Name, "Output should contain feature name")

	t.Logf("Features test with user output:\n%s", output)
}

func TestIntegration_FeaturesTestJSONOutput(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "features test json test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features test json project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name
	outputFormat = "json"

	output, err := executeFeaturesCommand(t, []string{"test", tempFeature.ID})

	require.NoError(t, err, "features test with JSON output should succeed")
	assert.Contains(t, output, "\"name\"", "Output should be valid JSON")
	assert.Contains(t, output, "\"active\"", "Output should contain active field")

	t.Logf("Features test JSON output:\n%s", output)
}

// ============================================================================
// Features Test-Definition Tests
// ============================================================================

func TestIntegration_FeaturesTestDefinition(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "features test-definition test").MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name
	featureData = `{"name":"test-def-feature","enabled":true,"resultType":"boolean","conditions":[]}`

	output, err := executeFeaturesCommand(t, []string{"test-definition", "--data", featureData})

	require.NoError(t, err, "features test-definition should succeed")
	assert.Contains(t, output, "test-def-feature", "Output should contain feature name")

	t.Logf("Features test-definition output:\n%s", output)
}

func TestIntegration_FeaturesTestDefinitionMissingData(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "features test-definition missing data test").MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name

	_, err := executeFeaturesCommand(t, []string{"test-definition"})

	require.Error(t, err, "features test-definition without --data should fail")
	// The error can be about invalid JSON (empty input) or data required
	assert.True(t, strings.Contains(err.Error(), "data") || strings.Contains(err.Error(), "JSON"),
		"Error should mention data is required or invalid JSON, got: %s", err.Error())
}

// ============================================================================
// Features Test-Bulk Tests
// ============================================================================

func TestIntegration_FeaturesTestBulkByProject(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "features test-bulk test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features test-bulk project").MustCreate(t).Cleanup(t)
	feature1 := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)
	feature2 := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(false).MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name
	featureTestProjects = []string{tempProject.Name}

	output, err := executeFeaturesCommand(t, []string{"test-bulk", "--projects", tempProject.Name})

	require.NoError(t, err, "features test-bulk should succeed")
	assert.Contains(t, output, feature1.Name, "Output should contain feature 1 name")
	assert.Contains(t, output, feature2.Name, "Output should contain feature 2 name")

	t.Logf("Features test-bulk output:\n%s", output)
}

func TestIntegration_FeaturesTestBulkByFeatureIDs(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "features test-bulk by id test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features test-bulk id project").MustCreate(t).Cleanup(t)
	feature1 := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)
	feature2 := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(false).MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name
	featureTestFeatures = []string{feature1.ID, feature2.ID}

	output, err := executeFeaturesCommand(t, []string{"test-bulk", "--features", feature1.ID + "," + feature2.ID})

	require.NoError(t, err, "features test-bulk by feature IDs should succeed")
	assert.Contains(t, output, feature1.Name, "Output should contain feature 1")
	assert.Contains(t, output, feature2.Name, "Output should contain feature 2")

	t.Logf("Features test-bulk by IDs output:\n%s", output)
}

func TestIntegration_FeaturesTestBulkMissingFilter(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "features test-bulk missing filter test").MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name

	_, err := executeFeaturesCommand(t, []string{"test-bulk"})

	require.Error(t, err, "features test-bulk without filters should fail")
	assert.Contains(t, err.Error(), "filter", "Error should mention filter is required")
}

func TestIntegration_FeaturesTestBulkJSONOutput(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupFeaturesTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "features test-bulk json test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "features test-bulk json project").MustCreate(t).Cleanup(t)
	NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)

	tenant = tempTenant.Name
	outputFormat = "json"
	featureTestProjects = []string{tempProject.Name}

	output, err := executeFeaturesCommand(t, []string{"test-bulk", "--projects", tempProject.Name})

	require.NoError(t, err, "features test-bulk with JSON output should succeed")
	assert.Contains(t, output, "\"name\"", "Output should be valid JSON")
	assert.Contains(t, output, "\"active\"", "Output should contain active field")

	t.Logf("Features test-bulk JSON output:\n%s", output)
}

// ============================================================================
// API Direct Tests (using izanami package)
// ============================================================================

func TestIntegration_APIListFeatures(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "API list features test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "API list features project").MustCreate(t).Cleanup(t)
	feature1 := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).MustCreate(t).Cleanup(t)
	feature2 := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).MustCreate(t).Cleanup(t)

	features, err := izanami.ListFeatures(client, context.Background(), tempTenant.Name, "", izanami.ParseFeatures)
	require.NoError(t, err, "ListFeatures API should succeed")

	names := make([]string, 0, len(features))
	for _, f := range features {
		names = append(names, f.Name)
	}

	assert.Contains(t, names, feature1.Name, "Should contain first feature")
	assert.Contains(t, names, feature2.Name, "Should contain second feature")

	t.Logf("API ListFeatures returned %d features", len(features))
}

func TestIntegration_APIGetFeature(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "API get feature test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "API get feature project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithDescription("API test feature").MustCreate(t).Cleanup(t)

	feature, err := izanami.GetFeature(client, context.Background(), tempTenant.Name, tempFeature.ID, izanami.ParseFeature)
	require.NoError(t, err, "GetFeature API should succeed")

	assert.Equal(t, tempFeature.Name, feature.Name, "Feature name should match")
	assert.Equal(t, tempProject.Name, feature.Project, "Feature project should match")
}

func TestIntegration_APICreateFeature(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "API create feature test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "API create feature project").MustCreate(t).Cleanup(t)

	featureName := fmt.Sprintf("api-created-feature-%d", time.Now().UnixNano())

	// Register cleanup
	t.Cleanup(func() {
		features, _ := izanami.ListFeatures(client, context.Background(), tempTenant.Name, "", izanami.ParseFeatures)
		for _, f := range features {
			if f.Name == featureName {
				_ = client.DeleteFeature(context.Background(), tempTenant.Name, f.ID)
				break
			}
		}
	})

	payload := map[string]interface{}{
		"name":        featureName,
		"description": "API created",
		"enabled":     true,
		"resultType":  "boolean",
		"conditions":  []interface{}{},
		"metadata":    map[string]interface{}{},
	}

	created, err := client.CreateFeature(context.Background(), tempTenant.Name, tempProject.Name, payload)
	require.NoError(t, err, "CreateFeature API should succeed")

	assert.Equal(t, featureName, created.Name, "Created feature name should match")
	assert.True(t, created.Enabled, "Created feature should be enabled")
}

func TestIntegration_APIPatchFeatures(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "API patch features test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "API patch features project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)

	// Verify initial state
	feature, _ := izanami.GetFeature(client, context.Background(), tempTenant.Name, tempFeature.ID, izanami.ParseFeature)
	assert.True(t, feature.Enabled, "Feature should initially be enabled")

	// Patch to disable
	patches := []map[string]interface{}{
		{
			"op":    "replace",
			"path":  "/" + tempFeature.ID + "/enabled",
			"value": false,
		},
	}

	err := client.PatchFeatures(context.Background(), tempTenant.Name, patches)
	require.NoError(t, err, "PatchFeatures API should succeed")

	// Verify patch
	feature, _ = izanami.GetFeature(client, context.Background(), tempTenant.Name, tempFeature.ID, izanami.ParseFeature)
	assert.False(t, feature.Enabled, "Feature should now be disabled")
}

func TestIntegration_APITestFeature(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "API test feature test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "API test feature project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)

	result, err := izanami.TestFeature(client, context.Background(), tempTenant.Name, tempFeature.ID, "", "test-user", nowISO8601(), "", izanami.ParseFeatureTestResult)
	require.NoError(t, err, "TestFeature API should succeed")

	assert.Equal(t, tempFeature.Name, result.Name, "Result name should match feature name")
	assert.Equal(t, true, result.Active, "Feature should be active")
}

func TestIntegration_APITestFeaturesBulk(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "API test features bulk test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "API test features bulk project").MustCreate(t).Cleanup(t)

	// Get project UUID for the request
	project, _ := izanami.GetProject(client, context.Background(), tempTenant.Name, tempProject.Name, izanami.ParseProject)

	feature1 := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(true).MustCreate(t).Cleanup(t)
	feature2 := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).WithEnabled(false).MustCreate(t).Cleanup(t)

	request := izanami.TestFeaturesAdminRequest{
		User:     "test-user",
		Date:     nowISO8601(),
		Projects: []string{project.ID},
	}

	results, err := izanami.TestFeaturesBulk(client, context.Background(), tempTenant.Name, request, izanami.ParseFeatureTestResults)
	require.NoError(t, err, "TestFeaturesBulk API should succeed")

	assert.GreaterOrEqual(t, len(results), 2, "Should have at least 2 results")

	// Check results by feature ID
	if result1, ok := results[feature1.ID]; ok {
		assert.Equal(t, true, result1.Active, "Feature 1 should be active")
	}
	if result2, ok := results[feature2.ID]; ok {
		assert.Equal(t, false, result2.Active, "Feature 2 should not be active")
	}

	t.Logf("API TestFeaturesBulk returned %d results", len(results))
}

func TestIntegration_APIDeleteFeature(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	tempTenant := NewTempTenant(t, client, "API delete feature test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "API delete feature project").MustCreate(t).Cleanup(t)
	tempFeature := NewTempFeature(t, client, tempTenant.Name, tempProject.Name).MustCreate(t)
	// Don't register cleanup - we're deleting it

	err := client.DeleteFeature(context.Background(), tempTenant.Name, tempFeature.ID)
	require.NoError(t, err, "DeleteFeature API should succeed")

	// Verify deletion
	features, err := izanami.ListFeatures(client, context.Background(), tempTenant.Name, "", izanami.ParseFeatures)
	require.NoError(t, err)

	for _, f := range features {
		assert.NotEqual(t, tempFeature.ID, f.ID, "Deleted feature should not be in list")
	}
}
