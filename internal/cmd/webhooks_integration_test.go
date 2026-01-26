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
// TempWebhook - Temporary webhook management for integration tests
// ============================================================================

// TempWebhook manages a temporary webhook for integration tests
type TempWebhook struct {
	ID          string
	Name        string
	URL         string
	Description string
	Tenant      string
	Enabled     bool
	Global      bool
	Features    []string
	Projects    []string
	client      *izanami.AdminClient
	ctx         context.Context
	created     bool
}

// NewTempWebhook creates a new temporary webhook helper with auto-generated unique name
func NewTempWebhook(t *testing.T, client *izanami.AdminClient, tenant string) *TempWebhook {
	t.Helper()
	name := fmt.Sprintf("testwebhook%d", time.Now().UnixNano())
	return &TempWebhook{
		Name:        name,
		URL:         "https://example.com/webhook",
		Description: "Test webhook created by integration test",
		Tenant:      tenant,
		Enabled:     false,
		Global:      true,
		Features:    []string{},
		Projects:    []string{},
		client:      client,
		ctx:         context.Background(),
		created:     false,
	}
}

// WithName sets a custom name for the webhook
func (tw *TempWebhook) WithName(name string) *TempWebhook {
	tw.Name = name
	return tw
}

// WithURL sets a custom URL for the webhook
func (tw *TempWebhook) WithURL(url string) *TempWebhook {
	tw.URL = url
	return tw
}

// WithDescription sets a custom description for the webhook
func (tw *TempWebhook) WithDescription(desc string) *TempWebhook {
	tw.Description = desc
	return tw
}

// WithEnabled sets the enabled state for the webhook
func (tw *TempWebhook) WithEnabled(enabled bool) *TempWebhook {
	tw.Enabled = enabled
	return tw
}

// WithGlobal sets the global state for the webhook
func (tw *TempWebhook) WithGlobal(global bool) *TempWebhook {
	tw.Global = global
	return tw
}

// WithFeatures sets the features for the webhook (mutually exclusive with Global=true)
func (tw *TempWebhook) WithFeatures(features []string) *TempWebhook {
	tw.Features = features
	tw.Global = false
	return tw
}

// WithProjects sets the projects for the webhook (mutually exclusive with Global=true)
func (tw *TempWebhook) WithProjects(projects []string) *TempWebhook {
	tw.Projects = projects
	tw.Global = false
	return tw
}

// Create creates the webhook on the server
func (tw *TempWebhook) Create(t *testing.T) (*izanami.WebhookFull, error) {
	t.Helper()
	data := map[string]interface{}{
		"name":    tw.Name,
		"url":     tw.URL,
		"enabled": tw.Enabled,
		"global":  tw.Global,
	}
	if tw.Description != "" {
		data["description"] = tw.Description
	}
	if len(tw.Features) > 0 {
		data["features"] = tw.Features
	}
	if len(tw.Projects) > 0 {
		data["projects"] = tw.Projects
	}

	result, err := tw.client.CreateWebhook(tw.ctx, tw.Tenant, data)
	if err == nil {
		tw.created = true
		tw.ID = result.ID
		t.Logf("TempWebhook created: %s (ID: %s, tenant: %s)", tw.Name, tw.ID, tw.Tenant)
	}
	return result, err
}

// MustCreate creates the webhook and fails the test on error
func (tw *TempWebhook) MustCreate(t *testing.T) *TempWebhook {
	t.Helper()
	_, err := tw.Create(t)
	require.NoError(t, err, "Failed to create temp webhook %s", tw.Name)
	return tw
}

// Delete removes the webhook from server
func (tw *TempWebhook) Delete(t *testing.T) {
	t.Helper()
	if !tw.created || tw.ID == "" {
		return
	}
	err := tw.client.DeleteWebhook(tw.ctx, tw.Tenant, tw.ID)
	if err != nil {
		t.Logf("Warning: failed to delete temp webhook %s: %v", tw.ID, err)
	} else {
		t.Logf("TempWebhook deleted: %s", tw.ID)
		tw.created = false
	}
}

// Cleanup registers automatic deletion via t.Cleanup()
func (tw *TempWebhook) Cleanup(t *testing.T) *TempWebhook {
	t.Helper()
	t.Cleanup(func() {
		tw.Delete(t)
	})
	return tw
}

// MarkCreated marks the webhook as created (for when creation happens outside TempWebhook)
func (tw *TempWebhook) MarkCreated(id string) *TempWebhook {
	tw.created = true
	tw.ID = id
	return tw
}

// ============================================================================
// Test Setup Helpers
// ============================================================================

// setupWebhooksTest sets up the test environment and logs in
func setupWebhooksTest(t *testing.T, env *IntegrationTestEnv) func() {
	t.Helper()
	env.Login(t)

	// Get JWT token from session
	token := env.GetJwtToken(t)

	// Save original values
	origCfg := cfg
	origOutputFormat := outputFormat
	origTenant := tenant
	origCompactJSON := compactJSON

	// Set up config
	cfg = &izanami.Config{
		BaseURL:  env.BaseURL,
		Username: env.Username,
		JwtToken: token,
		Timeout:  30,
	}
	outputFormat = "table"
	tenant = "" // Will be set per-test
	compactJSON = false

	// Reset webhooks-specific flags to defaults
	webhookURL = ""
	webhookDescription = ""
	webhookFeatures = []string{}
	webhookProjects = []string{}
	webhookContext = ""
	webhookUser = ""
	webhookEnabled = false
	webhookGlobal = false
	webhookHeaders = ""
	webhookBodyTemplate = ""
	webhookData = ""
	webhooksDeleteForce = false

	return func() {
		cfg = origCfg
		outputFormat = origOutputFormat
		tenant = origTenant
		compactJSON = origCompactJSON
		webhookURL = ""
		webhookDescription = ""
		webhookFeatures = []string{}
		webhookProjects = []string{}
		webhookContext = ""
		webhookUser = ""
		webhookEnabled = false
		webhookGlobal = false
		webhookHeaders = ""
		webhookBodyTemplate = ""
		webhookData = ""
		webhooksDeleteForce = false
	}
}

// executeWebhooksCommand executes a webhooks command with proper setup
func executeWebhooksCommand(t *testing.T, args []string) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "iz"}
	cmd.AddCommand(adminCmd)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	adminCmd.SetOut(&buf)
	adminCmd.SetErr(&buf)
	webhooksCmd.SetOut(&buf)
	webhooksCmd.SetErr(&buf)
	webhooksListCmd.SetOut(&buf)
	webhooksListCmd.SetErr(&buf)
	webhooksGetCmd.SetOut(&buf)
	webhooksGetCmd.SetErr(&buf)
	webhooksCreateCmd.SetOut(&buf)
	webhooksCreateCmd.SetErr(&buf)
	webhooksUpdateCmd.SetOut(&buf)
	webhooksUpdateCmd.SetErr(&buf)
	webhooksDeleteCmd.SetOut(&buf)
	webhooksDeleteCmd.SetErr(&buf)
	webhooksUsersCmd.SetOut(&buf)
	webhooksUsersCmd.SetErr(&buf)

	fullArgs := append([]string{"admin", "webhooks"}, args...)
	cmd.SetArgs(fullArgs)
	err := cmd.Execute()

	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	webhooksCmd.SetOut(nil)
	webhooksCmd.SetErr(nil)
	webhooksListCmd.SetOut(nil)
	webhooksListCmd.SetErr(nil)
	webhooksGetCmd.SetOut(nil)
	webhooksGetCmd.SetErr(nil)
	webhooksCreateCmd.SetOut(nil)
	webhooksCreateCmd.SetErr(nil)
	webhooksUpdateCmd.SetOut(nil)
	webhooksUpdateCmd.SetErr(nil)
	webhooksDeleteCmd.SetOut(nil)
	webhooksDeleteCmd.SetErr(nil)
	webhooksUsersCmd.SetOut(nil)
	webhooksUsersCmd.SetErr(nil)

	return buf.String(), err
}

// executeWebhooksCommandWithInput executes a webhooks command with stdin input
func executeWebhooksCommandWithInput(t *testing.T, args []string, input string) (string, error) {
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
	webhooksCmd.SetOut(&buf)
	webhooksCmd.SetErr(&buf)
	webhooksCmd.SetIn(inputBuf)
	webhooksDeleteCmd.SetOut(&buf)
	webhooksDeleteCmd.SetErr(&buf)
	webhooksDeleteCmd.SetIn(inputBuf)

	fullArgs := append([]string{"admin", "webhooks"}, args...)
	cmd.SetArgs(fullArgs)
	err := cmd.Execute()

	adminCmd.SetIn(nil)
	adminCmd.SetOut(nil)
	adminCmd.SetErr(nil)
	webhooksCmd.SetIn(nil)
	webhooksCmd.SetOut(nil)
	webhooksCmd.SetErr(nil)
	webhooksDeleteCmd.SetIn(nil)
	webhooksDeleteCmd.SetOut(nil)
	webhooksDeleteCmd.SetErr(nil)

	return buf.String(), err
}

// ============================================================================
// Webhooks List Tests
// ============================================================================

func TestIntegration_WebhooksListAfterLogin(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Webhooks integration test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	// Create a webhook to ensure we have at least one
	tempWebhook := NewTempWebhook(t, client, tempTenant.Name).MustCreate(t)
	defer tempWebhook.Delete(t)

	output, err := executeWebhooksCommand(t, []string{"list"})
	require.NoError(t, err)

	t.Logf("Webhooks list output:\n%s", output)

	// Should show the created webhook
	assert.Contains(t, output, tempWebhook.Name)
}

func TestIntegration_WebhooksListJSONOutput(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Webhooks JSON test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	outputFormat = "json"

	// Create a webhook
	tempWebhook := NewTempWebhook(t, client, tempTenant.Name).
		WithDescription("JSON test webhook").
		MustCreate(t)
	defer tempWebhook.Delete(t)

	output, err := executeWebhooksCommand(t, []string{"list"})
	require.NoError(t, err)

	t.Logf("Webhooks list JSON output:\n%s", output)

	// Should be valid JSON
	var webhooks []map[string]interface{}
	err = json.Unmarshal([]byte(output), &webhooks)
	require.NoError(t, err, "Output should be valid JSON array")

	// Find our webhook in the response
	var found bool
	for _, wh := range webhooks {
		if wh["name"] == tempWebhook.Name {
			found = true
			break
		}
	}
	assert.True(t, found, "Created webhook should be in the list")
}

func TestIntegration_WebhooksListEmpty(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	// Create a temp tenant for isolation (no webhooks created)
	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Empty webhooks test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	output, err := executeWebhooksCommand(t, []string{"list"})
	require.NoError(t, err)

	t.Logf("Webhooks list output (empty):\n%s", output)

	assert.Contains(t, output, "No webhooks found")
}

func TestIntegration_WebhooksListMissingTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	// Don't set tenant
	tenant = ""

	_, err := executeWebhooksCommand(t, []string{"list"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant is required")
}

// ============================================================================
// Webhooks Get Tests
// ============================================================================

func TestIntegration_WebhooksGetByID(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Webhooks get by ID test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	tempWebhook := NewTempWebhook(t, client, tempTenant.Name).
		WithDescription("Get by ID test webhook").
		MustCreate(t)
	defer tempWebhook.Delete(t)

	output, err := executeWebhooksCommand(t, []string{"get", tempWebhook.ID})
	require.NoError(t, err)

	t.Logf("Webhooks get by ID output:\n%s", output)

	assert.Contains(t, output, tempWebhook.Name)
	assert.Contains(t, output, tempWebhook.URL)
}

func TestIntegration_WebhooksGetByName(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Webhooks get by name test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	tempWebhook := NewTempWebhook(t, client, tempTenant.Name).
		WithDescription("Get by name test webhook").
		MustCreate(t)
	defer tempWebhook.Delete(t)

	output, err := executeWebhooksCommand(t, []string{"get", tempWebhook.Name})
	require.NoError(t, err)

	t.Logf("Webhooks get by name output:\n%s", output)

	assert.Contains(t, output, tempWebhook.Name)
	assert.Contains(t, output, tempWebhook.ID)
}

func TestIntegration_WebhooksGetJSONOutput(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Webhooks get JSON test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	outputFormat = "json"

	tempWebhook := NewTempWebhook(t, client, tempTenant.Name).MustCreate(t)
	defer tempWebhook.Delete(t)

	output, err := executeWebhooksCommand(t, []string{"get", tempWebhook.Name})
	require.NoError(t, err)

	t.Logf("Webhooks get JSON output:\n%s", output)

	// Should be valid JSON
	var webhook map[string]interface{}
	err = json.Unmarshal([]byte(output), &webhook)
	require.NoError(t, err, "Output should be valid JSON")

	assert.Equal(t, tempWebhook.Name, webhook["name"])
	assert.Equal(t, tempWebhook.ID, webhook["id"])
}

func TestIntegration_WebhooksGetNotFound(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Webhooks get not found test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	_, err := executeWebhooksCommand(t, []string{"get", "nonexistent-webhook"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ============================================================================
// Webhooks Create Tests
// ============================================================================

func TestIntegration_WebhooksCreateGlobal(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Webhooks create global test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	webhookURL = "https://example.com/test-hook"
	webhookGlobal = true

	webhookName := fmt.Sprintf("create-test-webhook-%d", time.Now().UnixNano())

	// Track for cleanup
	tempWebhook := NewTempWebhook(t, client, tempTenant.Name).WithName(webhookName)

	output, err := executeWebhooksCommand(t, []string{"create", webhookName})
	require.NoError(t, err)

	t.Logf("Webhooks create output:\n%s", output)

	assert.Contains(t, output, "Webhook created successfully")

	// Verify via API
	ctx := context.Background()
	webhooks, err := izanami.ListWebhooks(client, ctx, tempTenant.Name, izanami.ParseWebhooks)
	require.NoError(t, err)

	var found *izanami.WebhookFull
	for i := range webhooks {
		if webhooks[i].Name == webhookName {
			found = &webhooks[i]
			break
		}
	}
	require.NotNil(t, found, "Webhook should exist after creation")
	assert.Equal(t, "https://example.com/test-hook", found.URL)
	assert.True(t, found.Global)

	// Mark for cleanup
	tempWebhook.MarkCreated(found.ID)
	defer tempWebhook.Delete(t)
}

func TestIntegration_WebhooksCreateJSONOutput(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Webhooks create JSON test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	outputFormat = "json"
	webhookURL = "https://example.com/json-hook"
	webhookGlobal = true

	webhookName := fmt.Sprintf("create-json-webhook-%d", time.Now().UnixNano())
	tempWebhook := NewTempWebhook(t, client, tempTenant.Name).WithName(webhookName)

	output, err := executeWebhooksCommand(t, []string{"create", webhookName})
	require.NoError(t, err)

	t.Logf("Webhooks create JSON output:\n%s", output)

	// Should be valid JSON
	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "Output should be valid JSON")

	// Should have an ID
	id, ok := result["id"].(string)
	require.True(t, ok && id != "", "Should return created webhook ID")

	// Cleanup
	tempWebhook.MarkCreated(id)
	defer tempWebhook.Delete(t)
}

func TestIntegration_WebhooksCreateMissingURL(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Webhooks create missing URL test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	webhookGlobal = true
	// Don't set webhookURL

	_, err := executeWebhooksCommand(t, []string{"create", "test-webhook"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--url is required")
}

func TestIntegration_WebhooksCreateMissingTenant(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	// Don't set tenant
	tenant = ""
	webhookURL = "https://example.com/hook"
	webhookGlobal = true

	_, err := executeWebhooksCommand(t, []string{"create", "test-webhook"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant is required")
}

func TestIntegration_WebhooksCreateWithEnabled(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Webhooks create enabled test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	webhookURL = "https://example.com/enabled-hook"
	webhookGlobal = true
	webhookEnabled = true

	webhookName := fmt.Sprintf("enabled-webhook-%d", time.Now().UnixNano())
	tempWebhook := NewTempWebhook(t, client, tempTenant.Name).WithName(webhookName)

	output, err := executeWebhooksCommand(t, []string{"create", webhookName})
	require.NoError(t, err)

	t.Logf("Webhooks create enabled output:\n%s", output)

	// Verify via API
	ctx := context.Background()
	webhooks, err := izanami.ListWebhooks(client, ctx, tempTenant.Name, izanami.ParseWebhooks)
	require.NoError(t, err)

	var found *izanami.WebhookFull
	for i := range webhooks {
		if webhooks[i].Name == webhookName {
			found = &webhooks[i]
			break
		}
	}
	require.NotNil(t, found)
	assert.True(t, found.Enabled, "Webhook should be enabled")

	tempWebhook.MarkCreated(found.ID)
	defer tempWebhook.Delete(t)
}

// ============================================================================
// Webhooks Update Tests
// ============================================================================

func TestIntegration_WebhooksUpdateByName(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Webhooks update by name test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	tempWebhook := NewTempWebhook(t, client, tempTenant.Name).
		WithEnabled(false).
		MustCreate(t)
	defer tempWebhook.Delete(t)

	// Update enabled flag
	webhookEnabled = true

	output, err := executeWebhooksCommand(t, []string{"update", tempWebhook.Name, "--enabled"})
	require.NoError(t, err)

	t.Logf("Webhooks update output:\n%s", output)

	assert.Contains(t, output, "Webhook updated successfully")

	// Verify via API
	ctx := context.Background()
	webhooks, err := izanami.ListWebhooks(client, ctx, tempTenant.Name, izanami.ParseWebhooks)
	require.NoError(t, err)

	var found *izanami.WebhookFull
	for i := range webhooks {
		if webhooks[i].ID == tempWebhook.ID {
			found = &webhooks[i]
			break
		}
	}
	require.NotNil(t, found)
	assert.True(t, found.Enabled, "Webhook should be enabled after update")
}

func TestIntegration_WebhooksUpdateByID(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Webhooks update by ID test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	tempWebhook := NewTempWebhook(t, client, tempTenant.Name).
		WithURL("https://old-url.com/hook").
		MustCreate(t)
	defer tempWebhook.Delete(t)

	// Update URL
	webhookURL = "https://new-url.com/hook"

	output, err := executeWebhooksCommand(t, []string{"update", tempWebhook.ID, "--url", "https://new-url.com/hook"})
	require.NoError(t, err)

	t.Logf("Webhooks update output:\n%s", output)

	assert.Contains(t, output, "Webhook updated successfully")

	// Verify via API
	ctx := context.Background()
	webhooks, err := izanami.ListWebhooks(client, ctx, tempTenant.Name, izanami.ParseWebhooks)
	require.NoError(t, err)

	var found *izanami.WebhookFull
	for i := range webhooks {
		if webhooks[i].ID == tempWebhook.ID {
			found = &webhooks[i]
			break
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, "https://new-url.com/hook", found.URL)
}

func TestIntegration_WebhooksUpdateNotFound(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Webhooks update not found test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	webhookEnabled = true

	_, err := executeWebhooksCommand(t, []string{"update", "nonexistent-webhook", "--enabled"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ============================================================================
// Webhooks Delete Tests
// ============================================================================

func TestIntegration_WebhooksDeleteWithForce(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Webhooks delete force test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	webhooksDeleteForce = true

	tempWebhook := NewTempWebhook(t, client, tempTenant.Name).MustCreate(t)
	webhookID := tempWebhook.ID
	webhookName := tempWebhook.Name

	output, err := executeWebhooksCommand(t, []string{"delete", webhookName})
	require.NoError(t, err)

	t.Logf("Webhooks delete output:\n%s", output)

	assert.Contains(t, output, "Webhook deleted successfully")

	// Verify via API
	ctx := context.Background()
	webhooks, err := izanami.ListWebhooks(client, ctx, tempTenant.Name, izanami.ParseWebhooks)
	require.NoError(t, err)

	var found bool
	for i := range webhooks {
		if webhooks[i].ID == webhookID {
			found = true
			break
		}
	}
	assert.False(t, found, "Webhook should not exist after deletion")
}

func TestIntegration_WebhooksDeleteByID(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Webhooks delete by ID test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	webhooksDeleteForce = true

	tempWebhook := NewTempWebhook(t, client, tempTenant.Name).MustCreate(t)
	webhookID := tempWebhook.ID

	output, err := executeWebhooksCommand(t, []string{"delete", webhookID})
	require.NoError(t, err)

	t.Logf("Webhooks delete by ID output:\n%s", output)

	assert.Contains(t, output, "Webhook deleted successfully")
}

func TestIntegration_WebhooksDeleteWithConfirmation(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Webhooks delete confirmation test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	webhooksDeleteForce = false

	tempWebhook := NewTempWebhook(t, client, tempTenant.Name).MustCreate(t)
	webhookID := tempWebhook.ID

	// User confirms deletion
	output, err := executeWebhooksCommandWithInput(t, []string{"delete", tempWebhook.Name}, "y\n")
	require.NoError(t, err)

	t.Logf("Webhooks delete with confirmation output:\n%s", output)

	assert.Contains(t, output, "Webhook deleted successfully")

	// Verify via API
	ctx := context.Background()
	webhooks, err := izanami.ListWebhooks(client, ctx, tempTenant.Name, izanami.ParseWebhooks)
	require.NoError(t, err)

	var found bool
	for i := range webhooks {
		if webhooks[i].ID == webhookID {
			found = true
			break
		}
	}
	assert.False(t, found, "Webhook should not exist after deletion")
}

func TestIntegration_WebhooksDeleteCancelled(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Webhooks delete cancelled test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	webhooksDeleteForce = false

	tempWebhook := NewTempWebhook(t, client, tempTenant.Name).MustCreate(t)
	defer tempWebhook.Delete(t)

	// User cancels deletion
	output, err := executeWebhooksCommandWithInput(t, []string{"delete", tempWebhook.Name}, "n\n")
	require.NoError(t, err)

	t.Logf("Webhooks delete cancelled output:\n%s", output)

	assert.Contains(t, output, "Cancelled")

	// Verify webhook still exists via API
	ctx := context.Background()
	webhooks, err := izanami.ListWebhooks(client, ctx, tempTenant.Name, izanami.ParseWebhooks)
	require.NoError(t, err)

	var found bool
	for i := range webhooks {
		if webhooks[i].ID == tempWebhook.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "Webhook should still exist after cancellation")
}

func TestIntegration_WebhooksDeleteNotFound(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Webhooks delete not found test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	webhooksDeleteForce = true

	_, err := executeWebhooksCommand(t, []string{"delete", "nonexistent-webhook"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ============================================================================
// Webhooks Users Tests
// ============================================================================

func TestIntegration_WebhooksUsersListByName(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Webhooks users test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	tempWebhook := NewTempWebhook(t, client, tempTenant.Name).MustCreate(t)
	defer tempWebhook.Delete(t)

	output, err := executeWebhooksCommand(t, []string{"users", tempWebhook.Name})
	require.NoError(t, err)

	t.Logf("Webhooks users output:\n%s", output)

	// Should show at least the admin user
	// The output should contain either user info or "No users found"
	assert.True(t, len(output) > 0)
}

func TestIntegration_WebhooksUsersJSONOutput(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Webhooks users JSON test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name
	outputFormat = "json"

	tempWebhook := NewTempWebhook(t, client, tempTenant.Name).MustCreate(t)
	defer tempWebhook.Delete(t)

	output, err := executeWebhooksCommand(t, []string{"users", tempWebhook.Name})
	require.NoError(t, err)

	t.Logf("Webhooks users JSON output:\n%s", output)

	// Should be valid JSON array
	var users []map[string]interface{}
	err = json.Unmarshal([]byte(output), &users)
	require.NoError(t, err, "Output should be valid JSON array")
}

func TestIntegration_WebhooksUsersNotFound(t *testing.T) {
	env := setupIntegrationTest(t)
	cleanup := setupWebhooksTest(t, env)
	defer cleanup()

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "Webhooks users not found test").MustCreate(t)
	defer tempTenant.Delete(t)

	tenant = tempTenant.Name

	_, err := executeWebhooksCommand(t, []string{"users", "nonexistent-webhook"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ============================================================================
// API Function Tests
// ============================================================================

func TestIntegration_APIListWebhooks(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "API list webhooks test").MustCreate(t)
	defer tempTenant.Delete(t)

	// Create test webhook
	tempWebhook := NewTempWebhook(t, client, tempTenant.Name).
		WithDescription("API test webhook").
		MustCreate(t)
	defer tempWebhook.Delete(t)

	// List webhooks
	ctx := context.Background()
	webhooks, err := izanami.ListWebhooks(client, ctx, tempTenant.Name, izanami.ParseWebhooks)
	require.NoError(t, err)

	t.Logf("Found %d webhooks", len(webhooks))

	// Should find our webhook
	var found bool
	for _, wh := range webhooks {
		if wh.ID == tempWebhook.ID {
			found = true
			assert.Equal(t, tempWebhook.Name, wh.Name)
			assert.Equal(t, "API test webhook", wh.Description)
			break
		}
	}
	assert.True(t, found, "Created webhook should be in the list")
}

func TestIntegration_APICreateWebhook(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "API create webhook test").MustCreate(t)
	defer tempTenant.Delete(t)

	webhookName := fmt.Sprintf("api-create-webhook-%d", time.Now().UnixNano())
	webhookData := map[string]interface{}{
		"name":        webhookName,
		"url":         "https://api-test.example.com/hook",
		"description": "Created via API test",
		"enabled":     true,
		"global":      true,
	}

	ctx := context.Background()
	result, err := client.CreateWebhook(ctx, tempTenant.Name, webhookData)
	require.NoError(t, err)

	t.Logf("Created webhook: %s (ID: %s)", webhookName, result.ID)

	assert.NotEmpty(t, result.ID)

	// Cleanup
	err = client.DeleteWebhook(ctx, tempTenant.Name, result.ID)
	require.NoError(t, err)
}

func TestIntegration_APIUpdateWebhook(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "API update webhook test").MustCreate(t)
	defer tempTenant.Delete(t)

	// Create webhook
	tempWebhook := NewTempWebhook(t, client, tempTenant.Name).
		WithEnabled(false).
		MustCreate(t)
	defer tempWebhook.Delete(t)

	// Update webhook
	ctx := context.Background()
	updateData := map[string]interface{}{
		"name":    tempWebhook.Name,
		"url":     tempWebhook.URL,
		"enabled": true, // Change from false to true
		"global":  true,
	}

	err := client.UpdateWebhook(ctx, tempTenant.Name, tempWebhook.ID, updateData)
	require.NoError(t, err)

	// Verify update
	webhooks, err := izanami.ListWebhooks(client, ctx, tempTenant.Name, izanami.ParseWebhooks)
	require.NoError(t, err)

	var found *izanami.WebhookFull
	for i := range webhooks {
		if webhooks[i].ID == tempWebhook.ID {
			found = &webhooks[i]
			break
		}
	}
	require.NotNil(t, found)
	assert.True(t, found.Enabled, "Webhook should be enabled after update")
}

func TestIntegration_APIDeleteWebhook(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "API delete webhook test").MustCreate(t)
	defer tempTenant.Delete(t)

	// Create webhook
	tempWebhook := NewTempWebhook(t, client, tempTenant.Name).MustCreate(t)

	// Delete webhook
	ctx := context.Background()
	err := client.DeleteWebhook(ctx, tempTenant.Name, tempWebhook.ID)
	require.NoError(t, err)

	// Verify deletion
	webhooks, err := izanami.ListWebhooks(client, ctx, tempTenant.Name, izanami.ParseWebhooks)
	require.NoError(t, err)

	var found bool
	for _, wh := range webhooks {
		if wh.ID == tempWebhook.ID {
			found = true
			break
		}
	}
	assert.False(t, found, "Webhook should not exist after deletion")
}

func TestIntegration_APIListWebhookUsers(t *testing.T) {
	env := setupIntegrationTest(t)
	env.Login(t)

	client := env.NewAuthenticatedClient(t)
	tempTenant := NewTempTenant(t, client, "API list webhook users test").MustCreate(t)
	defer tempTenant.Delete(t)

	// Create webhook
	tempWebhook := NewTempWebhook(t, client, tempTenant.Name).MustCreate(t)
	defer tempWebhook.Delete(t)

	// List webhook users
	ctx := context.Background()
	users, err := izanami.ListWebhookUsers(client, ctx, tempTenant.Name, tempWebhook.ID, izanami.ParseWebhookUsers)
	require.NoError(t, err)

	t.Logf("Found %d users with webhook rights", len(users))

	// Should have at least the admin user
	assert.GreaterOrEqual(t, len(users), 1, "Should have at least one user with webhook rights")
}
