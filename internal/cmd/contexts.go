package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"github.com/webskin/izanami-go-cli/internal/output"
)

var (
	contextProject        string
	contextAll            bool
	contextParent         string
	contextProtected      bool
	contextGlobal         bool
	contextData           string
	contextsDeleteForce   bool
	contextUpdateProtected string
)

// contextsCmd represents the admin contexts command
var contextsCmd = &cobra.Command{
	Use:   "contexts",
	Short: "Manage feature contexts (admin)",
	Long: `Administrative operations for feature contexts (environments/overrides) in Izanami.

These are admin operations that use the /api/admin/... endpoints and require
admin authentication (username and personal access token).

Contexts allow you to define different behavior for features in different
environments or scenarios. For example:
  - prod vs dev vs staging
  - Different regions: prod/eu vs prod/us
  - Different customers or tenants

Contexts can be:
  - Global: Available across all projects in a tenant
  - Local: Specific to a single project
  - Hierarchical: Nested contexts like prod/eu/france

Features can have context-specific overrides, allowing you to enable/disable
or change behavior based on the context.`,
}

// contextsListCmd lists contexts
var contextsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all contexts",
	Long: `List all contexts for a tenant or project.

By default, only shows root-level contexts. Use --all to show all nested contexts.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.Validate(); err != nil {
			return err
		}
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()

		// For JSON output, use Identity mapper to get raw JSON
		if outputFormat == "json" {
			raw, err := izanami.ListContexts(client, ctx, cfg.Tenant, contextProject, contextAll, izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseContexts mapper
		contexts, err := izanami.ListContexts(client, ctx, cfg.Tenant, contextProject, contextAll, izanami.ParseContexts)
		if err != nil {
			return err
		}

		// For table output, convert to table view (PATH first, no CHILDREN)
		// Hide overload column when listing tenant-level contexts (no --project flag)
		// Show overload column only when --project flag is provided
		if contextProject != "" {
			tableView := izanami.FlattenContextsForTable(contexts)
			return output.PrintTo(cmd.OutOrStdout(), tableView, output.Format(outputFormat))
		} else {
			tableView := izanami.FlattenContextsForTableSimple(contexts)
			return output.PrintTo(cmd.OutOrStdout(), tableView, output.Format(outputFormat))
		}
	},
}

// contextsGetCmd gets a specific context
var contextsGetCmd = &cobra.Command{
	Use:   "get <context-path>",
	Short: "Get a specific context",
	Long: `Get detailed information about a specific context.

The context path should be the full hierarchical path, e.g.:
  - prod
  - prod/eu
  - prod/eu/france`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.Validate(); err != nil {
			return err
		}
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		contextName := args[0]

		// Determine project
		proj := contextProject
		if proj == "" {
			proj = cfg.Project
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		// List all contexts (without 'all' to get root + immediate children)
		ctx := context.Background()
		contexts, err := izanami.ListContexts(client, ctx, cfg.Tenant, proj, false, izanami.ParseContexts)
		if err != nil {
			return err
		}

		// Search for the context recursively
		found := findContextByName(contexts, contextName)
		if found == nil {
			return fmt.Errorf("context not found: %s", contextName)
		}

		return output.PrintTo(cmd.OutOrStdout(), found, output.Format(outputFormat))
	},
}

// contextsCreateCmd creates a new context
var contextsCreateCmd = &cobra.Command{
	Use:   "create <context-name>",
	Short: "Create a new context",
	Long: `Create a new feature context.

Contexts can be created at the root level or as children of existing contexts.

Examples:
  # Create a root-level global context
  iz contexts create prod --global

  # Create a root-level project context
  iz contexts create prod --project my-project

  # Create a nested context
  iz contexts create france --parent prod/eu --project my-project

  # Create with custom data
  iz contexts create prod --data '{"name":"prod","protected":true}'`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.Validate(); err != nil {
			return err
		}
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		contextName := args[0]

		// Determine if this is a project or global context
		proj := contextProject
		if proj == "" && !contextGlobal {
			proj = cfg.Project
		}
		if proj == "" && !contextGlobal {
			return fmt.Errorf("either --project or --global is required")
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		// Build context data
		var data interface{}
		if cmd.Flags().Changed("data") {
			if err := parseJSONData(contextData, &data); err != nil {
				return err
			}
		} else {
			data = map[string]interface{}{
				"name":      contextName,
				"protected": contextProtected,
			}
		}

		ctx := context.Background()
		if err := client.CreateContext(ctx, cfg.Tenant, proj, contextName, contextParent, data); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Context created successfully: %s\n", contextName)
		return nil
	},
}

// contextsUpdateCmd updates a global context
var contextsUpdateCmd = &cobra.Command{
	Use:   "update <context-path>",
	Short: "Update a global context",
	Long: `Update a global feature context.

NOTE: Only global contexts can be updated. Project-specific contexts do not
support the update operation.

The only property that can be updated is the protected status.

Examples:
  # Set context as protected
  iz admin contexts update prod --tenant my-tenant --protected=true

  # Remove protected status
  iz admin contexts update prod --tenant my-tenant --protected=false`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.Validate(); err != nil {
			return err
		}
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		contextPath := args[0]

		// Validate protected flag value
		var protected bool
		switch contextUpdateProtected {
		case "true":
			protected = true
		case "false":
			protected = false
		default:
			return fmt.Errorf("--protected must be 'true' or 'false', got '%s'", contextUpdateProtected)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		// Build context data - only protected is supported
		data := map[string]interface{}{
			"protected": protected,
		}

		ctx := context.Background()
		if err := client.UpdateContext(ctx, cfg.Tenant, contextPath, data); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Context updated successfully: %s\n", contextPath)
		return nil
	},
}

// contextsDeleteCmd deletes a context
var contextsDeleteCmd = &cobra.Command{
	Use:   "delete <context-path>",
	Short: "Delete a context",
	Long: `Delete a feature context.

WARNING: This will also delete all child contexts and context-specific
feature overrides. This operation cannot be undone.

The context path should be the full hierarchical path.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.Validate(); err != nil {
			return err
		}
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		contextPath := args[0]

		// Confirm deletion unless --force is used
		if !contextsDeleteForce {
			if !confirmDeletion(cmd, "context", contextPath) {
				return nil
			}
		}

		// Determine project
		proj := contextProject
		if proj == "" {
			proj = cfg.Project
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.DeleteContext(ctx, cfg.Tenant, proj, contextPath); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Context deleted successfully: %s\n", contextPath)
		return nil
	},
}

// findContextByName recursively searches for a context by name
func findContextByName(contexts []izanami.Context, name string) *izanami.Context {
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
			if found := findContextByName(childContexts, name); found != nil {
				return found
			}
		}
	}
	return nil
}

func init() {
	// Contexts commands are registered under admin in admin.go

	// Add subcommands
	contextsCmd.AddCommand(contextsListCmd)
	contextsCmd.AddCommand(contextsGetCmd)
	contextsCmd.AddCommand(contextsCreateCmd)
	contextsCmd.AddCommand(contextsUpdateCmd)
	contextsCmd.AddCommand(contextsDeleteCmd)

	// List flags
	contextsListCmd.Flags().StringVar(&contextProject, "project", "", "List project-specific contexts")
	contextsListCmd.Flags().BoolVar(&contextAll, "all", false, "Show all nested contexts")

	// Get flags
	contextsGetCmd.Flags().StringVar(&contextProject, "project", "", "Project for the context")

	// Create flags
	contextsCreateCmd.Flags().StringVar(&contextProject, "project", "", "Create context in this project")
	contextsCreateCmd.Flags().BoolVar(&contextGlobal, "global", false, "Create global context")
	contextsCreateCmd.Flags().StringVar(&contextParent, "parent", "", "Parent context path")
	contextsCreateCmd.Flags().BoolVar(&contextProtected, "protected", false, "Mark context as protected")
	contextsCreateCmd.Flags().StringVar(&contextData, "data", "", "JSON context data")

	// Update flags
	contextsUpdateCmd.Flags().StringVar(&contextUpdateProtected, "protected", "", "Set protected status (true/false)")
	_ = contextsUpdateCmd.MarkFlagRequired("protected")

	// Delete flags
	contextsDeleteCmd.Flags().StringVar(&contextProject, "project", "", "Project for the context")
	contextsDeleteCmd.Flags().BoolVarP(&contextsDeleteForce, "force", "f", false, "Skip confirmation prompt")
}
