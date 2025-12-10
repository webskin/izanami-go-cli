package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"github.com/webskin/izanami-go-cli/internal/output"
)

var (
	// Project flags
	projectDesc string
	projectData string
	// Delete confirmation flag
	projectsDeleteForce bool
)

var adminProjectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "Manage projects",
	Long:  `Manage Izanami projects. Projects organize features within a tenant.`,
}

var adminProjectsListCmd = &cobra.Command{
	Use:         "list",
	Short:       "List all projects",
	Annotations: map[string]string{"route": "GET /api/admin/tenants/:tenant/projects"},
	RunE: func(cmd *cobra.Command, args []string) error {
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
			raw, err := izanami.ListProjects(client, ctx, cfg.Tenant, izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseProjects mapper
		projects, err := izanami.ListProjects(client, ctx, cfg.Tenant, izanami.ParseProjects)
		if err != nil {
			return err
		}

		return output.PrintTo(cmd.OutOrStdout(), projects, output.Format(outputFormat))
	},
}

var adminProjectsGetCmd = &cobra.Command{
	Use:         "get <project-name>",
	Short:       "Get a specific project",
	Annotations: map[string]string{"route": "GET /api/admin/tenants/:tenant/projects/:project"},
	Args:        cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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
			raw, err := izanami.GetProject(client, ctx, cfg.Tenant, args[0], izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseProject mapper
		project, err := izanami.GetProject(client, ctx, cfg.Tenant, args[0], izanami.ParseProject)
		if err != nil {
			return err
		}

		return output.PrintTo(cmd.OutOrStdout(), project, output.Format(outputFormat))
	},
}

var adminProjectsCreateCmd = &cobra.Command{
	Use:         "create <project-name>",
	Short:       "Create a new project",
	Annotations: map[string]string{"route": "POST /api/admin/tenants/:tenant/projects"},
	Args:        cobra.ExactArgs(1),
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

		fmt.Fprintf(cmd.OutOrStderr(), "Project created successfully: %s\n", projectName)
		return nil
	},
}

var adminProjectsUpdateCmd = &cobra.Command{
	Use:         "update <project-name>",
	Short:       "Update a project",
	Annotations: map[string]string{"route": "PUT /api/admin/tenants/:tenant/projects/:project"},
	Long: `Update a project's properties.

You can provide the updated data via:
  - --description flag (merged with existing data)
  - --data flag with JSON data
  - Both flags (--description takes precedence)

Examples:
  # Update description only
  iz admin projects update my-project --tenant my-tenant --description "New description"

  # Update with JSON data
  iz admin projects update my-project --tenant my-tenant --data '{"name":"my-project","description":"Updated"}'

  # Update with both (description flag takes precedence)
  iz admin projects update my-project --tenant my-tenant --data @project.json --description "Override desc"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		projectName := args[0]
		var data map[string]interface{}

		// Parse JSON data if provided
		if cmd.Flags().Changed("data") {
			var jsonData interface{}
			if err := parseJSONData(projectData, &jsonData); err != nil {
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
		data["name"] = projectName

		// Merge description flag if provided
		if cmd.Flags().Changed("description") {
			data["description"] = projectDesc
		}

		// Validate that we have at least name and description
		if _, hasDesc := data["description"]; !hasDesc {
			return fmt.Errorf("description is required (use --description flag or --data)")
		}

		ctx := context.Background()
		if err := client.UpdateProject(ctx, cfg.Tenant, projectName, data); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Project updated successfully: %s\n", projectName)
		return nil
	},
}

var adminProjectsDeleteCmd = &cobra.Command{
	Use:         "delete <project-name>",
	Short:       "Delete a project",
	Annotations: map[string]string{"route": "DELETE /api/admin/tenants/:tenant/projects/:project"},
	Long:        `Delete a project. WARNING: This will delete all features in the project.`,
	Args:        cobra.ExactArgs(1),
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

		fmt.Fprintf(cmd.OutOrStderr(), "Project deleted successfully: %s\n", projectName)
		return nil
	},
}

var adminProjectsLogsCmd = &cobra.Command{
	Use:         "logs <project-name>",
	Short:       "View project event logs",
	Annotations: map[string]string{"route": "GET /api/admin/tenants/:tenant/projects/:project/logs"},
	Long: `View event logs for a project. Shows audit events like feature changes, user actions, etc.

Examples:
  # List recent logs for a project
  iz admin projects logs my-project --tenant my-tenant

  # List logs with filters
  iz admin projects logs my-project --tenant my-tenant --users admin,user1
  iz admin projects logs my-project --tenant my-tenant --types FEATURE_CREATED,FEATURE_UPDATED

  # List logs in descending order (newest first)
  iz admin projects logs my-project --tenant my-tenant --order desc

  # List logs within a time range
  iz admin projects logs my-project --tenant my-tenant --start 2024-01-01T00:00:00Z --end 2024-01-31T23:59:59Z

  # Paginate through logs
  iz admin projects logs my-project --tenant my-tenant --count 100
  iz admin projects logs my-project --tenant my-tenant --count 50 --cursor 12345`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		projectName := args[0]

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		opts := &izanami.LogsRequest{
			Order:    logsOrder,
			Users:    logsUsers,
			Types:    logsTypes,
			Features: logsFeatures,
			Start:    logsStart,
			End:      logsEnd,
			Cursor:   logsCursor,
			Count:    logsCount,
			Total:    logsTotal,
		}

		ctx := context.Background()

		// For JSON output, use Identity mapper to get raw JSON
		if outputFormat == "json" {
			raw, err := izanami.ListProjectLogs(client, ctx, cfg.Tenant, projectName, opts, izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseLogsResponse mapper
		logs, err := izanami.ListProjectLogs(client, ctx, cfg.Tenant, projectName, opts, izanami.ParseLogsResponse)
		if err != nil {
			return err
		}

		return output.PrintTo(cmd.OutOrStdout(), logs.ToTableView(), output.Format(outputFormat))
	},
}

func init() {
	// Projects
	adminCmd.AddCommand(adminProjectsCmd)
	adminProjectsCmd.AddCommand(adminProjectsListCmd)
	adminProjectsCmd.AddCommand(adminProjectsGetCmd)
	adminProjectsCmd.AddCommand(adminProjectsCreateCmd)
	adminProjectsCmd.AddCommand(adminProjectsUpdateCmd)
	adminProjectsCmd.AddCommand(adminProjectsDeleteCmd)

	// Dynamic completion for project name argument
	adminProjectsGetCmd.ValidArgsFunction = completeProjectNames
	adminProjectsUpdateCmd.ValidArgsFunction = completeProjectNames
	adminProjectsDeleteCmd.ValidArgsFunction = completeProjectNames
	adminProjectsLogsCmd.ValidArgsFunction = completeProjectNames

	adminProjectsCreateCmd.Flags().StringVar(&projectDesc, "description", "", "Project description")
	adminProjectsCreateCmd.Flags().StringVar(&projectData, "data", "", "JSON project data")
	adminProjectsUpdateCmd.Flags().StringVar(&projectDesc, "description", "", "Project description")
	adminProjectsUpdateCmd.Flags().StringVar(&projectData, "data", "", "JSON project data")
	adminProjectsDeleteCmd.Flags().BoolVarP(&projectsDeleteForce, "force", "f", false, "Skip confirmation prompt")

	// Project logs
	adminProjectsCmd.AddCommand(adminProjectsLogsCmd)
	adminProjectsLogsCmd.Flags().StringVar(&logsOrder, "order", "", "Sort order: asc or desc")
	adminProjectsLogsCmd.Flags().StringVar(&logsUsers, "users", "", "Filter by users (comma-separated)")
	adminProjectsLogsCmd.Flags().StringVar(&logsTypes, "types", "", "Filter by event types (comma-separated)")
	adminProjectsLogsCmd.Flags().StringVar(&logsFeatures, "features", "", "Filter by features (comma-separated)")
	adminProjectsLogsCmd.Flags().StringVar(&logsStart, "start", "", "Start date-time (ISO 8601)")
	adminProjectsLogsCmd.Flags().StringVar(&logsEnd, "end", "", "End date-time (ISO 8601)")
	adminProjectsLogsCmd.Flags().Int64Var(&logsCursor, "cursor", 0, "Cursor for pagination")
	adminProjectsLogsCmd.Flags().IntVar(&logsCount, "count", 50, "Number of logs to retrieve")
	adminProjectsLogsCmd.Flags().BoolVar(&logsTotal, "total", false, "Include total count in response")
}
