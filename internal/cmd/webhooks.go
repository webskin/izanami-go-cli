package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/errors"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"github.com/webskin/izanami-go-cli/internal/output"
)

var (
	webhookURL          string
	webhookDescription  string
	webhookFeatures     []string
	webhookProjects     []string
	webhookContext      string
	webhookUser         string
	webhookEnabled      bool
	webhookGlobal       bool
	webhookHeaders      string
	webhookBodyTemplate string
	webhookData         string
	webhooksDeleteForce bool
)

// webhooksCmd represents the webhooks command
var webhooksCmd = &cobra.Command{
	Use:   "webhooks",
	Short: "Manage webhooks",
	Long: `Manage webhooks for event notifications.

Webhooks allow Izanami to notify external services when feature flags change.
Each webhook can be configured to trigger on specific features, projects, or contexts.

Examples:
  # List all webhooks
  iz admin webhooks list --tenant my-tenant

  # Create a new webhook
  iz admin webhooks create my-webhook --url https://example.com/hook --tenant my-tenant

  # Get webhook details
  iz admin webhooks get <webhook-id> --tenant my-tenant

  # Delete a webhook
  iz admin webhooks delete <webhook-id> --tenant my-tenant`,
}

// webhooksListCmd lists webhooks
var webhooksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all webhooks in a tenant",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.Tenant == "" {
			return fmt.Errorf(errors.MsgTenantRequired)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()

		// For JSON output, use Identity mapper to get raw JSON
		if outputFormat == "json" {
			raw, err := izanami.ListWebhooks(client, ctx, cfg.Tenant, izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseWebhooks mapper
		webhooks, err := izanami.ListWebhooks(client, ctx, cfg.Tenant, izanami.ParseWebhooks)
		if err != nil {
			return err
		}

		if len(webhooks) == 0 {
			fmt.Fprintln(cmd.OutOrStderr(), "No webhooks found")
			return nil
		}

		return output.PrintTo(cmd.OutOrStdout(), webhooks, output.Format(outputFormat))
	},
}

// webhooksGetCmd gets a specific webhook by filtering from the list
// Note: Izanami API doesn't have a GET endpoint for single webhooks, so we fetch all and filter
var webhooksGetCmd = &cobra.Command{
	Use:   "get <webhook-id-or-name>",
	Short: "Get details of a specific webhook by ID or name",
	Long: `Get details of a specific webhook by ID or name.

The webhook can be identified by its UUID or by name. When using a name,
the command fetches all webhooks and filters by name.

Examples:
  # Get by UUID
  iz admin webhooks get 550e8400-e29b-41d4-a716-446655440000 --tenant my-tenant

  # Get by name
  iz admin webhooks get my-webhook --tenant my-tenant

  # Output as JSON
  iz admin webhooks get my-webhook --tenant my-tenant -o json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		webhookIDOrName := args[0]

		if cfg.Tenant == "" {
			return fmt.Errorf(errors.MsgTenantRequired)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()

		// Fetch all webhooks and filter by ID or name
		webhooks, err := izanami.ListWebhooks(client, ctx, cfg.Tenant, izanami.ParseWebhooks)
		if err != nil {
			return err
		}

		// Find the webhook with matching ID or name
		var found *izanami.WebhookFull
		for i := range webhooks {
			if webhooks[i].ID == webhookIDOrName || webhooks[i].Name == webhookIDOrName {
				found = &webhooks[i]
				break
			}
		}

		if found == nil {
			return fmt.Errorf("webhook '%s' not found", webhookIDOrName)
		}

		// For JSON output
		if outputFormat == "json" {
			encoder := json.NewEncoder(cmd.OutOrStdout())
			if !compactJSON {
				encoder.SetIndent("", "  ")
			}
			return encoder.Encode(found)
		}

		return output.PrintTo(cmd.OutOrStdout(), found, output.Format(outputFormat))
	},
}

// webhooksCreateCmd creates a new webhook
var webhooksCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new webhook",
	Long: `Create a new webhook for event notifications.

A webhook will be triggered when feature flags matching its configuration change.

Examples:
  # Create a basic webhook
  iz admin webhooks create my-webhook --url https://example.com/hook --tenant my-tenant

  # Create an enabled webhook for specific features
  iz admin webhooks create my-webhook --url https://... --features feat1,feat2 --enabled --tenant my-tenant

  # Create with custom headers
  iz admin webhooks create my-webhook --url https://... --headers '{"Authorization":"Bearer token"}' --tenant my-tenant

  # Create from JSON file
  iz admin webhooks create my-webhook --data @webhook.json --tenant my-tenant`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if cfg.Tenant == "" {
			return fmt.Errorf(errors.MsgTenantRequired)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		var webhookPayload interface{}

		// If --data is provided, use it as the webhook payload
		if cmd.Flags().Changed("data") {
			if err := parseJSONData(webhookData, &webhookPayload); err != nil {
				return fmt.Errorf("invalid JSON data: %w", err)
			}
		} else {
			// Build webhook data from flags
			if webhookURL == "" {
				return fmt.Errorf("--url is required when not using --data")
			}

			data := map[string]interface{}{
				"name":    name,
				"url":     webhookURL,
				"enabled": webhookEnabled,
				"global":  webhookGlobal,
			}

			if webhookDescription != "" {
				data["description"] = webhookDescription
			}
			if len(webhookFeatures) > 0 {
				data["features"] = webhookFeatures
			}
			if len(webhookProjects) > 0 {
				data["projects"] = webhookProjects
			}
			if webhookContext != "" {
				data["context"] = webhookContext
			}
			if webhookUser != "" {
				data["user"] = webhookUser
			}
			if webhookBodyTemplate != "" {
				data["bodyTemplate"] = webhookBodyTemplate
			}
			if webhookHeaders != "" {
				var headers map[string]string
				if err := json.Unmarshal([]byte(webhookHeaders), &headers); err != nil {
					return fmt.Errorf("invalid headers JSON: %w", err)
				}
				data["headers"] = headers
			}

			webhookPayload = data
		}

		ctx := context.Background()
		result, err := client.CreateWebhook(ctx, cfg.Tenant, webhookPayload)
		if err != nil {
			return err
		}

		// Print the result
		if output.Format(outputFormat) == output.JSON {
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(result)
		}

		// For table output, show important info
		fmt.Fprintf(cmd.OutOrStderr(), "✅ Webhook created successfully\n\n")
		fmt.Fprintf(cmd.OutOrStderr(), "ID:      %s\n", result.ID)
		fmt.Fprintf(cmd.OutOrStderr(), "Name:    %s\n", result.Name)
		fmt.Fprintf(cmd.OutOrStderr(), "URL:     %s\n", result.URL)
		fmt.Fprintf(cmd.OutOrStderr(), "Enabled: %t\n", result.Enabled)
		fmt.Fprintf(cmd.OutOrStderr(), "Global:  %t\n", result.Global)

		return nil
	},
}

// webhooksUpdateCmd updates a webhook
var webhooksUpdateCmd = &cobra.Command{
	Use:   "update <webhook-id-or-name>",
	Short: "Update an existing webhook by ID or name",
	Long: `Update an existing webhook configuration.

The webhook can be identified by its UUID or by name. The API requires a full
update, so the current webhook is fetched first and your changes are merged
with it before sending. This allows partial updates via flags.

Examples:
  # Disable a webhook by name
  iz admin webhooks update my-webhook --enabled=false --tenant my-tenant

  # Update the URL by ID
  iz admin webhooks update 550e8400-e29b-41d4-a716-446655440000 --url https://new-url.com --tenant my-tenant

  # Update from JSON file (must contain complete webhook data)
  iz admin webhooks update my-webhook --data @webhook.json --tenant my-tenant`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		webhookIDOrName := args[0]

		if cfg.Tenant == "" {
			return fmt.Errorf(errors.MsgTenantRequired)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()

		// Fetch all webhooks to find the one to update (needed for both ID/name lookup and merge)
		webhooks, err := izanami.ListWebhooks(client, ctx, cfg.Tenant, izanami.ParseWebhooks)
		if err != nil {
			return fmt.Errorf("failed to fetch webhooks: %w", err)
		}

		// Find the webhook with matching ID or name
		var current *izanami.WebhookFull
		for i := range webhooks {
			if webhooks[i].ID == webhookIDOrName || webhooks[i].Name == webhookIDOrName {
				current = &webhooks[i]
				break
			}
		}

		if current == nil {
			return fmt.Errorf("webhook '%s' not found", webhookIDOrName)
		}

		// Use the actual ID for the API call
		webhookID := current.ID

		var updateData interface{}

		// If --data is provided, use it as the complete update payload
		if cmd.Flags().Changed("data") {
			if err := parseJSONData(webhookData, &updateData); err != nil {
				return fmt.Errorf("invalid JSON data: %w", err)
			}
		} else {

			// Build update data starting from current values
			data := map[string]interface{}{
				"name":    current.Name,
				"url":     current.URL,
				"enabled": current.Enabled,
				"global":  current.Global,
			}

			// Include optional fields if they have values
			if current.Description != "" {
				data["description"] = current.Description
			}
			if len(current.Features) > 0 {
				// Extract feature IDs from the feature refs
				featureIDs := make([]string, len(current.Features))
				for i, f := range current.Features {
					featureIDs[i] = f.ID
				}
				data["features"] = featureIDs
			}
			if len(current.Projects) > 0 {
				// Extract project IDs from the project refs
				projectIDs := make([]string, len(current.Projects))
				for i, p := range current.Projects {
					projectIDs[i] = p.ID
				}
				data["projects"] = projectIDs
			}
			if current.Context != "" {
				data["context"] = current.Context
			}
			if current.User != "" {
				data["user"] = current.User
			}
			if current.BodyTemplate != "" {
				data["bodyTemplate"] = current.BodyTemplate
			}
			if len(current.Headers) > 0 {
				data["headers"] = current.Headers
			}

			// Override with any changed flags
			if cmd.Flags().Changed("url") {
				data["url"] = webhookURL
			}
			if cmd.Flags().Changed("description") {
				data["description"] = webhookDescription
			}
			if cmd.Flags().Changed("features") {
				data["features"] = webhookFeatures
			}
			if cmd.Flags().Changed("projects") {
				data["projects"] = webhookProjects
			}
			if cmd.Flags().Changed("context") {
				data["context"] = webhookContext
			}
			if cmd.Flags().Changed("user") {
				data["user"] = webhookUser
			}
			if cmd.Flags().Changed("enabled") {
				data["enabled"] = webhookEnabled
			}
			if cmd.Flags().Changed("global") {
				data["global"] = webhookGlobal
			}
			if cmd.Flags().Changed("body-template") {
				data["bodyTemplate"] = webhookBodyTemplate
			}
			if cmd.Flags().Changed("headers") {
				var headers map[string]string
				if err := json.Unmarshal([]byte(webhookHeaders), &headers); err != nil {
					return fmt.Errorf("invalid headers JSON: %w", err)
				}
				data["headers"] = headers
			}

			updateData = data
		}

		if err := client.UpdateWebhook(ctx, cfg.Tenant, webhookID, updateData); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "✅ Webhook updated successfully\n")
		return nil
	},
}

// webhooksDeleteCmd deletes a webhook
var webhooksDeleteCmd = &cobra.Command{
	Use:   "delete <webhook-id-or-name>",
	Short: "Delete a webhook by ID or name",
	Long: `Delete a webhook by ID or name.

The webhook can be identified by its UUID or by name.

Examples:
  # Delete by name
  iz admin webhooks delete my-webhook --tenant my-tenant --force

  # Delete by ID
  iz admin webhooks delete 550e8400-e29b-41d4-a716-446655440000 --tenant my-tenant --force`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		webhookIDOrName := args[0]

		if cfg.Tenant == "" {
			return fmt.Errorf(errors.MsgTenantRequired)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()

		// Resolve webhook ID from name if needed
		webhookID := webhookIDOrName
		webhooks, err := izanami.ListWebhooks(client, ctx, cfg.Tenant, izanami.ParseWebhooks)
		if err != nil {
			return fmt.Errorf("failed to fetch webhooks: %w", err)
		}

		var found *izanami.WebhookFull
		for i := range webhooks {
			if webhooks[i].ID == webhookIDOrName || webhooks[i].Name == webhookIDOrName {
				found = &webhooks[i]
				break
			}
		}

		if found == nil {
			return fmt.Errorf("webhook '%s' not found", webhookIDOrName)
		}
		webhookID = found.ID

		// Confirm deletion unless --force is used
		if !webhooksDeleteForce {
			if !confirmDeletion(cmd, "webhook", found.Name) {
				return nil
			}
		}

		if err := client.DeleteWebhook(ctx, cfg.Tenant, webhookID); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "✅ Webhook deleted successfully\n")
		return nil
	},
}

// webhooksUsersCmd lists users with rights on a webhook
var webhooksUsersCmd = &cobra.Command{
	Use:   "users <webhook-id-or-name>",
	Short: "List users with rights on a webhook by ID or name",
	Long: `List all users who have been granted rights to access a specific webhook.

The webhook can be identified by its UUID or by name.
Shows each user's right level (Read, Write, Admin) and whether they are a tenant admin.

Examples:
  # List users for a webhook by name
  iz admin webhooks users my-webhook --tenant my-tenant

  # List users for a webhook by ID
  iz admin webhooks users 550e8400-e29b-41d4-a716-446655440000 --tenant my-tenant

  # Output as JSON
  iz admin webhooks users my-webhook --tenant my-tenant -o json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		webhookIDOrName := args[0]

		if cfg.Tenant == "" {
			return fmt.Errorf(errors.MsgTenantRequired)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()

		// Resolve webhook ID from name if needed
		webhooks, err := izanami.ListWebhooks(client, ctx, cfg.Tenant, izanami.ParseWebhooks)
		if err != nil {
			return fmt.Errorf("failed to fetch webhooks: %w", err)
		}

		var found *izanami.WebhookFull
		for i := range webhooks {
			if webhooks[i].ID == webhookIDOrName || webhooks[i].Name == webhookIDOrName {
				found = &webhooks[i]
				break
			}
		}

		if found == nil {
			return fmt.Errorf("webhook '%s' not found", webhookIDOrName)
		}
		webhookID := found.ID

		// For JSON output, use Identity mapper for raw JSON passthrough
		if outputFormat == "json" {
			raw, err := izanami.ListWebhookUsers(client, ctx, cfg.Tenant, webhookID, izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseWebhookUsers mapper
		users, err := izanami.ListWebhookUsers(client, ctx, cfg.Tenant, webhookID, izanami.ParseWebhookUsers)
		if err != nil {
			return err
		}

		if len(users) == 0 {
			fmt.Fprintln(cmd.OutOrStderr(), "No users found for this webhook")
			return nil
		}

		return output.PrintTo(cmd.OutOrStdout(), users, output.Format(outputFormat))
	},
}

func init() {
	adminCmd.AddCommand(webhooksCmd)

	webhooksCmd.AddCommand(webhooksListCmd)
	webhooksCmd.AddCommand(webhooksGetCmd)
	webhooksCmd.AddCommand(webhooksCreateCmd)
	webhooksCmd.AddCommand(webhooksUpdateCmd)
	webhooksCmd.AddCommand(webhooksDeleteCmd)
	webhooksCmd.AddCommand(webhooksUsersCmd)

	// Create flags
	webhooksCreateCmd.Flags().StringVar(&webhookURL, "url", "", "Webhook URL (required)")
	webhooksCreateCmd.Flags().StringVar(&webhookDescription, "description", "", "Description of the webhook")
	webhooksCreateCmd.Flags().StringSliceVar(&webhookFeatures, "features", []string{}, "Feature IDs to trigger webhook")
	webhooksCreateCmd.Flags().StringSliceVar(&webhookProjects, "projects", []string{}, "Project IDs to trigger webhook")
	webhooksCreateCmd.Flags().StringVar(&webhookContext, "context", "", "Context path filter")
	webhooksCreateCmd.Flags().StringVar(&webhookUser, "user", "", "User filter")
	webhooksCreateCmd.Flags().BoolVar(&webhookEnabled, "enabled", false, "Whether the webhook is enabled")
	webhooksCreateCmd.Flags().BoolVar(&webhookGlobal, "global", false, "Whether this is a global webhook")
	webhooksCreateCmd.Flags().StringVar(&webhookHeaders, "headers", "", "Custom headers as JSON object")
	webhooksCreateCmd.Flags().StringVar(&webhookBodyTemplate, "body-template", "", "Custom body template")
	webhooksCreateCmd.Flags().StringVar(&webhookData, "data", "", "JSON data (inline, @file, or - for stdin)")

	// Update flags
	webhooksUpdateCmd.Flags().StringVar(&webhookURL, "url", "", "New webhook URL")
	webhooksUpdateCmd.Flags().StringVar(&webhookDescription, "description", "", "New description")
	webhooksUpdateCmd.Flags().StringSliceVar(&webhookFeatures, "features", []string{}, "New feature IDs")
	webhooksUpdateCmd.Flags().StringSliceVar(&webhookProjects, "projects", []string{}, "New project IDs")
	webhooksUpdateCmd.Flags().StringVar(&webhookContext, "context", "", "New context path filter")
	webhooksUpdateCmd.Flags().StringVar(&webhookUser, "user", "", "New user filter")
	webhooksUpdateCmd.Flags().BoolVar(&webhookEnabled, "enabled", false, "Whether the webhook is enabled")
	webhooksUpdateCmd.Flags().BoolVar(&webhookGlobal, "global", false, "Whether this is a global webhook")
	webhooksUpdateCmd.Flags().StringVar(&webhookHeaders, "headers", "", "New custom headers as JSON object")
	webhooksUpdateCmd.Flags().StringVar(&webhookBodyTemplate, "body-template", "", "New custom body template")
	webhooksUpdateCmd.Flags().StringVar(&webhookData, "data", "", "JSON data (inline, @file, or - for stdin)")

	// Delete flags
	webhooksDeleteCmd.Flags().BoolVarP(&webhooksDeleteForce, "force", "f", false, "Skip confirmation prompt")
}
