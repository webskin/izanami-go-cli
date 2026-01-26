package cmd

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// ============================================================================
// TempContext - Temporary context management for integration tests
// ============================================================================

// TempContext manages a temporary context for integration tests
type TempContext struct {
	Name       string
	Path       string // Full path including parent (e.g., "parent/child/name")
	Tenant     string
	Project    string // Empty for global contexts
	ParentPath string // Parent context path (e.g., "parent/child")
	Protected  bool
	client     *izanami.AdminClient
	ctx        context.Context
	created    bool
}

// NewTempContext creates a new temporary context helper with auto-generated unique name
func NewTempContext(t *testing.T, client *izanami.AdminClient, tenant string) *TempContext {
	t.Helper()
	name := fmt.Sprintf("testctx%d", time.Now().UnixNano())
	return &TempContext{
		Name:      name,
		Path:      name,
		Tenant:    tenant,
		Project:   "",
		Protected: false,
		client:    client,
		ctx:       context.Background(),
		created:   false,
	}
}

// WithName sets a custom name for the context
func (tc *TempContext) WithName(name string) *TempContext {
	tc.Name = name
	if tc.ParentPath != "" {
		tc.Path = tc.ParentPath + "/" + name
	} else {
		tc.Path = name
	}
	return tc
}

// WithParent sets the parent path for nested contexts
func (tc *TempContext) WithParent(parentPath string) *TempContext {
	tc.ParentPath = parentPath
	if parentPath != "" {
		tc.Path = parentPath + "/" + tc.Name
	} else {
		tc.Path = tc.Name
	}
	return tc
}

// WithProject sets the project (makes it a project context instead of global)
func (tc *TempContext) WithProject(project string) *TempContext {
	tc.Project = project
	return tc
}

// WithProtected sets the protected flag
func (tc *TempContext) WithProtected(protected bool) *TempContext {
	tc.Protected = protected
	return tc
}

// Create creates the context on the server
func (tc *TempContext) Create(t *testing.T) error {
	t.Helper()
	data := map[string]interface{}{
		"name":      tc.Name,
		"protected": tc.Protected,
	}
	err := tc.client.CreateContext(tc.ctx, tc.Tenant, tc.Project, tc.Name, tc.ParentPath, data)
	if err == nil {
		tc.created = true
		t.Logf("TempContext created: %s (tenant: %s, project: %s)", tc.Path, tc.Tenant, tc.Project)
	}
	return err
}

// MustCreate creates the context and fails the test on error
func (tc *TempContext) MustCreate(t *testing.T) *TempContext {
	t.Helper()
	err := tc.Create(t)
	require.NoError(t, err, "Failed to create temp context %s", tc.Path)
	return tc
}

// Delete removes the context from server
func (tc *TempContext) Delete(t *testing.T) {
	t.Helper()
	if !tc.created {
		return
	}
	err := tc.client.DeleteContext(tc.ctx, tc.Tenant, tc.Project, tc.Path)
	if err != nil {
		t.Logf("Warning: failed to delete temp context %s: %v", tc.Path, err)
	} else {
		t.Logf("TempContext deleted: %s", tc.Path)
		tc.created = false
	}
}

// Cleanup registers automatic deletion via t.Cleanup()
func (tc *TempContext) Cleanup(t *testing.T) *TempContext {
	t.Helper()
	t.Cleanup(func() {
		tc.Delete(t)
	})
	return tc
}

// MarkCreated marks the context as created (for when creation happens outside TempContext)
func (tc *TempContext) MarkCreated() *TempContext {
	tc.created = true
	return tc
}

// findContextByNameRecursive searches for a context by name in a hierarchical context list
func findContextByNameRecursive(contexts []izanami.Context, name string) *izanami.Context {
	for i := range contexts {
		if contexts[i].Name == name {
			return &contexts[i]
		}
		// Search in children recursively
		if len(contexts[i].Children) > 0 {
			childContexts := make([]izanami.Context, 0, len(contexts[i].Children))
			for j := range contexts[i].Children {
				if contexts[i].Children[j] != nil {
					childContexts = append(childContexts, *contexts[i].Children[j])
				}
			}
			if found := findContextByNameRecursive(childContexts, name); found != nil {
				return found
			}
		}
	}
	return nil
}

// ============================================================================
// Test Setup Helpers
// ============================================================================

// setupContextsTest sets up the test environment and logs in
func setupContextsTest(t *testing.T, env *IntegrationTestEnv) func() {
	t.Helper()
	env.Login(t)

	// Get JWT token from session
	token := env.GetJwtToken(t)

	// Save original values
	origCfg := cfg
	origOutputFormat := outputFormat
	origTenant := tenant   // Save the global tenant flag
	origProject := project // Save the global project flag

	// Set up config
	cfg = &izanami.Config{
		BaseURL:  env.BaseURL,
		Username: env.Username,
		JwtToken: token,
		Timeout:  30,
	}
	outputFormat = "table"
	tenant = ""  // Will be set per-test
	project = "" // Reset project to avoid polluting other tests

	// Reset global flags to defaults
	contextAll = false
	contextParent = ""
	contextProtected = false
	contextGlobal = false
	contextData = ""
	contextsDeleteForce = false
	contextUpdateProtected = ""

	return func() {
		cfg = origCfg
		outputFormat = origOutputFormat
		tenant = origTenant
		project = origProject
		contextAll = false
		contextParent = ""
		contextProtected = false
		contextGlobal = false
		contextData = ""
		contextsDeleteForce = false
		contextUpdateProtected = ""
	}
}

// executeContextsCommand executes a contexts command with proper setup
func executeContextsCommand(t *testing.T, args []string) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.PersistentFlags().StringVar(&project, "project", "", "Default project")
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	contextsCmd.SetOut(&buf)
	contextsCmd.SetErr(&buf)
	contextsListCmd.SetOut(&buf)
	contextsListCmd.SetErr(&buf)
	contextsGetCmd.SetOut(&buf)
	contextsGetCmd.SetErr(&buf)
	contextsCreateCmd.SetOut(&buf)
	contextsCreateCmd.SetErr(&buf)
	contextsUpdateCmd.SetOut(&buf)
	contextsUpdateCmd.SetErr(&buf)
	contextsDeleteCmd.SetOut(&buf)
	contextsDeleteCmd.SetErr(&buf)

	fullArgs := append([]string{"admin", "contexts"}, args...)
	cmd.SetArgs(fullArgs)
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	contextsCmd.SetOut(nil)
	contextsCmd.SetErr(nil)
	contextsListCmd.SetOut(nil)
	contextsListCmd.SetErr(nil)
	contextsGetCmd.SetOut(nil)
	contextsGetCmd.SetErr(nil)
	contextsCreateCmd.SetOut(nil)
	contextsCreateCmd.SetErr(nil)
	contextsUpdateCmd.SetOut(nil)
	contextsUpdateCmd.SetErr(nil)
	contextsDeleteCmd.SetOut(nil)
	contextsDeleteCmd.SetErr(nil)

	return buf.String(), err
}

// executeContextsCommandWithInput executes a contexts command with stdin input
func executeContextsCommandWithInput(t *testing.T, args []string, input string) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	inputBuf := bytes.NewBufferString(input)

	cmd := &cobra.Command{Use: "iz"}
	cmd.PersistentFlags().StringVar(&project, "project", "", "Default project")
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(inputBuf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminCmd.SetIn(inputBuf)
	contextsCmd.SetOut(&buf)
	contextsCmd.SetErr(&buf)
	contextsCmd.SetIn(inputBuf)
	contextsDeleteCmd.SetOut(&buf)
	contextsDeleteCmd.SetErr(&buf)
	contextsDeleteCmd.SetIn(inputBuf)

	fullArgs := append([]string{"admin", "contexts"}, args...)
	cmd.SetArgs(fullArgs)
	err := cmd.Execute()

	adminCmd.SetIn(nil)
	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	contextsCmd.SetIn(nil)
	contextsCmd.SetOut(nil)
	contextsCmd.SetErr(nil)
	contextsDeleteCmd.SetIn(nil)
	contextsDeleteCmd.SetOut(nil)
	contextsDeleteCmd.SetErr(nil)

	return buf.String(), err
}

// ============================================================================
// Contexts List Tests
// ============================================================================

func TestIntegration_ContextsListWithProject(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupContextsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant with a project
	tempTenant := NewTempTenant(t, client, "contexts list test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "contexts test project").MustCreate(t).Cleanup(t)

	// Create context within project scope
	ctx := NewTempContext(t, client, tempTenant.Name).WithProject(tempProject.Name).MustCreate(t).Cleanup(t)

	// Set tenant global flag (picked up by MergeWithFlags in PersistentPreRunE)
	tenant = tempTenant.Name

	output, err := executeContextsCommand(t, []string{"list", "--project", tempProject.Name})

	require.NoError(t, err, "contexts list should succeed")
	assert.Contains(t, output, ctx.Name, "Output should contain created context name")

	t.Logf("Contexts list output:\n%s", output)
}

func TestIntegration_ContextsListWithAllFlag(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupContextsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant with nested contexts
	tempTenant := NewTempTenant(t, client, "contexts list all test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "contexts all test project").MustCreate(t).Cleanup(t)

	parentCtx := NewTempContext(t, client, tempTenant.Name).WithName("parentctx").WithProject(tempProject.Name).MustCreate(t).Cleanup(t)
	childCtx := NewTempContext(t, client, tempTenant.Name).WithName("childctx").WithParent(parentCtx.Name).WithProject(tempProject.Name).MustCreate(t).Cleanup(t)

	// Set tenant global flag (picked up by MergeWithFlags in PersistentPreRunE)
	tenant = tempTenant.Name

	output, err := executeContextsCommand(t, []string{"list", "--project", tempProject.Name, "--all"})

	require.NoError(t, err, "contexts list --all should succeed")
	assert.Contains(t, output, parentCtx.Name, "Output should contain parent context")
	assert.Contains(t, output, childCtx.Name, "Output should contain child context")

	t.Logf("Contexts list --all output:\n%s", output)
}

func TestIntegration_ContextsListJSONOutput(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupContextsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant with a context
	tempTenant := NewTempTenant(t, client, "contexts list json test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "contexts json test project").MustCreate(t).Cleanup(t)
	ctx := NewTempContext(t, client, tempTenant.Name).WithProject(tempProject.Name).MustCreate(t).Cleanup(t)

	// Set tenant global flag (picked up by MergeWithFlags in PersistentPreRunE)
	tenant = tempTenant.Name
	// Set output format global variable (--output flag is on rootCmd, not inherited in test setup)
	outputFormat = "json"

	cmdOutput, err := executeContextsCommand(t, []string{"list", "--project", tempProject.Name})

	require.NoError(t, err, "contexts list --output json should succeed")
	assert.Contains(t, cmdOutput, ctx.Name, "JSON output should contain context name")
	assert.Contains(t, cmdOutput, "\"name\"", "Output should be valid JSON with name field")

	t.Logf("Contexts list JSON output:\n%s", cmdOutput)
}

func TestIntegration_ContextsListMissingTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupContextsTest(t, env)
	defer cleanup()

	// Don't set tenant - leave tenant flag empty (setupContextsTest already sets tenant = "")

	_, err := executeContextsCommand(t, []string{"list"})

	require.Error(t, err, "contexts list without tenant should fail")
	assert.Contains(t, err.Error(), "tenant", "Error should mention tenant is required")
}

// ============================================================================
// Contexts Get Tests
// ============================================================================

func TestIntegration_ContextsGetExisting(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupContextsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant with a context
	tempTenant := NewTempTenant(t, client, "contexts get test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "contexts get test project").MustCreate(t).Cleanup(t)
	ctx := NewTempContext(t, client, tempTenant.Name).WithName("gettestctx").WithProject(tempProject.Name).MustCreate(t).Cleanup(t)

	// Set tenant global flag (picked up by MergeWithFlags in PersistentPreRunE)
	tenant = tempTenant.Name

	output, err := executeContextsCommand(t, []string{"get", ctx.Name, "--project", tempProject.Name})

	require.NoError(t, err, "contexts get should succeed")
	assert.Contains(t, output, ctx.Name, "Output should contain context name")

	t.Logf("Contexts get output:\n%s", output)
}

func TestIntegration_ContextsGetNotFound(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupContextsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant without the context we'll try to get
	tempTenant := NewTempTenant(t, client, "contexts get not found test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "contexts not found project").MustCreate(t).Cleanup(t)

	// Set tenant global flag (picked up by MergeWithFlags in PersistentPreRunE)
	tenant = tempTenant.Name

	_, err := executeContextsCommand(t, []string{"get", "nonexistent-context", "--project", tempProject.Name})

	require.Error(t, err, "contexts get for nonexistent context should fail")
	assert.Contains(t, err.Error(), "not found", "Error should mention context not found")
}

// ============================================================================
// Contexts Create Tests
// ============================================================================

func TestIntegration_ContextsCreateWithProject(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupContextsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant and project
	tempTenant := NewTempTenant(t, client, "contexts create project test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "contexts create test project").MustCreate(t).Cleanup(t)

	contextName := fmt.Sprintf("clicreate%d", time.Now().UnixNano())

	// Register cleanup for the context we're about to create
	t.Cleanup(func() {
		_ = client.DeleteContext(context.Background(), tempTenant.Name, tempProject.Name, contextName)
	})

	// Set tenant global flag (picked up by MergeWithFlags in PersistentPreRunE)
	tenant = tempTenant.Name

	output, err := executeContextsCommand(t, []string{"create", contextName, "--project", tempProject.Name})

	require.NoError(t, err, "contexts create --project should succeed")
	assert.Contains(t, output, "created successfully", "Output should confirm creation")
	assert.Contains(t, output, contextName, "Output should contain context name")

	// Verify via API
	contexts, err := izanami.ListContexts(client, context.Background(), tempTenant.Name, tempProject.Name, false, izanami.ParseContexts)
	require.NoError(t, err, "Should list contexts via API")

	found := false
	for _, c := range contexts {
		if c.Name == contextName {
			found = true
			break
		}
	}
	assert.True(t, found, "Created context should be found in list")

	t.Logf("Contexts create output:\n%s", output)
}

func TestIntegration_ContextsCreateWithParent(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupContextsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant with a parent context
	tempTenant := NewTempTenant(t, client, "contexts create nested test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "contexts nested test project").MustCreate(t).Cleanup(t)
	parentCtx := NewTempContext(t, client, tempTenant.Name).WithName("parentfornested").WithProject(tempProject.Name).MustCreate(t).Cleanup(t)

	childName := fmt.Sprintf("childctx%d", time.Now().UnixNano())
	childPath := parentCtx.Name + "/" + childName

	// Register cleanup for the child context
	t.Cleanup(func() {
		_ = client.DeleteContext(context.Background(), tempTenant.Name, tempProject.Name, childPath)
	})

	// Set tenant global flag (picked up by MergeWithFlags in PersistentPreRunE)
	tenant = tempTenant.Name

	output, err := executeContextsCommand(t, []string{"create", childName, "--project", tempProject.Name, "--parent", parentCtx.Name})

	require.NoError(t, err, "contexts create with --parent should succeed")
	assert.Contains(t, output, "created successfully", "Output should confirm creation")

	// Verify via API - list with --all to see nested contexts (searches recursively in children)
	contexts, err := izanami.ListContexts(client, context.Background(), tempTenant.Name, tempProject.Name, true, izanami.ParseContexts)
	require.NoError(t, err, "Should list contexts via API")

	found := findContextByNameRecursive(contexts, childName)
	assert.NotNil(t, found, "Created nested context should be found in list (recursive search)")

	t.Logf("Contexts create with parent output:\n%s", output)
}

func TestIntegration_ContextsCreateProtected(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupContextsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant
	tempTenant := NewTempTenant(t, client, "contexts create protected test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "contexts protected test project").MustCreate(t).Cleanup(t)

	contextName := fmt.Sprintf("protectedctx%d", time.Now().UnixNano())

	// Register cleanup
	t.Cleanup(func() {
		_ = client.DeleteContext(context.Background(), tempTenant.Name, tempProject.Name, contextName)
	})

	// Set tenant global flag (picked up by MergeWithFlags in PersistentPreRunE)
	tenant = tempTenant.Name

	output, err := executeContextsCommand(t, []string{"create", contextName, "--project", tempProject.Name, "--protected"})

	require.NoError(t, err, "contexts create --protected should succeed")

	// Verify via API
	contexts, err := izanami.ListContexts(client, context.Background(), tempTenant.Name, tempProject.Name, false, izanami.ParseContexts)
	require.NoError(t, err, "Should list contexts via API")

	for _, c := range contexts {
		if c.Name == contextName {
			assert.True(t, c.IsProtected, "Created context should be protected")
			break
		}
	}

	t.Logf("Contexts create protected output:\n%s", output)
}

func TestIntegration_ContextsCreateMissingProjectOrGlobal(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupContextsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant
	tempTenant := NewTempTenant(t, client, "contexts create missing flags test").MustCreate(t).Cleanup(t)

	// Set tenant global flag (picked up by MergeWithFlags in PersistentPreRunE)
	tenant = tempTenant.Name

	_, err := executeContextsCommand(t, []string{"create", "some-context"})

	require.Error(t, err, "contexts create without --global or --project should fail")
	assert.Contains(t, err.Error(), "--project or --global", "Error should mention required flags")
}

// ============================================================================
// Contexts Update Tests
// ============================================================================

func TestIntegration_ContextsUpdateProtectedTrue(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupContextsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant with a global context (not protected)
	tempTenant := NewTempTenant(t, client, "contexts update test").MustCreate(t).Cleanup(t)
	ctx := NewTempContext(t, client, tempTenant.Name).WithName("updatetestctx").WithProtected(false).MustCreate(t).Cleanup(t)

	// Set tenant global flag (picked up by MergeWithFlags in PersistentPreRunE)
	tenant = tempTenant.Name

	output, err := executeContextsCommand(t, []string{"update", ctx.Name, "--protected=true"})

	require.NoError(t, err, "contexts update --protected=true should succeed")
	assert.Contains(t, output, "updated successfully", "Output should confirm update")

	// Verify via API
	contexts, err := izanami.ListContexts(client, context.Background(), tempTenant.Name, "", false, izanami.ParseContexts)
	require.NoError(t, err, "Should list contexts via API")

	for _, c := range contexts {
		if c.Name == ctx.Name {
			assert.True(t, c.IsProtected, "Context should now be protected")
			break
		}
	}

	t.Logf("Contexts update output:\n%s", output)
}

func TestIntegration_ContextsUpdateProtectedFalse(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupContextsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant with a protected global context
	tempTenant := NewTempTenant(t, client, "contexts update false test").MustCreate(t).Cleanup(t)
	ctx := NewTempContext(t, client, tempTenant.Name).WithName("updatefalsectx").WithProtected(true).MustCreate(t).Cleanup(t)

	// Set tenant global flag (picked up by MergeWithFlags in PersistentPreRunE)
	tenant = tempTenant.Name

	output, err := executeContextsCommand(t, []string{"update", ctx.Name, "--protected=false"})

	require.NoError(t, err, "contexts update --protected=false should succeed")

	// Verify via API
	contexts, err := izanami.ListContexts(client, context.Background(), tempTenant.Name, "", false, izanami.ParseContexts)
	require.NoError(t, err, "Should list contexts via API")

	for _, c := range contexts {
		if c.Name == ctx.Name {
			assert.False(t, c.IsProtected, "Context should now be unprotected")
			break
		}
	}

	t.Logf("Contexts update protected=false output:\n%s", output)
}

func TestIntegration_ContextsUpdateInvalidProtectedValue(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupContextsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant with a context
	tempTenant := NewTempTenant(t, client, "contexts update invalid test").MustCreate(t).Cleanup(t)
	ctx := NewTempContext(t, client, tempTenant.Name).MustCreate(t).Cleanup(t)

	// Set tenant global flag (picked up by MergeWithFlags in PersistentPreRunE)
	tenant = tempTenant.Name

	_, err := executeContextsCommand(t, []string{"update", ctx.Name, "--protected=invalid"})

	require.Error(t, err, "contexts update with invalid protected value should fail")
	assert.Contains(t, err.Error(), "'true' or 'false'", "Error should mention valid values")
}

func TestIntegration_ContextsUpdateMissingProtectedFlag(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupContextsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant with a context
	tempTenant := NewTempTenant(t, client, "contexts update missing flag test").MustCreate(t).Cleanup(t)
	ctx := NewTempContext(t, client, tempTenant.Name).MustCreate(t).Cleanup(t)

	// Set tenant global flag (picked up by MergeWithFlags in PersistentPreRunE)
	tenant = tempTenant.Name

	_, err := executeContextsCommand(t, []string{"update", ctx.Name})

	require.Error(t, err, "contexts update without --protected should fail")
	assert.Contains(t, err.Error(), "protected", "Error should mention required flag")
}

// ============================================================================
// Contexts Delete Tests
// ============================================================================

func TestIntegration_ContextsDeleteWithForce(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupContextsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant with a context to delete
	tempTenant := NewTempTenant(t, client, "contexts delete test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "contexts delete test project").MustCreate(t).Cleanup(t)
	ctx := NewTempContext(t, client, tempTenant.Name).WithName("deletetestctx").WithProject(tempProject.Name).MustCreate(t)
	// Don't register cleanup - we're deleting it

	// Set tenant global flag (picked up by MergeWithFlags in PersistentPreRunE)
	tenant = tempTenant.Name

	output, err := executeContextsCommand(t, []string{"delete", ctx.Name, "--project", tempProject.Name, "--force"})

	require.NoError(t, err, "contexts delete --force should succeed")
	assert.Contains(t, output, "deleted successfully", "Output should confirm deletion")

	// Verify via API
	contexts, err := izanami.ListContexts(client, context.Background(), tempTenant.Name, tempProject.Name, false, izanami.ParseContexts)
	require.NoError(t, err, "Should list contexts via API")

	for _, c := range contexts {
		assert.NotEqual(t, ctx.Name, c.Name, "Deleted context should not be in list")
	}

	t.Logf("Contexts delete output:\n%s", output)
}

func TestIntegration_ContextsDeleteNestedContext(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupContextsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant with nested contexts
	tempTenant := NewTempTenant(t, client, "contexts delete nested test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "contexts delete nested project").MustCreate(t).Cleanup(t)
	parentCtx := NewTempContext(t, client, tempTenant.Name).WithName("deleteparent").WithProject(tempProject.Name).MustCreate(t).Cleanup(t)
	childCtx := NewTempContext(t, client, tempTenant.Name).WithName("deletechild").WithParent(parentCtx.Name).WithProject(tempProject.Name).MustCreate(t)
	// Don't register cleanup for child - we're deleting it

	// Set tenant global flag (picked up by MergeWithFlags in PersistentPreRunE)
	tenant = tempTenant.Name

	output, err := executeContextsCommand(t, []string{"delete", childCtx.Path, "--project", tempProject.Name, "--force"})

	require.NoError(t, err, "contexts delete nested context should succeed")
	assert.Contains(t, output, "deleted successfully", "Output should confirm deletion")

	t.Logf("Contexts delete nested output:\n%s", output)
}

func TestIntegration_ContextsDeleteWithoutForceAbort(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupContextsTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant with a context
	tempTenant := NewTempTenant(t, client, "contexts delete abort test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "contexts abort test project").MustCreate(t).Cleanup(t)
	ctx := NewTempContext(t, client, tempTenant.Name).WithName("deleteabortctx").WithProject(tempProject.Name).MustCreate(t).Cleanup(t)

	// Set tenant global flag (picked up by MergeWithFlags in PersistentPreRunE)
	tenant = tempTenant.Name

	output, err := executeContextsCommandWithInput(t, []string{"delete", ctx.Name, "--project", tempProject.Name}, "n\n")

	require.NoError(t, err, "contexts delete aborted should not error")
	assert.Contains(t, output, "Cancelled", "Output should indicate deletion was cancelled")

	// Verify context still exists via API
	contexts, err := izanami.ListContexts(client, context.Background(), tempTenant.Name, tempProject.Name, false, izanami.ParseContexts)
	require.NoError(t, err, "Should list contexts via API")

	found := false
	for _, c := range contexts {
		if c.Name == ctx.Name {
			found = true
			break
		}
	}
	assert.True(t, found, "Context should still exist after aborted deletion")

	t.Logf("Contexts delete aborted output:\n%s", output)
}

// ============================================================================
// API Direct Tests (using izanami package)
// ============================================================================

func TestIntegration_APIListContexts(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant with contexts
	tempTenant := NewTempTenant(t, client, "API list contexts test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "API list contexts project").MustCreate(t).Cleanup(t)
	ctx1 := NewTempContext(t, client, tempTenant.Name).WithName("apictx1").WithProject(tempProject.Name).MustCreate(t).Cleanup(t)
	ctx2 := NewTempContext(t, client, tempTenant.Name).WithName("apictx2").WithProject(tempProject.Name).MustCreate(t).Cleanup(t)

	// Test ListContexts API
	contexts, err := izanami.ListContexts(client, context.Background(), tempTenant.Name, tempProject.Name, false, izanami.ParseContexts)
	require.NoError(t, err, "ListContexts API should succeed")

	names := make([]string, 0, len(contexts))
	for _, c := range contexts {
		names = append(names, c.Name)
	}

	assert.Contains(t, names, ctx1.Name, "Should contain first context")
	assert.Contains(t, names, ctx2.Name, "Should contain second context")

	t.Logf("API ListContexts returned %d contexts: %v", len(contexts), names)
}

func TestIntegration_APICreateContext(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant
	tempTenant := NewTempTenant(t, client, "API create context test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "API create context project").MustCreate(t).Cleanup(t)

	contextName := fmt.Sprintf("apicreatectx%d", time.Now().UnixNano())

	// Register cleanup
	t.Cleanup(func() {
		_ = client.DeleteContext(context.Background(), tempTenant.Name, tempProject.Name, contextName)
	})

	// Test CreateContext API
	data := map[string]interface{}{
		"name":      contextName,
		"protected": true,
	}
	err := client.CreateContext(context.Background(), tempTenant.Name, tempProject.Name, contextName, "", data)
	require.NoError(t, err, "CreateContext API should succeed")

	// Verify
	contexts, err := izanami.ListContexts(client, context.Background(), tempTenant.Name, tempProject.Name, false, izanami.ParseContexts)
	require.NoError(t, err, "ListContexts should succeed")

	found := false
	for _, c := range contexts {
		if c.Name == contextName {
			found = true
			assert.True(t, c.IsProtected, "Created context should be protected")
			break
		}
	}
	assert.True(t, found, "Created context should be in list")
}

func TestIntegration_APIUpdateGlobalContext(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant with an unprotected global context
	tempTenant := NewTempTenant(t, client, "API update context test").MustCreate(t).Cleanup(t)
	ctx := NewTempContext(t, client, tempTenant.Name).WithName("apiupdatectx").WithProtected(false).MustCreate(t).Cleanup(t)

	// Verify initial state
	contexts, err := izanami.ListContexts(client, context.Background(), tempTenant.Name, "", false, izanami.ParseContexts)
	require.NoError(t, err)
	for _, c := range contexts {
		if c.Name == ctx.Name {
			assert.False(t, c.IsProtected, "Context should initially not be protected")
			break
		}
	}

	// Test UpdateContext API
	updateData := map[string]interface{}{
		"protected": true,
	}
	err = client.UpdateContext(context.Background(), tempTenant.Name, ctx.Name, updateData)
	require.NoError(t, err, "UpdateContext API should succeed")

	// Verify update
	contexts, err = izanami.ListContexts(client, context.Background(), tempTenant.Name, "", false, izanami.ParseContexts)
	require.NoError(t, err)
	for _, c := range contexts {
		if c.Name == ctx.Name {
			assert.True(t, c.IsProtected, "Context should now be protected")
			break
		}
	}
}

func TestIntegration_APIDeleteContext(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant with a context to delete
	tempTenant := NewTempTenant(t, client, "API delete context test").MustCreate(t).Cleanup(t)
	tempProject := NewTempProject(t, client, tempTenant.Name, "API delete context project").MustCreate(t).Cleanup(t)
	ctx := NewTempContext(t, client, tempTenant.Name).WithName("apideletectx").WithProject(tempProject.Name).MustCreate(t)
	// Don't register cleanup - we're deleting it

	// Test DeleteContext API
	err := client.DeleteContext(context.Background(), tempTenant.Name, tempProject.Name, ctx.Name)
	require.NoError(t, err, "DeleteContext API should succeed")

	// Verify deletion
	contexts, err := izanami.ListContexts(client, context.Background(), tempTenant.Name, tempProject.Name, false, izanami.ParseContexts)
	require.NoError(t, err)

	for _, c := range contexts {
		assert.NotEqual(t, ctx.Name, c.Name, "Deleted context should not be in list")
	}
}
