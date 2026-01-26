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
// TempTag - Temporary tag management for integration tests
// ============================================================================

// TempTag manages a temporary tag for integration tests
type TempTag struct {
	Name        string
	Description string
	Tenant      string
	client      *izanami.AdminClient
	ctx         context.Context
	created     bool
}

// NewTempTag creates a new temporary tag helper with auto-generated unique name
func NewTempTag(t *testing.T, client *izanami.AdminClient, tenant string) *TempTag {
	t.Helper()
	name := fmt.Sprintf("testtag%d", time.Now().UnixNano())
	return &TempTag{
		Name:        name,
		Description: "Test tag created by integration test",
		Tenant:      tenant,
		client:      client,
		ctx:         context.Background(),
		created:     false,
	}
}

// WithName sets a custom name for the tag
func (tt *TempTag) WithName(name string) *TempTag {
	tt.Name = name
	return tt
}

// WithDescription sets a custom description for the tag
func (tt *TempTag) WithDescription(desc string) *TempTag {
	tt.Description = desc
	return tt
}

// Create creates the tag on the server
func (tt *TempTag) Create(t *testing.T) error {
	t.Helper()
	data := map[string]interface{}{
		"name":        tt.Name,
		"description": tt.Description,
	}
	err := tt.client.CreateTag(tt.ctx, tt.Tenant, data)
	if err == nil {
		tt.created = true
		t.Logf("TempTag created: %s (tenant: %s)", tt.Name, tt.Tenant)
	}
	return err
}

// MustCreate creates the tag and fails the test on error
func (tt *TempTag) MustCreate(t *testing.T) *TempTag {
	t.Helper()
	err := tt.Create(t)
	require.NoError(t, err, "Failed to create temp tag %s", tt.Name)
	return tt
}

// Delete removes the tag from server
func (tt *TempTag) Delete(t *testing.T) {
	t.Helper()
	if !tt.created {
		return
	}
	err := tt.client.DeleteTag(tt.ctx, tt.Tenant, tt.Name)
	if err != nil {
		t.Logf("Warning: failed to delete temp tag %s: %v", tt.Name, err)
	} else {
		t.Logf("TempTag deleted: %s", tt.Name)
		tt.created = false
	}
}

// Cleanup registers automatic deletion via t.Cleanup()
func (tt *TempTag) Cleanup(t *testing.T) *TempTag {
	t.Helper()
	t.Cleanup(func() {
		tt.Delete(t)
	})
	return tt
}

// MarkCreated marks the tag as created (for when creation happens outside TempTag)
func (tt *TempTag) MarkCreated() *TempTag {
	tt.created = true
	return tt
}

// ============================================================================
// Test Setup Helpers
// ============================================================================

// setupTagsTest sets up the test environment and logs in
func setupTagsTest(t *testing.T, env *IntegrationTestEnv) func() {
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

	// Reset tag-specific flags to defaults
	tagDesc = ""
	tagData = ""
	tagsDeleteForce = false

	return func() {
		cfg = origCfg
		outputFormat = origOutputFormat
		tenant = origTenant
		tagDesc = ""
		tagData = ""
		tagsDeleteForce = false
	}
}

// executeTagsCommand executes a tags command with proper setup
func executeTagsCommand(t *testing.T, args []string) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	adminTagsCmd.SetOut(&buf)
	adminTagsCmd.SetErr(&buf)
	adminTagsListCmd.SetOut(&buf)
	adminTagsListCmd.SetErr(&buf)
	adminTagsGetCmd.SetOut(&buf)
	adminTagsGetCmd.SetErr(&buf)
	adminTagsCreateCmd.SetOut(&buf)
	adminTagsCreateCmd.SetErr(&buf)
	adminTagsDeleteCmd.SetOut(&buf)
	adminTagsDeleteCmd.SetErr(&buf)

	fullArgs := append([]string{"admin", "tags"}, args...)
	cmd.SetArgs(fullArgs)
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminTagsCmd.SetOut(nil)
	adminTagsCmd.SetErr(nil)
	adminTagsListCmd.SetOut(nil)
	adminTagsListCmd.SetErr(nil)
	adminTagsGetCmd.SetOut(nil)
	adminTagsGetCmd.SetErr(nil)
	adminTagsCreateCmd.SetOut(nil)
	adminTagsCreateCmd.SetErr(nil)
	adminTagsDeleteCmd.SetOut(nil)
	adminTagsDeleteCmd.SetErr(nil)

	return buf.String(), err
}

// executeTagsCommandWithInput executes a tags command with stdin input
func executeTagsCommandWithInput(t *testing.T, args []string, input string) (string, error) {
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
	adminTagsCmd.SetOut(&buf)
	adminTagsCmd.SetErr(&buf)
	adminTagsCmd.SetIn(inputBuf)
	adminTagsDeleteCmd.SetOut(&buf)
	adminTagsDeleteCmd.SetErr(&buf)
	adminTagsDeleteCmd.SetIn(inputBuf)

	fullArgs := append([]string{"admin", "tags"}, args...)
	cmd.SetArgs(fullArgs)
	err := cmd.Execute()

	adminCmd.SetIn(nil)
	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	adminTagsCmd.SetIn(nil)
	adminTagsCmd.SetOut(nil)
	adminTagsCmd.SetErr(nil)
	adminTagsDeleteCmd.SetIn(nil)
	adminTagsDeleteCmd.SetOut(nil)
	adminTagsDeleteCmd.SetErr(nil)

	return buf.String(), err
}

// ============================================================================
// Tags List Tests
// ============================================================================

func TestIntegration_TagsListAfterLogin(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupTagsTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Tags integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	// Create a tag to ensure we have at least one
	tempTag := NewTempTag(t, client, tempTenant.Name).MustCreate(t)
	defer tempTag.Delete(t)

	output, err := executeTagsCommand(t, []string{"list"})
	require.NoError(t, err)

	t.Logf("Tags list output:\n%s", output)

	// Should show the created tag
	assert.Contains(t, output, tempTag.Name)
}

func TestIntegration_TagsListJSONOutput(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupTagsTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Tags JSON test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	outputFormat = "json"

	// Create a tag
	tempTag := NewTempTag(t, client, tempTenant.Name).
		WithDescription("JSON test tag").
		MustCreate(t)
	defer tempTag.Delete(t)

	output, err := executeTagsCommand(t, []string{"list"})
	require.NoError(t, err)

	t.Logf("Tags list JSON output:\n%s", output)

	// Should be valid JSON
	var tags []map[string]interface{}
	err = json.Unmarshal([]byte(output), &tags)
	require.NoError(t, err, "Output should be valid JSON array")

	// Find our tag in the response
	var found bool
	for _, tag := range tags {
		if tag["name"] == tempTag.Name {
			found = true
			assert.Equal(t, "JSON test tag", tag["description"])
			break
		}
	}
	assert.True(t, found, "Created tag should be in the list")
}

func TestIntegration_TagsListMissingTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupTagsTest(t, env)
	defer cleanup()

	// Don't set tenant
	tenant = ""

	_, err := executeTagsCommand(t, []string{"list"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant is required")
}

// ============================================================================
// Tags Get Tests
// ============================================================================

func TestIntegration_TagsGetExisting(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupTagsTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Tags integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	// Create a tag
	tempTag := NewTempTag(t, client, tempTenant.Name).
		WithDescription("Tag to get").
		MustCreate(t)
	defer tempTag.Delete(t)

	output, err := executeTagsCommand(t, []string{"get", tempTag.Name})
	require.NoError(t, err)

	t.Logf("Tags get output:\n%s", output)

	// Should show the tag details
	assert.Contains(t, output, tempTag.Name)
}

func TestIntegration_TagsGetJSONOutput(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupTagsTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Tags integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	outputFormat = "json"

	// Create a tag
	tempTag := NewTempTag(t, client, tempTenant.Name).
		WithDescription("Get JSON test").
		MustCreate(t)
	defer tempTag.Delete(t)

	output, err := executeTagsCommand(t, []string{"get", tempTag.Name})
	require.NoError(t, err)

	t.Logf("Tags get JSON output:\n%s", output)

	// Should be valid JSON
	var tag map[string]interface{}
	err = json.Unmarshal([]byte(output), &tag)
	require.NoError(t, err, "Output should be valid JSON object")

	assert.Equal(t, tempTag.Name, tag["name"])
	assert.Equal(t, "Get JSON test", tag["description"])
}

func TestIntegration_TagsGetNotFound(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupTagsTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Tags integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	_, err := executeTagsCommand(t, []string{"get", "nonexistent-tag-12345"})
	require.Error(t, err)
	// Should get a 404 or "not found" error
}

func TestIntegration_TagsGetMissingArg(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupTagsTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Tags integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	_, err := executeTagsCommand(t, []string{"get"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

// ============================================================================
// Tags Create Tests
// ============================================================================

func TestIntegration_TagsCreateBasic(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupTagsTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Tags integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	tagName := fmt.Sprintf("clitag%d", time.Now().UnixNano())

	// Create a TempTag to track for cleanup (but don't create it yet)
	tempTag := NewTempTag(t, client, tempTenant.Name).WithName(tagName)
	defer tempTag.Delete(t)

	output, err := executeTagsCommand(t, []string{"create", tagName})
	require.NoError(t, err)
	tempTag.MarkCreated() // Mark as created for cleanup

	t.Logf("Tags create output:\n%s", output)
	assert.Contains(t, output, "Tag created successfully")
	assert.Contains(t, output, tagName)

	// Verify via API
	ctx := context.Background()
	tag, err := izanami.GetTag(client, ctx, tempTenant.Name, tagName, izanami.ParseTag)
	require.NoError(t, err)
	assert.Equal(t, tagName, tag.Name)
}

func TestIntegration_TagsCreateWithDescription(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupTagsTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Tags integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	tagDesc = "My custom description"

	tagName := fmt.Sprintf("clitag%d", time.Now().UnixNano())

	// Create a TempTag to track for cleanup
	tempTag := NewTempTag(t, client, tempTenant.Name).WithName(tagName)
	defer tempTag.Delete(t)

	output, err := executeTagsCommand(t, []string{"create", tagName})
	require.NoError(t, err)
	tempTag.MarkCreated()

	t.Logf("Tags create with description output:\n%s", output)
	assert.Contains(t, output, "Tag created successfully")

	// Verify via API
	ctx := context.Background()
	tag, err := izanami.GetTag(client, ctx, tempTenant.Name, tagName, izanami.ParseTag)
	require.NoError(t, err)
	assert.Equal(t, tagName, tag.Name)
	assert.Equal(t, "My custom description", tag.Description)
}

func TestIntegration_TagsCreateWithJSONData(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupTagsTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Tags integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	tagName := fmt.Sprintf("clitag%d", time.Now().UnixNano())
	jsonData := fmt.Sprintf(`{"name":"%s","description":"JSON data description"}`, tagName)

	// Create a TempTag to track for cleanup
	tempTag := NewTempTag(t, client, tempTenant.Name).WithName(tagName)
	defer tempTag.Delete(t)

	// Pass --data flag explicitly (setting tagData variable doesn't mark flag as "changed")
	output, err := executeTagsCommand(t, []string{"create", tagName, "--data", jsonData})
	require.NoError(t, err)
	tempTag.MarkCreated()

	t.Logf("Tags create with JSON data output:\n%s", output)
	assert.Contains(t, output, "Tag created successfully")

	// Verify via API
	ctx := context.Background()
	tag, err := izanami.GetTag(client, ctx, tempTenant.Name, tagName, izanami.ParseTag)
	require.NoError(t, err)
	assert.Equal(t, tagName, tag.Name)
	assert.Equal(t, "JSON data description", tag.Description)
}

func TestIntegration_TagsCreateDuplicate(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupTagsTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Tags integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	// Create a tag first
	tempTag := NewTempTag(t, client, tempTenant.Name).MustCreate(t)
	defer tempTag.Delete(t)

	// Try to create a tag with the same name
	_, err := executeTagsCommand(t, []string{"create", tempTag.Name})
	require.Error(t, err)
	// Should get a conflict/duplicate error
}

func TestIntegration_TagsCreateMissingTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupTagsTest(t, env)
	defer cleanup()

	// Don't set tenant
	tenant = ""

	_, err := executeTagsCommand(t, []string{"create", "some-tag"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant is required")
}

// ============================================================================
// Tags Delete Tests
// ============================================================================

func TestIntegration_TagsDeleteWithForce(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupTagsTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Tags integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	tagsDeleteForce = true

	// Create a tag to delete
	tempTag := NewTempTag(t, client, tempTenant.Name).MustCreate(t)
	// Don't defer delete - we're deleting it in the test

	output, err := executeTagsCommand(t, []string{"delete", tempTag.Name})
	require.NoError(t, err)

	t.Logf("Tags delete with force output:\n%s", output)
	assert.Contains(t, output, "Tag deleted successfully")
	assert.Contains(t, output, tempTag.Name)

	// Verify tag no longer exists
	ctx := context.Background()
	_, err = izanami.GetTag(client, ctx, tempTenant.Name, tempTag.Name, izanami.ParseTag)
	require.Error(t, err, "Tag should no longer exist")
}

func TestIntegration_TagsDeleteWithConfirmation(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupTagsTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Tags integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	tagsDeleteForce = false

	// Create a tag to delete
	tempTag := NewTempTag(t, client, tempTenant.Name).MustCreate(t)
	// Don't defer delete - we're deleting it in the test

	// User types "y" to confirm
	output, err := executeTagsCommandWithInput(t, []string{"delete", tempTag.Name}, "y\n")
	require.NoError(t, err)

	t.Logf("Tags delete with confirmation output:\n%s", output)
	assert.Contains(t, output, "Tag deleted successfully")

	// Verify tag no longer exists
	ctx := context.Background()
	_, err = izanami.GetTag(client, ctx, tempTenant.Name, tempTag.Name, izanami.ParseTag)
	require.Error(t, err, "Tag should no longer exist")
}

func TestIntegration_TagsDeleteCancelled(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupTagsTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Tags integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	tagsDeleteForce = false

	// Create a tag
	tempTag := NewTempTag(t, client, tempTenant.Name).MustCreate(t)
	defer tempTag.Delete(t) // Will still exist after cancelled delete

	// User types "n" to cancel
	output, err := executeTagsCommandWithInput(t, []string{"delete", tempTag.Name}, "n\n")
	require.NoError(t, err)

	t.Logf("Tags delete cancelled output:\n%s", output)
	assert.Contains(t, output, "Cancelled")

	// Verify tag still exists
	ctx := context.Background()
	tag, err := izanami.GetTag(client, ctx, tempTenant.Name, tempTag.Name, izanami.ParseTag)
	require.NoError(t, err, "Tag should still exist after cancelled delete")
	assert.Equal(t, tempTag.Name, tag.Name)
}

func TestIntegration_TagsDeleteNotFound(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupTagsTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Tags integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	tagsDeleteForce = true

	_, err := executeTagsCommand(t, []string{"delete", "nonexistent-tag-12345"})
	require.Error(t, err)
	// Should get a 404 or "not found" error
}

func TestIntegration_TagsDeleteMissingArg(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupTagsTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Tags integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	_, err := executeTagsCommand(t, []string{"delete"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

// ============================================================================
// API Direct Tests
// ============================================================================

func TestIntegration_APIListTags(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant for isolation
	tempTenant := NewTempTenant(t, client, "Tags integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	// Create some tags
	tempTag1 := NewTempTag(t, client, tempTenant.Name).WithDescription("First tag").MustCreate(t)
	defer tempTag1.Delete(t)

	tempTag2 := NewTempTag(t, client, tempTenant.Name).WithDescription("Second tag").MustCreate(t)
	defer tempTag2.Delete(t)

	// List tags via API
	ctx := context.Background()
	tags, err := izanami.ListTags(client, ctx, tempTenant.Name, izanami.ParseTags)
	require.NoError(t, err)

	t.Logf("API listed %d tags", len(tags))

	// Should contain both tags
	var found1, found2 bool
	for _, tag := range tags {
		if tag.Name == tempTag1.Name {
			found1 = true
			assert.Equal(t, "First tag", tag.Description)
		}
		if tag.Name == tempTag2.Name {
			found2 = true
			assert.Equal(t, "Second tag", tag.Description)
		}
	}
	assert.True(t, found1, "First tag should be in list")
	assert.True(t, found2, "Second tag should be in list")
}

func TestIntegration_APICreateTag(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant for isolation
	tempTenant := NewTempTenant(t, client, "Tags integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tagName := fmt.Sprintf("apitag%d", time.Now().UnixNano())

	// Create tag via API
	ctx := context.Background()
	err := client.CreateTag(ctx, tempTenant.Name, map[string]interface{}{
		"name":        tagName,
		"description": "Created via API",
	})
	require.NoError(t, err)

	// Cleanup
	defer func() {
		_ = client.DeleteTag(ctx, tempTenant.Name, tagName)
	}()

	// Verify it exists
	tag, err := izanami.GetTag(client, ctx, tempTenant.Name, tagName, izanami.ParseTag)
	require.NoError(t, err)
	assert.Equal(t, tagName, tag.Name)
	assert.Equal(t, "Created via API", tag.Description)
}

func TestIntegration_APIDeleteTag(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)

	// Create a temp tenant for isolation
	tempTenant := NewTempTenant(t, client, "Tags integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	// Create a tag to delete
	tempTag := NewTempTag(t, client, tempTenant.Name).MustCreate(t)
	// Don't defer delete - we're testing deletion

	// Delete via API
	ctx := context.Background()
	err := client.DeleteTag(ctx, tempTenant.Name, tempTag.Name)
	require.NoError(t, err)

	// Verify it no longer exists
	_, err = izanami.GetTag(client, ctx, tempTenant.Name, tempTag.Name, izanami.ParseTag)
	require.Error(t, err, "Tag should no longer exist after deletion")
}
