package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"github.com/webskin/izanami-go-cli/internal/output"
)

var (
	// Admin authentication flags
	adminPATUsername string
	adminJwtToken    string
	adminPatToken    string
	// Delete confirmation flags
	tenantsDeleteForce  bool
	projectsDeleteForce bool
	tagsDeleteForce     bool
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

These operations require authentication via JWT token (from 'iz login') or Personal Access Token.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Load config first (parent's PersistentPreRunE)
		if rootCmd.PersistentPreRunE != nil {
			if err := rootCmd.PersistentPreRunE(cmd, args); err != nil {
				return err
			}
		}

		// Apply admin-specific authentication flags
		if adminPATUsername != "" {
			cfg.Username = adminPATUsername
		}
		if adminJwtToken != "" {
			cfg.JwtToken = adminJwtToken
		}
		if adminPatToken != "" {
			cfg.PatToken = adminPatToken
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

		// Convert to summaries (list endpoint doesn't include projects/tags)
		summaries := make([]izanami.TenantSummary, len(tenants))
		for i, t := range tenants {
			summaries[i] = izanami.TenantSummary{
				Name:        t.Name,
				Description: t.Description,
			}
		}

		return output.Print(summaries, output.Format(outputFormat))
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

var adminTenantsUpdateCmd = &cobra.Command{
	Use:   "update <tenant-name>",
	Short: "Update a tenant",
	Long: `Update a tenant's properties.

You can provide the updated data via:
  - --description flag (merged with existing data)
  - --data flag with JSON data
  - Both flags (--description takes precedence)

Examples:
  # Update description only
  iz admin tenants update my-tenant --description "New description"

  # Update with JSON data
  iz admin tenants update my-tenant --data '{"name":"my-tenant","description":"Updated"}'

  # Update with both (description flag takes precedence)
  iz admin tenants update my-tenant --data @tenant.json --description "Override desc"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		tenantName := args[0]
		var data map[string]interface{}

		// Parse JSON data if provided
		if cmd.Flags().Changed("data") {
			var jsonData interface{}
			if err := parseJSONData(tenantData, &jsonData); err != nil {
				return err
			}
			// Convert to map
			if dataMap, ok := jsonData.(map[string]interface{}); ok {
				data = dataMap
			} else {
				return fmt.Errorf("invalid data format: expected JSON object")
			}
		} else {
			// Start with empty map if no data provided
			data = make(map[string]interface{})
		}

		// Always set the name field
		data["name"] = tenantName

		// Merge description flag if provided
		if cmd.Flags().Changed("description") {
			data["description"] = tenantDesc
		}

		// Validate that we have at least name and description
		if _, hasDesc := data["description"]; !hasDesc {
			return fmt.Errorf("description is required (use --description flag or --data)")
		}

		ctx := context.Background()
		if err := client.UpdateTenant(ctx, tenantName, data); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Tenant updated successfully: %s\n", tenantName)
		return nil
	},
}

var adminTenantsDeleteCmd = &cobra.Command{
	Use:   "delete <tenant-name>",
	Short: "Delete a tenant",
	Long:  `Delete a tenant. WARNING: This will delete all projects, features, and data in the tenant.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tenantName := args[0]

		// Confirm deletion unless --force is used
		if !tenantsDeleteForce {
			if !confirmDeletion(cmd, "tenant", tenantName) {
				return nil
			}
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.DeleteTenant(ctx, tenantName); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Tenant deleted successfully: %s\n", tenantName)
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

		projectName := args[0]

		// Confirm deletion unless --force is used
		if !projectsDeleteForce {
			if !confirmDeletion(cmd, "project", projectName) {
				return nil
			}
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.DeleteProject(ctx, cfg.Tenant, projectName); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Project deleted successfully: %s\n", projectName)
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

var adminTagsGetCmd = &cobra.Command{
	Use:   "get <tag-name>",
	Short: "Get a specific tag",
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
		tag, err := client.GetTag(ctx, cfg.Tenant, args[0])
		if err != nil {
			return err
		}

		return output.Print(tag, output.Format(outputFormat))
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

		tagName := args[0]

		// Confirm deletion unless --force is used
		if !tagsDeleteForce {
			if !confirmDeletion(cmd, "tag", tagName) {
				return nil
			}
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.DeleteTag(ctx, cfg.Tenant, tagName); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Tag deleted successfully: %s\n", tagName)
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

	// Admin authentication flags (persistent for all admin commands)
	adminCmd.PersistentFlags().StringVar(&adminPATUsername, "personal-access-token-username", "", "Username for PAT authentication (env: IZ_PERSONAL_ACCESS_TOKEN_USERNAME)")
	adminCmd.PersistentFlags().StringVar(&adminJwtToken, "jwt-token", "", "JWT token for admin authentication (env: IZ_JWT_TOKEN)")
	adminCmd.PersistentFlags().StringVar(&adminPatToken, "personal-access-token", "", "Personal access token for admin authentication (env: IZ_PERSONAL_ACCESS_TOKEN)")

	// Features (admin operations)
	adminCmd.AddCommand(featuresCmd)
	featuresCmd.AddCommand(featuresListCmd)
	featuresCmd.AddCommand(featuresGetCmd)
	featuresCmd.AddCommand(featuresCreateCmd)
	featuresCmd.AddCommand(featuresUpdateCmd)
	featuresCmd.AddCommand(featuresDeleteCmd)

	// Contexts (admin operations)
	adminCmd.AddCommand(contextsCmd)

	// Tenants
	adminCmd.AddCommand(adminTenantsCmd)
	adminTenantsCmd.AddCommand(adminTenantsListCmd)
	adminTenantsCmd.AddCommand(adminTenantsGetCmd)
	adminTenantsCmd.AddCommand(adminTenantsCreateCmd)
	adminTenantsCmd.AddCommand(adminTenantsUpdateCmd)
	adminTenantsCmd.AddCommand(adminTenantsDeleteCmd)

	adminTenantsCreateCmd.Flags().StringVar(&tenantDesc, "description", "", "Tenant description")
	adminTenantsCreateCmd.Flags().StringVar(&tenantData, "data", "", "JSON tenant data")
	adminTenantsUpdateCmd.Flags().StringVar(&tenantDesc, "description", "", "Tenant description")
	adminTenantsUpdateCmd.Flags().StringVar(&tenantData, "data", "", "JSON tenant data")
	adminTenantsDeleteCmd.Flags().BoolVarP(&tenantsDeleteForce, "force", "f", false, "Skip confirmation prompt")

	// Projects
	adminCmd.AddCommand(adminProjectsCmd)
	adminProjectsCmd.AddCommand(adminProjectsListCmd)
	adminProjectsCmd.AddCommand(adminProjectsGetCmd)
	adminProjectsCmd.AddCommand(adminProjectsCreateCmd)
	adminProjectsCmd.AddCommand(adminProjectsDeleteCmd)

	adminProjectsCreateCmd.Flags().StringVar(&projectDesc, "description", "", "Project description")
	adminProjectsCreateCmd.Flags().StringVar(&projectData, "data", "", "JSON project data")
	adminProjectsDeleteCmd.Flags().BoolVarP(&projectsDeleteForce, "force", "f", false, "Skip confirmation prompt")

	// Tags
	adminCmd.AddCommand(adminTagsCmd)
	adminTagsCmd.AddCommand(adminTagsListCmd)
	adminTagsCmd.AddCommand(adminTagsGetCmd)
	adminTagsCmd.AddCommand(adminTagsCreateCmd)
	adminTagsCmd.AddCommand(adminTagsDeleteCmd)

	adminTagsCreateCmd.Flags().StringVar(&tagDesc, "description", "", "Tag description")
	adminTagsCreateCmd.Flags().StringVar(&tagData, "data", "", "JSON tag data")
	adminTagsDeleteCmd.Flags().BoolVarP(&tagsDeleteForce, "force", "f", false, "Skip confirmation prompt")

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
