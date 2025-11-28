package cmd

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// setupProjectsTest sets up the global cfg for project command tests
func setupProjectsTest(t *testing.T, env *IntegrationTestEnv, tenantName string) func() {
	t.Helper()

	// Get JWT token from session
	token := env.GetJwtToken(t)

	// Save original values
	origCfg := cfg
	origTenant := tenant // global flag variable used by MergeWithFlags
	origOutputFormat := outputFormat
	origProjectDesc := projectDesc
	origProjectData := projectData
	origDeleteForce := projectsDeleteForce

	// Set up config
	cfg = &izanami.Config{
		BaseURL:  env.BaseURL,
		Username: env.Username,
		JwtToken: token,
		Tenant:   tenantName,
		Timeout:  30,
	}
	// Set tenant global flag so MergeWithFlags picks it up
	tenant = tenantName
	outputFormat = "table"
	projectDesc = ""
	projectData = ""
	projectsDeleteForce = false

	return func() {
		cfg = origCfg
		tenant = origTenant
		outputFormat = origOutputFormat
		projectDesc = origProjectDesc
		projectData = origProjectData
		projectsDeleteForce = origDeleteForce
	}
}

// ============================================================================
// PROJECTS LIST
// ============================================================================

func TestIntegration_ProjectsListAfterLogin(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant - deleting it will cascade delete all projects
	tempTenant := NewTempTenant(t, client, "Projects list test").Cleanup(t).MustCreate(t)

	cleanup := setupProjectsTest(t, env, tempTenant.Name)
	defer cleanup()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminProjectsCmd.SetOut(&buf)
	adminProjectsCmd.SetErr(&buf)
	adminProjectsListCmd.SetOut(&buf)
	adminProjectsListCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "projects", "list"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminProjectsCmd.SetOut(nil)
	adminProjectsCmd.SetErr(nil)
	adminProjectsListCmd.SetOut(nil)
	adminProjectsListCmd.SetErr(nil)

	require.NoError(t, err, "Projects list should succeed")

	// Verify via API that we can list projects (empty for new tenant)
	ctx := context.Background()
	projects, err := izanami.ListProjects(client, ctx, tempTenant.Name, izanami.ParseProjects)
	require.NoError(t, err)

	t.Logf("Listed %d projects in tenant '%s' via API", len(projects), tempTenant.Name)
}

func TestIntegration_ProjectsListJSONOutput(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant - deleting it will cascade delete all projects
	tempTenant := NewTempTenant(t, client, "Projects list JSON test").Cleanup(t).MustCreate(t)

	cleanup := setupProjectsTest(t, env, tempTenant.Name)
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
	adminProjectsCmd.SetOut(&buf)
	adminProjectsCmd.SetErr(&buf)
	adminProjectsListCmd.SetOut(&buf)
	adminProjectsListCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "projects", "list"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminProjectsCmd.SetOut(nil)
	adminProjectsCmd.SetErr(nil)
	adminProjectsListCmd.SetOut(nil)
	adminProjectsListCmd.SetErr(nil)

	require.NoError(t, err, "Projects list JSON should succeed")
	output := buf.String()

	// Should be valid JSON array
	assert.True(t, strings.HasPrefix(strings.TrimSpace(output), "["), "JSON output should start with [")

	t.Logf("Projects list JSON output length: %d chars", len(output))
}

func TestIntegration_ProjectsListMissingTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant just to have valid setup, then clear tenant
	tempTenant := NewTempTenant(t, client, "Projects list missing tenant test").Cleanup(t).MustCreate(t)

	cleanup := setupProjectsTest(t, env, tempTenant.Name)
	defer cleanup()

	// Clear tenant from config AND global flag variable
	cfg.Tenant = ""
	tenant = ""

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminProjectsCmd.SetOut(&buf)
	adminProjectsCmd.SetErr(&buf)
	adminProjectsListCmd.SetOut(&buf)
	adminProjectsListCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "projects", "list"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminProjectsCmd.SetOut(nil)
	adminProjectsCmd.SetErr(nil)
	adminProjectsListCmd.SetOut(nil)
	adminProjectsListCmd.SetErr(nil)

	require.Error(t, err, "Projects list without tenant should fail")

	t.Logf("Expected error for missing tenant: %v", err)
}

// ============================================================================
// PROJECTS GET
// ============================================================================

func TestIntegration_ProjectsGetExisting(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant - deleting it will cascade delete all projects
	tempTenant := NewTempTenant(t, client, "Projects get test").Cleanup(t).MustCreate(t)

	cleanup := setupProjectsTest(t, env, tempTenant.Name)
	defer cleanup()

	// Create a temp project to get (no Cleanup needed - tenant deletion cascades)
	tempProject := NewTempProject(t, client, tempTenant.Name, "Test project for get").MustCreate(t)

	// Set JSON output for reliable assertion
	outputFormat = "json"

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminProjectsCmd.SetOut(&buf)
	adminProjectsCmd.SetErr(&buf)
	adminProjectsGetCmd.SetOut(&buf)
	adminProjectsGetCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "projects", "get", tempProject.Name})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminProjectsCmd.SetOut(nil)
	adminProjectsCmd.SetErr(nil)
	adminProjectsGetCmd.SetOut(nil)
	adminProjectsGetCmd.SetErr(nil)

	require.NoError(t, err, "Projects get should succeed")
	output := buf.String()

	// Should display project name in JSON
	assert.Contains(t, output, tempProject.Name, "Output should contain project name")
	assert.True(t, strings.HasPrefix(strings.TrimSpace(output), "{"), "JSON output should start with {")

	t.Logf("Projects get output for '%s': %d chars", tempProject.Name, len(output))
}

func TestIntegration_ProjectsGetNonExistent(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant - deleting it will cascade delete all projects
	tempTenant := NewTempTenant(t, client, "Projects get non-existent test").Cleanup(t).MustCreate(t)

	cleanup := setupProjectsTest(t, env, tempTenant.Name)
	defer cleanup()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminProjectsCmd.SetOut(&buf)
	adminProjectsCmd.SetErr(&buf)
	adminProjectsGetCmd.SetOut(&buf)
	adminProjectsGetCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "projects", "get", "non-existent-project-12345"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminProjectsCmd.SetOut(nil)
	adminProjectsCmd.SetErr(nil)
	adminProjectsGetCmd.SetOut(nil)
	adminProjectsGetCmd.SetErr(nil)

	require.Error(t, err, "Getting non-existent project should fail")

	t.Logf("Expected error for non-existent project: %v", err)
}

func TestIntegration_ProjectsGetMissingArg(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant - deleting it will cascade delete all projects
	tempTenant := NewTempTenant(t, client, "Projects get missing arg test").Cleanup(t).MustCreate(t)

	cleanup := setupProjectsTest(t, env, tempTenant.Name)
	defer cleanup()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminProjectsCmd.SetOut(&buf)
	adminProjectsCmd.SetErr(&buf)
	adminProjectsGetCmd.SetOut(&buf)
	adminProjectsGetCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "projects", "get"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminProjectsCmd.SetOut(nil)
	adminProjectsCmd.SetErr(nil)
	adminProjectsGetCmd.SetOut(nil)
	adminProjectsGetCmd.SetErr(nil)

	require.Error(t, err, "Get without project name should fail")
	assert.Contains(t, err.Error(), "accepts 1 arg", "Error should mention argument requirement")

	t.Logf("Expected error for missing arg: %v", err)
}

// ============================================================================
// PROJECTS CREATE
// ============================================================================

func TestIntegration_ProjectsCreateAndDelete(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant - deleting it will cascade delete all projects
	tempTenant := NewTempTenant(t, client, "Projects create/delete test").Cleanup(t).MustCreate(t)

	cleanup := setupProjectsTest(t, env, tempTenant.Name)
	defer cleanup()

	// Use TempProject for name generation (no Cleanup - tenant cascade or CLI delete handles it)
	projectDescription := "Integration test project"
	tempProject := NewTempProject(t, client, tempTenant.Name, projectDescription)

	// Set description flag
	projectDesc = projectDescription

	// Create project via CLI
	var createBuf bytes.Buffer
	createCmd := &cobra.Command{Use: "iz"}
	createCmd.AddCommand(adminCmd)
	createCmd.SetOut(&createBuf)
	createCmd.SetErr(&createBuf)
	adminCmd.SetOut(&createBuf)
	adminCmd.SetErr(&createBuf)
	adminProjectsCmd.SetOut(&createBuf)
	adminProjectsCmd.SetErr(&createBuf)
	adminProjectsCreateCmd.SetOut(&createBuf)
	adminProjectsCreateCmd.SetErr(&createBuf)

	createCmd.SetArgs([]string{"admin", "projects", "create", tempProject.Name})
	err := createCmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminProjectsCmd.SetOut(nil)
	adminProjectsCmd.SetErr(nil)
	adminProjectsCreateCmd.SetOut(nil)
	adminProjectsCreateCmd.SetErr(nil)

	require.NoError(t, err, "Project create should succeed")
	createOutput := createBuf.String()
	assert.Contains(t, createOutput, "created successfully", "Should confirm creation")
	assert.Contains(t, createOutput, tempProject.Name, "Should mention project name")
	tempProject.MarkCreated() // Mark for tracking

	t.Logf("Project created: %s/%s", tempTenant.Name, tempProject.Name)

	// Verify via API
	project := tempProject.Get(t)
	assert.Equal(t, tempProject.Name, project.Name, "Project name should match")

	// Delete project via CLI
	projectsDeleteForce = true

	var deleteBuf bytes.Buffer
	deleteCmd := &cobra.Command{Use: "iz"}
	deleteCmd.AddCommand(adminCmd)
	deleteCmd.SetOut(&deleteBuf)
	deleteCmd.SetErr(&deleteBuf)
	adminCmd.SetOut(&deleteBuf)
	adminCmd.SetErr(&deleteBuf)
	adminProjectsCmd.SetOut(&deleteBuf)
	adminProjectsCmd.SetErr(&deleteBuf)
	adminProjectsDeleteCmd.SetOut(&deleteBuf)
	adminProjectsDeleteCmd.SetErr(&deleteBuf)

	deleteCmd.SetArgs([]string{"admin", "projects", "delete", tempProject.Name})
	err = deleteCmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminProjectsCmd.SetOut(nil)
	adminProjectsCmd.SetErr(nil)
	adminProjectsDeleteCmd.SetOut(nil)
	adminProjectsDeleteCmd.SetErr(nil)

	require.NoError(t, err, "Project delete should succeed")
	deleteOutput := deleteBuf.String()
	assert.Contains(t, deleteOutput, "deleted successfully", "Should confirm deletion")

	t.Logf("Project deleted: %s/%s", tempTenant.Name, tempProject.Name)
}

func TestIntegration_ProjectsCreateWithJSONData(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant - deleting it will cascade delete all projects
	tempTenant := NewTempTenant(t, client, "Projects create JSON test").Cleanup(t).MustCreate(t)

	cleanup := setupProjectsTest(t, env, tempTenant.Name)
	defer cleanup()

	// Prepare temp project (no Cleanup - tenant cascade handles it)
	tempProject := NewTempProject(t, client, tempTenant.Name, "Created with JSON data")
	jsonData := fmt.Sprintf(`{"name":"%s","description":"Created with JSON data"}`, tempProject.Name)

	// Set data flag
	projectData = jsonData

	// Create project with JSON data
	var createBuf bytes.Buffer
	createCmd := &cobra.Command{Use: "iz"}
	createCmd.AddCommand(adminCmd)
	createCmd.SetOut(&createBuf)
	createCmd.SetErr(&createBuf)
	adminCmd.SetOut(&createBuf)
	adminCmd.SetErr(&createBuf)
	adminProjectsCmd.SetOut(&createBuf)
	adminProjectsCmd.SetErr(&createBuf)
	adminProjectsCreateCmd.SetOut(&createBuf)
	adminProjectsCreateCmd.SetErr(&createBuf)

	// Mark the flag as changed
	adminProjectsCreateCmd.Flags().Set("data", jsonData)

	createCmd.SetArgs([]string{"admin", "projects", "create", tempProject.Name})
	err := createCmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminProjectsCmd.SetOut(nil)
	adminProjectsCmd.SetErr(nil)
	adminProjectsCreateCmd.SetOut(nil)
	adminProjectsCreateCmd.SetErr(nil)

	require.NoError(t, err, "Project create with JSON should succeed")
	tempProject.MarkCreated() // Mark as created

	// Verify via API
	project := tempProject.Get(t)
	assert.Equal(t, "Created with JSON data", project.Description, "Description from JSON should match")

	t.Logf("Project created with JSON data: %s/%s", tempTenant.Name, tempProject.Name)
}

func TestIntegration_ProjectsCreateDuplicate(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant - deleting it will cascade delete all projects
	tempTenant := NewTempTenant(t, client, "Projects create duplicate test").Cleanup(t).MustCreate(t)

	cleanup := setupProjectsTest(t, env, tempTenant.Name)
	defer cleanup()

	// Create project first time via TempProject (no Cleanup - tenant cascade)
	tempProject := NewTempProject(t, client, tempTenant.Name, "First creation").MustCreate(t)

	// Set description for CLI create
	projectDesc = "Duplicate"

	// Try to create duplicate via CLI
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminProjectsCmd.SetOut(&buf)
	adminProjectsCmd.SetErr(&buf)
	adminProjectsCreateCmd.SetOut(&buf)
	adminProjectsCreateCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "projects", "create", tempProject.Name})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminProjectsCmd.SetOut(nil)
	adminProjectsCmd.SetErr(nil)
	adminProjectsCreateCmd.SetOut(nil)
	adminProjectsCreateCmd.SetErr(nil)

	require.Error(t, err, "Creating duplicate project should fail")

	t.Logf("Expected error for duplicate project: %v", err)
}

// ============================================================================
// PROJECTS DELETE
// ============================================================================

func TestIntegration_ProjectsDeleteWithForce(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant - deleting it will cascade delete all projects
	tempTenant := NewTempTenant(t, client, "Projects delete force test").Cleanup(t).MustCreate(t)

	cleanup := setupProjectsTest(t, env, tempTenant.Name)
	defer cleanup()

	// Create project via TempProject (no Cleanup - testing CLI delete)
	tempProject := NewTempProject(t, client, tempTenant.Name, "To be deleted").MustCreate(t)

	// Set force flag
	projectsDeleteForce = true

	// Delete with force flag
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminProjectsCmd.SetOut(&buf)
	adminProjectsCmd.SetErr(&buf)
	adminProjectsDeleteCmd.SetOut(&buf)
	adminProjectsDeleteCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "projects", "delete", tempProject.Name})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminProjectsCmd.SetOut(nil)
	adminProjectsCmd.SetErr(nil)
	adminProjectsDeleteCmd.SetOut(nil)
	adminProjectsDeleteCmd.SetErr(nil)

	require.NoError(t, err, "Project delete should succeed")
	output := buf.String()
	assert.Contains(t, output, "deleted successfully", "Should confirm deletion")

	// Verify project no longer exists
	ctx := context.Background()
	_, err = izanami.GetProject(client, ctx, tempTenant.Name, tempProject.Name, izanami.ParseProject)
	require.Error(t, err, "Project should no longer exist")

	t.Logf("Project deleted: %s/%s", tempTenant.Name, tempProject.Name)
}

func TestIntegration_ProjectsDeleteWithConfirmation(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant - deleting it will cascade delete all projects
	tempTenant := NewTempTenant(t, client, "Projects delete confirmation test").Cleanup(t).MustCreate(t)

	cleanup := setupProjectsTest(t, env, tempTenant.Name)
	defer cleanup()

	// Create project via TempProject (no Cleanup - testing CLI delete)
	tempProject := NewTempProject(t, client, tempTenant.Name, "To be deleted with confirmation").MustCreate(t)

	// Force flag is false (default from cleanup setup)
	projectsDeleteForce = false

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
	adminProjectsCmd.SetOut(&buf)
	adminProjectsCmd.SetErr(&buf)
	adminProjectsCmd.SetIn(input)
	adminProjectsDeleteCmd.SetOut(&buf)
	adminProjectsDeleteCmd.SetErr(&buf)
	adminProjectsDeleteCmd.SetIn(input)

	cmd.SetArgs([]string{"admin", "projects", "delete", tempProject.Name})
	err := cmd.Execute()

	adminCmd.SetIn(nil)
	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminProjectsCmd.SetIn(nil)
	adminProjectsCmd.SetOut(nil)
	adminProjectsCmd.SetErr(nil)
	adminProjectsDeleteCmd.SetIn(nil)
	adminProjectsDeleteCmd.SetOut(nil)
	adminProjectsDeleteCmd.SetErr(nil)

	require.NoError(t, err, "Project delete with confirmation should succeed")
	output := buf.String()
	assert.Contains(t, output, "deleted successfully", "Should confirm deletion")

	// Verify project no longer exists
	ctx := context.Background()
	_, err = izanami.GetProject(client, ctx, tempTenant.Name, tempProject.Name, izanami.ParseProject)
	require.Error(t, err, "Project should no longer exist")

	t.Logf("Project deleted with confirmation: %s/%s", tempTenant.Name, tempProject.Name)
}

func TestIntegration_ProjectsDeleteCancelled(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant - deleting it will cascade delete all projects
	tempTenant := NewTempTenant(t, client, "Projects delete cancelled test").Cleanup(t).MustCreate(t)

	cleanup := setupProjectsTest(t, env, tempTenant.Name)
	defer cleanup()

	// Create project via TempProject (no Cleanup - tenant cascade handles it)
	tempProject := NewTempProject(t, client, tempTenant.Name, "Should not be deleted").MustCreate(t)

	// Force flag is false
	projectsDeleteForce = false

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
	adminProjectsCmd.SetOut(&buf)
	adminProjectsCmd.SetErr(&buf)
	adminProjectsCmd.SetIn(input)
	adminProjectsDeleteCmd.SetOut(&buf)
	adminProjectsDeleteCmd.SetErr(&buf)
	adminProjectsDeleteCmd.SetIn(input)

	cmd.SetArgs([]string{"admin", "projects", "delete", tempProject.Name})
	err := cmd.Execute()

	adminCmd.SetIn(nil)
	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminProjectsCmd.SetIn(nil)
	adminProjectsCmd.SetOut(nil)
	adminProjectsCmd.SetErr(nil)
	adminProjectsDeleteCmd.SetIn(nil)
	adminProjectsDeleteCmd.SetOut(nil)
	adminProjectsDeleteCmd.SetErr(nil)

	require.NoError(t, err, "Cancelled delete should not error")

	// Verify project still exists
	project := tempProject.Get(t)
	assert.Equal(t, tempProject.Name, project.Name)

	t.Logf("Project deletion cancelled: %s/%s still exists", tempTenant.Name, tempProject.Name)
}

func TestIntegration_ProjectsDeleteNonExistent(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant - deleting it will cascade delete all projects
	tempTenant := NewTempTenant(t, client, "Projects delete non-existent test").Cleanup(t).MustCreate(t)

	cleanup := setupProjectsTest(t, env, tempTenant.Name)
	defer cleanup()

	// Set force flag
	projectsDeleteForce = true

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminProjectsCmd.SetOut(&buf)
	adminProjectsCmd.SetErr(&buf)
	adminProjectsDeleteCmd.SetOut(&buf)
	adminProjectsDeleteCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "projects", "delete", "non-existent-project-99999"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminProjectsCmd.SetOut(nil)
	adminProjectsCmd.SetErr(nil)
	adminProjectsDeleteCmd.SetOut(nil)
	adminProjectsDeleteCmd.SetErr(nil)

	// Delete of non-existent may succeed (idempotent) or fail depending on server
	// Either is acceptable behavior
	t.Logf("Delete non-existent project result: err=%v", err)
}

// ============================================================================
// AUTH ERROR CASES
// ============================================================================

func TestIntegration_ProjectsListWithoutLogin(t *testing.T) {
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
	adminProjectsCmd.SetOut(&buf)
	adminProjectsCmd.SetErr(&buf)
	adminProjectsListCmd.SetOut(&buf)
	adminProjectsListCmd.SetErr(&buf)

	cmd.SetArgs([]string{"admin", "projects", "list"})
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminProjectsCmd.SetOut(nil)
	adminProjectsCmd.SetErr(nil)
	adminProjectsListCmd.SetOut(nil)
	adminProjectsListCmd.SetErr(nil)

	require.Error(t, err, "Projects list without login should fail")

	t.Logf("Expected error without login: %v", err)
}
