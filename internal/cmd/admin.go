package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"github.com/webskin/izanami-go-cli/internal/output"
)

// adminCmd represents the admin command
var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Administrative operations",
	Long: `Perform administrative operations in Izanami.

Admin operations require elevated privileges and are typically used for:
  - Managing tenants and projects
  - Managing users and API keys
  - Configuring webhooks
  - Importing/exporting data
  - Global search

These operations require username and personal access token authentication.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Load config first (parent's PersistentPreRunE)
		if rootCmd.PersistentPreRunE != nil {
			if err := rootCmd.PersistentPreRunE(cmd, args); err != nil {
				return err
			}
		}

		// Validate admin authentication
		if err := cfg.ValidateAdminAuth(); err != nil {
			return err
		}

		return nil
	},
}

// ============================================================================
// TENANTS
// ============================================================================

var adminTenantsCmd = &cobra.Command{
	Use:   "tenants",
	Short: "Manage tenants",
	Long:  `Manage Izanami tenants. Tenants are the top-level organizational unit.`,
}

var adminTenantsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tenants",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		tenants, err := client.ListTenants(ctx, nil)
		if err != nil {
			return err
		}

		return output.Print(tenants, output.Format(outputFormat))
	},
}

var adminTenantsGetCmd = &cobra.Command{
	Use:   "get <tenant-name>",
	Short: "Get a specific tenant",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		tenant, err := client.GetTenant(ctx, args[0])
		if err != nil {
			return err
		}

		return output.Print(tenant, output.Format(outputFormat))
	},
}

var (
	tenantDesc string
	tenantData string
)

var adminTenantsCreateCmd = &cobra.Command{
	Use:   "create <tenant-name>",
	Short: "Create a new tenant",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		tenantName := args[0]
		var data interface{}

		if cmd.Flags().Changed("data") {
			if err := parseJSONData(tenantData, &data); err != nil {
				return err
			}
		} else {
			data = map[string]interface{}{
				"name":        tenantName,
				"description": tenantDesc,
			}
		}

		ctx := context.Background()
		if err := client.CreateTenant(ctx, data); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Tenant created successfully: %s\n", tenantName)
		return nil
	},
}

var adminTenantsDeleteCmd = &cobra.Command{
	Use:   "delete <tenant-name>",
	Short: "Delete a tenant",
	Long:  `Delete a tenant. WARNING: This will delete all projects, features, and data in the tenant.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.DeleteTenant(ctx, args[0]); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Tenant deleted successfully: %s\n", args[0])
		return nil
	},
}

// ============================================================================
// PROJECTS
// ============================================================================

var adminProjectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "Manage projects",
	Long:  `Manage Izanami projects. Projects organize features within a tenant.`,
}

var adminProjectsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		projects, err := client.ListProjects(ctx, cfg.Tenant)
		if err != nil {
			return err
		}

		return output.Print(projects, output.Format(outputFormat))
	},
}

var adminProjectsGetCmd = &cobra.Command{
	Use:   "get <project-name>",
	Short: "Get a specific project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		project, err := client.GetProject(ctx, cfg.Tenant, args[0])
		if err != nil {
			return err
		}

		return output.Print(project, output.Format(outputFormat))
	},
}

var (
	projectDesc string
	projectData string
)

var adminProjectsCreateCmd = &cobra.Command{
	Use:   "create <project-name>",
	Short: "Create a new project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		projectName := args[0]
		var data interface{}

		if cmd.Flags().Changed("data") {
			if err := parseJSONData(projectData, &data); err != nil {
				return err
			}
		} else {
			data = map[string]interface{}{
				"name":        projectName,
				"description": projectDesc,
			}
		}

		ctx := context.Background()
		if err := client.CreateProject(ctx, cfg.Tenant, data); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Project created successfully: %s\n", projectName)
		return nil
	},
}

var adminProjectsDeleteCmd = &cobra.Command{
	Use:   "delete <project-name>",
	Short: "Delete a project",
	Long:  `Delete a project. WARNING: This will delete all features in the project.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.DeleteProject(ctx, cfg.Tenant, args[0]); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Project deleted successfully: %s\n", args[0])
		return nil
	},
}

// ============================================================================
// TAGS
// ============================================================================

var adminTagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "Manage tags",
	Long:  `Manage feature tags. Tags help organize and categorize features.`,
}

var adminTagsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tags",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		tags, err := client.ListTags(ctx, cfg.Tenant)
		if err != nil {
			return err
		}

		return output.Print(tags, output.Format(outputFormat))
	},
}

var (
	tagDesc string
	tagData string
)

var adminTagsCreateCmd = &cobra.Command{
	Use:   "create <tag-name>",
	Short: "Create a new tag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		tagName := args[0]
		var data interface{}

		if cmd.Flags().Changed("data") {
			if err := parseJSONData(tagData, &data); err != nil {
				return err
			}
		} else {
			data = map[string]interface{}{
				"name":        tagName,
				"description": tagDesc,
			}
		}

		ctx := context.Background()
		if err := client.CreateTag(ctx, cfg.Tenant, data); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Tag created successfully: %s\n", tagName)
		return nil
	},
}

var adminTagsDeleteCmd = &cobra.Command{
	Use:   "delete <tag-name>",
	Short: "Delete a tag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.DeleteTag(ctx, cfg.Tenant, args[0]); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Tag deleted successfully: %s\n", args[0])
		return nil
	},
}

// ============================================================================
// SEARCH
// ============================================================================

var (
	searchFilters []string
)

var adminSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Global search across resources",
	Long: `Search across all resources in Izanami (or within a specific tenant).

Available filters: PROJECT, FEATURE, KEY, TAG, SCRIPT, GLOBAL_CONTEXT, LOCAL_CONTEXT, WEBHOOK

Examples:
  # Search globally
  iz admin search "my-feature"

  # Search within a tenant
  iz admin search "auth" --tenant my-tenant

  # Search with filters
  iz admin search "user" --filter FEATURE,PROJECT`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		results, err := client.Search(ctx, cfg.Tenant, args[0], searchFilters)
		if err != nil {
			return err
		}

		return output.Print(results, output.Format(outputFormat))
	},
}

// ============================================================================
// IMPORT/EXPORT
// ============================================================================

var (
	exportOutput string
)

var adminExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export tenant data",
	Long: `Export all data from a tenant in newline-delimited JSON format.

The export includes:
  - Projects
  - Features
  - Contexts
  - Tags
  - API keys
  - Webhooks

Examples:
  # Export to file
  iz admin export --output export.ndjson

  # Export to stdout
  iz admin export`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		data, err := client.Export(ctx, cfg.Tenant)
		if err != nil {
			return err
		}

		if exportOutput != "" {
			if err := os.WriteFile(exportOutput, []byte(data), 0644); err != nil {
				return fmt.Errorf("failed to write export file: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Export written to: %s\n", exportOutput)
		} else {
			fmt.Print(data)
		}

		return nil
	},
}

var (
	importConflict string
	importTimezone string
)

var adminImportCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import tenant data",
	Long: `Import data into a tenant from a newline-delimited JSON file.

Conflict resolution strategies:
  - FAIL: Stop on first conflict (default)
  - SKIP: Skip conflicting items
  - OVERWRITE: Overwrite existing items

Examples:
  # Import with default settings (fail on conflict)
  iz admin import export.ndjson

  # Import and overwrite conflicts
  iz admin import export.ndjson --conflict OVERWRITE

  # Import with timezone
  iz admin import export.ndjson --timezone "Europe/Paris"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		req := izanami.ImportRequest{
			Conflict: importConflict,
			Timezone: importTimezone,
		}

		ctx := context.Background()
		status, err := client.Import(ctx, cfg.Tenant, args[0], req)
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Import started: %s\n", status.ID)
		fmt.Fprintf(os.Stderr, "Status: %s\n", status.Status)
		if status.Message != "" {
			fmt.Fprintf(os.Stderr, "Message: %s\n", status.Message)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(adminCmd)

	// Tenants
	adminCmd.AddCommand(adminTenantsCmd)
	adminTenantsCmd.AddCommand(adminTenantsListCmd)
	adminTenantsCmd.AddCommand(adminTenantsGetCmd)
	adminTenantsCmd.AddCommand(adminTenantsCreateCmd)
	adminTenantsCmd.AddCommand(adminTenantsDeleteCmd)

	adminTenantsCreateCmd.Flags().StringVar(&tenantDesc, "description", "", "Tenant description")
	adminTenantsCreateCmd.Flags().StringVar(&tenantData, "data", "", "JSON tenant data")

	// Projects
	adminCmd.AddCommand(adminProjectsCmd)
	adminProjectsCmd.AddCommand(adminProjectsListCmd)
	adminProjectsCmd.AddCommand(adminProjectsGetCmd)
	adminProjectsCmd.AddCommand(adminProjectsCreateCmd)
	adminProjectsCmd.AddCommand(adminProjectsDeleteCmd)

	adminProjectsCreateCmd.Flags().StringVar(&projectDesc, "description", "", "Project description")
	adminProjectsCreateCmd.Flags().StringVar(&projectData, "data", "", "JSON project data")

	// Tags
	adminCmd.AddCommand(adminTagsCmd)
	adminTagsCmd.AddCommand(adminTagsListCmd)
	adminTagsCmd.AddCommand(adminTagsCreateCmd)
	adminTagsCmd.AddCommand(adminTagsDeleteCmd)

	adminTagsCreateCmd.Flags().StringVar(&tagDesc, "description", "", "Tag description")
	adminTagsCreateCmd.Flags().StringVar(&tagData, "data", "", "JSON tag data")

	// Search
	adminCmd.AddCommand(adminSearchCmd)
	adminSearchCmd.Flags().StringSliceVar(&searchFilters, "filter", []string{}, "Filter by resource type")

	// Import/Export
	adminCmd.AddCommand(adminExportCmd)
	adminCmd.AddCommand(adminImportCmd)

	adminExportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file (default: stdout)")
	adminImportCmd.Flags().StringVar(&importConflict, "conflict", "FAIL", "Conflict resolution: FAIL, SKIP, OVERWRITE")
	adminImportCmd.Flags().StringVar(&importTimezone, "timezone", "", "Timezone for time-based features")
}
