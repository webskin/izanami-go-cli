package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"github.com/webskin/izanami-go-cli/internal/output"
)

const (
	errMsgTenantRequired = "tenant is required (use --tenant flag)"
)

var (
	keyName        string
	keyDescription string
	keyProjects    []string
	keyEnabled     bool
	keyAdmin       bool
)

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
	Use:   "list",
	Short: "List all API keys in a tenant",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.Tenant == "" {
			return fmt.Errorf(errMsgTenantRequired)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		keys, err := client.ListAPIKeys(ctx, cfg.Tenant)
		if err != nil {
			return err
		}

		if len(keys) == 0 {
			fmt.Fprintln(os.Stderr, "No API keys found")
			return nil
		}

		return output.Print(keys, output.Format(outputFormat))
	},
}

// keysGetCmd gets a specific API key
var keysGetCmd = &cobra.Command{
	Use:   "get <client-id>",
	Short: "Get details of a specific API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		clientID := args[0]

		if cfg.Tenant == "" {
			return fmt.Errorf(errMsgTenantRequired)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		key, err := client.GetAPIKey(ctx, cfg.Tenant, clientID)
		if err != nil {
			return err
		}

		return output.Print(key, output.Format(outputFormat))
	},
}

// keysCreateCmd creates a new API key
var keysCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if cfg.Tenant == "" {
			return fmt.Errorf(errMsgTenantRequired)
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
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(result)
		}

		// For table output, show important info
		fmt.Fprintf(os.Stderr, "✅ API key created successfully\n\n")
		fmt.Fprintf(os.Stderr, "Client ID:     %s\n", result.ClientID)
		fmt.Fprintf(os.Stderr, "Client Secret: %s\n", result.ClientSecret)
		fmt.Fprintf(os.Stderr, "Name:          %s\n", result.Name)
		fmt.Fprintf(os.Stderr, "Enabled:       %t\n", result.Enabled)
		fmt.Fprintf(os.Stderr, "Admin:         %t\n", result.Admin)
		if len(result.Projects) > 0 {
			fmt.Fprintf(os.Stderr, "Projects:      %v\n", result.Projects)
		}
		fmt.Fprintf(os.Stderr, "\n⚠️  IMPORTANT: Save the Client Secret - it won't be shown again!\n")

		return nil
	},
}

// keysUpdateCmd updates an API key
var keysUpdateCmd = &cobra.Command{
	Use:   "update <client-id>",
	Short: "Update an existing API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		clientID := args[0]

		if cfg.Tenant == "" {
			return fmt.Errorf(errMsgTenantRequired)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		// Build update data from changed flags
		keyData := make(map[string]interface{})

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

		if len(keyData) == 0 {
			return fmt.Errorf("no fields to update (use --name, --description, --projects, --enabled, or --admin)")
		}

		ctx := context.Background()
		if err := client.UpdateAPIKey(ctx, cfg.Tenant, clientID, keyData); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "✅ API key updated successfully\n")
		return nil
	},
}

// keysDeleteCmd deletes an API key
var keysDeleteCmd = &cobra.Command{
	Use:   "delete <client-id>",
	Short: "Delete an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		clientID := args[0]

		if cfg.Tenant == "" {
			return fmt.Errorf(errMsgTenantRequired)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.DeleteAPIKey(ctx, cfg.Tenant, clientID); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "✅ API key deleted successfully\n")
		return nil
	},
}

func init() {
	adminCmd.AddCommand(keysCmd)

	keysCmd.AddCommand(keysListCmd)
	keysCmd.AddCommand(keysGetCmd)
	keysCmd.AddCommand(keysCreateCmd)
	keysCmd.AddCommand(keysUpdateCmd)
	keysCmd.AddCommand(keysDeleteCmd)

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
}
