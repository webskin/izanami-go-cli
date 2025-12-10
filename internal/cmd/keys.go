package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/errors"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"github.com/webskin/izanami-go-cli/internal/output"
)

// redactAPIKeySecrets redacts the clientSecret field in API key JSON data (array)
func redactAPIKeySecrets(data []byte) ([]byte, error) {
	var keys []map[string]interface{}
	if err := json.Unmarshal(data, &keys); err != nil {
		return nil, err
	}
	for i := range keys {
		if _, ok := keys[i]["clientSecret"]; ok {
			keys[i]["clientSecret"] = redactedSecret
		}
	}
	// Use encoder with HTML escaping disabled to avoid \u003c for < characters
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(keys); err != nil {
		return nil, err
	}
	// Remove trailing newline added by Encode
	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}
	return result, nil
}

// redactAPIKeySecret redacts the clientSecret field in a single API key
func redactAPIKeySecret(key *izanami.APIKey) {
	if key.ClientSecret != "" {
		key.ClientSecret = redactedSecret
	}
}

// redactAPIKeySecretsSlice redacts the clientSecret field in a slice of API keys
func redactAPIKeySecretsSlice(keys []izanami.APIKey) {
	for i := range keys {
		if keys[i].ClientSecret != "" {
			keys[i].ClientSecret = redactedSecret
		}
	}
}

var (
	keyName         string
	keyDescription  string
	keyProjects     []string
	keyEnabled      bool
	keyAdmin        bool
	keysDeleteForce bool
	keysShowSecrets bool
)

const redactedSecret = "<redacted>"

// keysCmd represents the keys command
var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage API keys",
	Long: `Manage API keys for client authentication.

API keys are used by applications to access the Izanami Client API (/api/v2/*).
Each key can be associated with specific projects and can have admin privileges.

Examples:
  # List all API keys
  iz admin keys list --tenant my-tenant

  # Create a new API key
  iz admin keys create my-app-key --tenant my-tenant --projects proj1,proj2

  # Delete an API key
  iz admin keys delete <client-id> --tenant my-tenant`,
}

// keysListCmd lists API keys
var keysListCmd = &cobra.Command{
	Use:         "list",
	Short:       "List all API keys in a tenant",
	Annotations: map[string]string{"route": "GET /api/admin/tenants/:tenant/keys"},
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
			raw, err := izanami.ListAPIKeys(client, ctx, cfg.Tenant, izanami.Identity)
			if err != nil {
				return err
			}
			// Redact secrets unless --show-secrets is set
			if !keysShowSecrets {
				raw, err = redactAPIKeySecrets(raw)
				if err != nil {
					return err
				}
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseAPIKeys mapper
		keys, err := izanami.ListAPIKeys(client, ctx, cfg.Tenant, izanami.ParseAPIKeys)
		if err != nil {
			return err
		}

		// Redact secrets unless --show-secrets is set
		if !keysShowSecrets {
			redactAPIKeySecretsSlice(keys)
		}

		if len(keys) == 0 {
			fmt.Fprintln(cmd.OutOrStderr(), "No API keys found")
			return nil
		}

		return output.PrintTo(cmd.OutOrStdout(), keys, output.Format(outputFormat))
	},
}

// keysGetCmd gets a specific API key
var keysGetCmd = &cobra.Command{
	Use:         "get <name>",
	Short:       "Get details of a specific API key by name",
	Annotations: map[string]string{"route": "GET /api/admin/tenants/:tenant/keys"},
	Args:        cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if cfg.Tenant == "" {
			return fmt.Errorf(errors.MsgTenantRequired)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		// Note: GetAPIKeyByName doesn't have a dedicated endpoint, it filters from list
		// So raw JSON output isn't available for a single key
		key, err := client.GetAPIKeyByName(ctx, cfg.Tenant, name)
		if err != nil {
			return err
		}

		// Redact secret unless --show-secrets is set
		if !keysShowSecrets {
			redactAPIKeySecret(key)
		}

		return output.PrintTo(cmd.OutOrStdout(), key, output.Format(outputFormat))
	},
}

// keysCreateCmd creates a new API key
var keysCreateCmd = &cobra.Command{
	Use:         "create <name>",
	Short:       "Create a new API key",
	Annotations: map[string]string{"route": "POST /api/admin/tenants/:tenant/keys"},
	Args:        cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if cfg.Tenant == "" {
			return fmt.Errorf(errors.MsgTenantRequired)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		keyData := map[string]interface{}{
			"name":        name,
			"description": keyDescription,
			"enabled":     keyEnabled,
			"admin":       keyAdmin,
		}

		if len(keyProjects) > 0 {
			keyData["projects"] = keyProjects
		}

		ctx := context.Background()
		result, err := client.CreateAPIKey(ctx, cfg.Tenant, keyData)
		if err != nil {
			return err
		}

		// Print the result with the secret
		if output.Format(outputFormat) == output.JSON {
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(result)
		}

		// For table output, show important info
		fmt.Fprintf(cmd.OutOrStderr(), "✅ API key created successfully\n\n")
		fmt.Fprintf(cmd.OutOrStderr(), "Client ID:     %s\n", result.ClientID)
		fmt.Fprintf(cmd.OutOrStderr(), "Client Secret: %s\n", result.ClientSecret)
		fmt.Fprintf(cmd.OutOrStderr(), "Name:          %s\n", result.Name)
		fmt.Fprintf(cmd.OutOrStderr(), "Enabled:       %t\n", result.Enabled)
		fmt.Fprintf(cmd.OutOrStderr(), "Admin:         %t\n", result.Admin)
		if len(result.Projects) > 0 {
			fmt.Fprintf(cmd.OutOrStderr(), "Projects:      %v\n", result.Projects)
		}
		fmt.Fprintf(cmd.OutOrStderr(), "\n⚠️  IMPORTANT: Save the Client Secret - it won't be shown again!\n")

		return nil
	},
}

// keysUpdateCmd updates an API key
var keysUpdateCmd = &cobra.Command{
	Use:         "update <name>",
	Short:       "Update an existing API key by name",
	Annotations: map[string]string{"route": "PUT /api/admin/tenants/:tenant/keys/:name"},
	Args:        cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if cfg.Tenant == "" {
			return fmt.Errorf(errors.MsgTenantRequired)
		}

		// Check if any update flags were provided
		hasUpdates := cmd.Flags().Changed("name") ||
			cmd.Flags().Changed("description") ||
			cmd.Flags().Changed("projects") ||
			cmd.Flags().Changed("enabled") ||
			cmd.Flags().Changed("admin")

		if !hasUpdates {
			return fmt.Errorf("no fields to update (use --name, --description, --projects, --enabled, or --admin)")
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()

		// Fetch current key by name to get clientID and existing values (API requires full object)
		currentKey, err := client.GetAPIKeyByName(ctx, cfg.Tenant, name)
		if err != nil {
			return fmt.Errorf("failed to get current key: %w", err)
		}

		// Start with current values
		keyData := map[string]interface{}{
			"name":        currentKey.Name,
			"description": currentKey.Description,
			"projects":    currentKey.Projects,
			"enabled":     currentKey.Enabled,
			"admin":       currentKey.Admin,
		}

		// Override with user-provided values
		if cmd.Flags().Changed("name") {
			keyData["name"] = keyName
		}
		if cmd.Flags().Changed("description") {
			keyData["description"] = keyDescription
		}
		if cmd.Flags().Changed("projects") {
			keyData["projects"] = keyProjects
		}
		if cmd.Flags().Changed("enabled") {
			keyData["enabled"] = keyEnabled
		}
		if cmd.Flags().Changed("admin") {
			keyData["admin"] = keyAdmin
		}

		if err := client.UpdateAPIKey(ctx, cfg.Tenant, name, keyData); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "✅ API key updated successfully\n")
		return nil
	},
}

// keysDeleteCmd deletes an API key
var keysDeleteCmd = &cobra.Command{
	Use:         "delete <name>",
	Short:       "Delete an API key by name",
	Annotations: map[string]string{"route": "DELETE /api/admin/tenants/:tenant/keys/:name"},
	Args:        cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if cfg.Tenant == "" {
			return fmt.Errorf(errors.MsgTenantRequired)
		}

		// Confirm deletion unless --force is used
		if !keysDeleteForce {
			if !confirmDeletion(cmd, "API key", name) {
				return nil
			}
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.DeleteAPIKey(ctx, cfg.Tenant, name); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "✅ API key deleted successfully\n")
		return nil
	},
}

// keysUsersCmd lists users with rights on an API key
var keysUsersCmd = &cobra.Command{
	Use:         "users <client-id>",
	Short:       "List users with rights on an API key",
	Annotations: map[string]string{"route": "GET /api/admin/tenants/:tenant/keys/:name/users"},
	Long: `List all users who have been granted rights to access a specific API key.

Shows each user's right level (Read, Write, Admin) and whether they are a tenant admin.

Examples:
  # List users for an API key
  iz admin keys users my-client-id --tenant my-tenant

  # Output as JSON
  iz admin keys users my-client-id --tenant my-tenant -o json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		clientID := args[0]

		if cfg.Tenant == "" {
			return fmt.Errorf(errors.MsgTenantRequired)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()

		// For JSON output, use Identity mapper for raw JSON passthrough
		if outputFormat == "json" {
			raw, err := izanami.ListAPIKeyUsers(client, ctx, cfg.Tenant, clientID, izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseKeyScopedUsers mapper
		users, err := izanami.ListAPIKeyUsers(client, ctx, cfg.Tenant, clientID, izanami.ParseKeyScopedUsers)
		if err != nil {
			return err
		}

		if len(users) == 0 {
			fmt.Fprintln(cmd.OutOrStderr(), "No users found for this API key")
			return nil
		}

		return output.PrintTo(cmd.OutOrStdout(), users, output.Format(outputFormat))
	},
}

func init() {
	adminCmd.AddCommand(keysCmd)

	keysCmd.AddCommand(keysListCmd)
	keysCmd.AddCommand(keysGetCmd)
	keysCmd.AddCommand(keysCreateCmd)
	keysCmd.AddCommand(keysUpdateCmd)
	keysCmd.AddCommand(keysDeleteCmd)
	keysCmd.AddCommand(keysUsersCmd)

	// Create flags
	keysCreateCmd.Flags().StringVar(&keyDescription, "description", "", "Description of the API key")
	keysCreateCmd.Flags().StringSliceVar(&keyProjects, "projects", []string{}, "Projects this key can access")
	keysCreateCmd.Flags().BoolVar(&keyEnabled, "enabled", true, "Whether the key is enabled")
	keysCreateCmd.Flags().BoolVar(&keyAdmin, "admin", false, "Whether this key has admin privileges")

	// Update flags
	keysUpdateCmd.Flags().StringVar(&keyName, "name", "", "New name for the key")
	keysUpdateCmd.Flags().StringVar(&keyDescription, "description", "", "New description")
	keysUpdateCmd.Flags().StringSliceVar(&keyProjects, "projects", []string{}, "New project list")
	keysUpdateCmd.Flags().BoolVar(&keyEnabled, "enabled", true, "Whether the key is enabled")
	keysUpdateCmd.Flags().BoolVar(&keyAdmin, "admin", false, "Whether this key has admin privileges")

	// Delete flags
	keysDeleteCmd.Flags().BoolVarP(&keysDeleteForce, "force", "f", false, "Skip confirmation prompt")

	// Show secrets flags
	keysListCmd.Flags().BoolVar(&keysShowSecrets, "show-secrets", false, "Show client secrets (hidden by default)")
	keysGetCmd.Flags().BoolVar(&keysShowSecrets, "show-secrets", false, "Show client secret (hidden by default)")
}
