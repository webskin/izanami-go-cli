package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"github.com/webskin/izanami-go-cli/internal/output"
)

var (
	overloadContext          string
	overloadEnabled          bool
	overloadData             string
	overloadPreserveProtect  bool
	overloadDeleteForce      bool
)

// overloadsCmd represents the admin overloads command
var overloadsCmd = &cobra.Command{
	Use:   "overloads",
	Short: "Manage feature overloads in contexts",
	Long: `Manage feature overloads (context-specific feature strategies) in Izanami.

Overloads allow you to define different feature behavior for specific contexts.
For example, you can:
  - Enable a feature only for certain users in production
  - Override a feature's conditions for a specific region
  - Disable a feature in a test environment

The --context flag specifies the context path (e.g., "PROD", "PROD/mobile", "PROD/mobile/EU").
The --project flag (global) specifies which project the feature belongs to.

Examples:
  # Set a simple overload (enable feature for all users in PROD)
  iz admin overloads set my-feature --context PROD --project my-project --enabled

  # Set an overload with user list conditions
  iz admin overloads set my-feature --context PROD --project my-project --data '{
    "enabled": true,
    "conditions": [{"rule": {"type": "UserList", "users": ["Bob"]}}]
  }'

  # View an existing overload
  iz admin overloads get my-feature --context PROD --project my-project

  # Delete an overload
  iz admin overloads delete my-feature --context PROD --project my-project`,
}

// overloadsSetCmd creates or updates a feature overload in a context
var overloadsSetCmd = &cobra.Command{
	Use:         "set <feature-name>",
	Short:       "Create or update a feature overload in a context",
	Annotations: map[string]string{"route": "PUT /api/admin/tenants/:tenant/projects/:project/contexts/:context/features/:name"},
	Long: `Create or update a feature overload (context-specific strategy) in Izanami.

You can provide the overload strategy via:
  - The --enabled flag for simple boolean overloads
  - The --data flag for complex strategies with conditions

Strategy format (JSON):
  {
    "enabled": true,
    "resultType": "boolean",
    "conditions": [
      {
        "rule": { "type": "UserList", "users": ["alice", "bob"] }
      }
    ]
  }

Rule types:
  - UserList: Activate for specific users
  - UserPercentage: Activate for a percentage of users
  - All: Activate for all users (when no rule is specified)

Examples:
  # Simple: Enable feature in PROD context
  iz admin overloads set my-feature --context PROD --project my-project --enabled

  # Enable only for specific users
  iz admin overloads set my-feature --context PROD --project my-project --data '{
    "enabled": true,
    "conditions": [{"rule": {"type": "UserList", "users": ["Bob"]}}]
  }'

  # Enable for 50% of users
  iz admin overloads set my-feature --context PROD --project my-project --data '{
    "enabled": true,
    "conditions": [{"rule": {"type": "UserPercentage", "percentage": 50}}]
  }'

  # From file
  iz admin overloads set my-feature --context PROD --project my-project --data @strategy.json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.Validate(); err != nil {
			return err
		}
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		// Project is required
		if cfg.Project == "" {
			return fmt.Errorf("project is required (use --project flag or IZ_PROJECT)")
		}

		// Context is required
		if overloadContext == "" {
			return fmt.Errorf("context is required (use --context flag)")
		}

		featureName := args[0]

		// Build strategy from flags
		var strategy interface{}
		if overloadData != "" {
			// Parse JSON data if provided
			if err := parseJSONData(overloadData, &strategy); err != nil {
				return fmt.Errorf("invalid JSON data: %w", err)
			}
		} else {
			// Simple boolean strategy - resultType is required by the API
			strategy = map[string]interface{}{
				"enabled":    overloadEnabled,
				"resultType": "boolean",
			}
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.SetOverload(ctx, cfg.Tenant, cfg.Project, overloadContext, featureName, strategy, overloadPreserveProtect); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Overload set successfully: %s in context %s\n", featureName, overloadContext)
		return nil
	},
}

// overloadsGetCmd retrieves a feature overload from a context
var overloadsGetCmd = &cobra.Command{
	Use:         "get <feature-name>",
	Short:       "Get a feature overload from a context",
	Annotations: map[string]string{"route": "GET /api/admin/tenants/:tenant/features/:id (extracts overload for context)"},
	Long: `Get a feature overload (context-specific strategy) from Izanami.

This command retrieves the feature and extracts the overload configuration
for the specified context path.

Examples:
  # Get overload in table format
  iz admin overloads get my-feature --context PROD --project my-project

  # Get overload in JSON format
  iz admin overloads get my-feature --context PROD --project my-project -o json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.Validate(); err != nil {
			return err
		}
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		// Project is required for fetching context tree
		if cfg.Project == "" {
			return fmt.Errorf("project is required (use --project flag or IZ_PROJECT)")
		}

		// Context is required
		if overloadContext == "" {
			return fmt.Errorf("context is required (use --context flag)")
		}

		featureName := args[0]

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()

		// For JSON output, use Identity mapper
		if outputFormat == "json" {
			raw, err := izanami.GetOverload(client, ctx, cfg.Tenant, cfg.Project, featureName, overloadContext, izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, parse into FeatureOverload and display
		overload, err := izanami.GetOverload(client, ctx, cfg.Tenant, cfg.Project, featureName, overloadContext, izanami.Unmarshal[izanami.FeatureOverload]())
		if err != nil {
			return err
		}

		return output.PrintTo(cmd.OutOrStdout(), overload, output.Table)
	},
}

// overloadsDeleteCmd removes a feature overload from a context
var overloadsDeleteCmd = &cobra.Command{
	Use:         "delete <feature-name>",
	Short:       "Delete a feature overload from a context",
	Annotations: map[string]string{"route": "DELETE /api/admin/tenants/:tenant/projects/:project/contexts/:context/features/:name"},
	Long: `Delete a feature overload (context-specific strategy) from Izanami.

This removes the context-specific configuration, causing the feature to fall back
to its base configuration or parent context configuration.

Examples:
  # Delete with confirmation prompt
  iz admin overloads delete my-feature --context PROD --project my-project

  # Delete without confirmation
  iz admin overloads delete my-feature --context PROD --project my-project --force`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.Validate(); err != nil {
			return err
		}
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		// Project is required
		if cfg.Project == "" {
			return fmt.Errorf("project is required (use --project flag or IZ_PROJECT)")
		}

		// Context is required
		if overloadContext == "" {
			return fmt.Errorf("context is required (use --context flag)")
		}

		featureName := args[0]

		// Confirm deletion unless --force is used
		if !overloadDeleteForce {
			if !confirmDeletion(cmd, "overload", fmt.Sprintf("%s in context %s", featureName, overloadContext)) {
				return nil
			}
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.DeleteOverload(ctx, cfg.Tenant, cfg.Project, overloadContext, featureName, overloadPreserveProtect); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Overload deleted successfully: %s from context %s\n", featureName, overloadContext)
		return nil
	},
}

func init() {
	// Overloads commands are registered under admin in admin.go

	// Add subcommands
	overloadsCmd.AddCommand(overloadsSetCmd)
	overloadsCmd.AddCommand(overloadsGetCmd)
	overloadsCmd.AddCommand(overloadsDeleteCmd)

	// Set flags
	overloadsSetCmd.Flags().StringVar(&overloadContext, "context", "", "Context path (e.g., PROD, PROD/mobile)")
	overloadsSetCmd.Flags().BoolVar(&overloadEnabled, "enabled", true, "Enable the feature in this context")
	overloadsSetCmd.Flags().StringVar(&overloadData, "data", "", "JSON strategy data (inline, @file, or - for stdin)")
	overloadsSetCmd.Flags().BoolVar(&overloadPreserveProtect, "preserve-protected", false, "Preserve protected contexts")
	_ = overloadsSetCmd.MarkFlagRequired("context")

	// Get flags
	overloadsGetCmd.Flags().StringVar(&overloadContext, "context", "", "Context path (e.g., PROD, PROD/mobile)")
	_ = overloadsGetCmd.MarkFlagRequired("context")

	// Delete flags
	overloadsDeleteCmd.Flags().StringVar(&overloadContext, "context", "", "Context path (e.g., PROD, PROD/mobile)")
	overloadsDeleteCmd.Flags().BoolVarP(&overloadDeleteForce, "force", "f", false, "Skip confirmation prompt")
	overloadsDeleteCmd.Flags().BoolVar(&overloadPreserveProtect, "preserve-protected", false, "Preserve protected contexts")
	_ = overloadsDeleteCmd.MarkFlagRequired("context")
}
