package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"github.com/webskin/izanami-go-cli/internal/output"
)

var (
	// Tenant flags
	tenantDesc string
	tenantData string
	// Delete confirmation flag
	tenantsDeleteForce bool
	// Logs command flags
	logsOrder    string
	logsUsers    string
	logsTypes    string
	logsFeatures string
	logsProjects string
	logsStart    string
	logsEnd      string
	logsCursor   int64
	logsCount    int
	logsTotal    bool
)

// adminTenantsCmd represents the admin tenants command
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

		// For JSON output, use Identity mapper to get raw JSON
		if outputFormat == "json" {
			raw, err := izanami.ListTenants(client, ctx, nil, izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseTenants mapper
		tenants, err := izanami.ListTenants(client, ctx, nil, izanami.ParseTenants)
		if err != nil {
			return err
		}

		// Convert to summaries for table output (list endpoint doesn't include projects/tags)
		summaries := make([]izanami.TenantSummary, len(tenants))
		for i, t := range tenants {
			summaries[i] = izanami.TenantSummary{
				Name:        t.Name,
				Description: t.Description,
			}
		}

		return output.PrintTo(cmd.OutOrStdout(), summaries, output.Format(outputFormat))
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

		// For JSON output, use Identity mapper to get raw JSON
		if outputFormat == "json" {
			raw, err := izanami.GetTenant(client, ctx, args[0], izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseTenant mapper
		tenant, err := izanami.GetTenant(client, ctx, args[0], izanami.ParseTenant)
		if err != nil {
			return err
		}

		return output.PrintTo(cmd.OutOrStdout(), tenant, output.Format(outputFormat))
	},
}

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

		fmt.Fprintf(cmd.OutOrStderr(), "Tenant created successfully: %s\n", tenantName)
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

		fmt.Fprintf(cmd.OutOrStderr(), "Tenant updated successfully: %s\n", tenantName)
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

		fmt.Fprintf(cmd.OutOrStderr(), "Tenant deleted successfully: %s\n", tenantName)
		return nil
	},
}

var adminTenantsLogsCmd = &cobra.Command{
	Use:   "logs <tenant-name>",
	Short: "View tenant event logs",
	Long: `View event logs for a tenant. Shows audit events like feature changes, user actions, etc.

Examples:
  # List recent logs for a tenant
  iz admin tenants logs my-tenant

  # List logs with filters
  iz admin tenants logs my-tenant --users admin,user1
  iz admin tenants logs my-tenant --types FEATURE_CREATED,FEATURE_UPDATED

  # List logs in descending order (newest first)
  iz admin tenants logs my-tenant --order desc

  # List logs within a time range
  iz admin tenants logs my-tenant --start 2024-01-01T00:00:00Z --end 2024-01-31T23:59:59Z

  # Paginate through logs
  iz admin tenants logs my-tenant --count 100
  iz admin tenants logs my-tenant --count 50 --cursor 12345`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tenantName := args[0]

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		opts := &izanami.LogsRequest{
			Order:    logsOrder,
			Users:    logsUsers,
			Types:    logsTypes,
			Features: logsFeatures,
			Projects: logsProjects,
			Start:    logsStart,
			End:      logsEnd,
			Cursor:   logsCursor,
			Count:    logsCount,
			Total:    logsTotal,
		}

		ctx := context.Background()

		// For JSON output, use Identity mapper to get raw JSON
		if outputFormat == "json" {
			raw, err := izanami.ListTenantLogs(client, ctx, tenantName, opts, izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseLogsResponse mapper
		logs, err := izanami.ListTenantLogs(client, ctx, tenantName, opts, izanami.ParseLogsResponse)
		if err != nil {
			return err
		}

		return output.PrintTo(cmd.OutOrStdout(), logs.ToTableView(), output.Format(outputFormat))
	},
}

func init() {
	// Tenants
	adminCmd.AddCommand(adminTenantsCmd)
	adminTenantsCmd.AddCommand(adminTenantsListCmd)
	adminTenantsCmd.AddCommand(adminTenantsGetCmd)
	adminTenantsCmd.AddCommand(adminTenantsCreateCmd)
	adminTenantsCmd.AddCommand(adminTenantsUpdateCmd)
	adminTenantsCmd.AddCommand(adminTenantsDeleteCmd)

	// Dynamic completion for tenant name argument
	adminTenantsGetCmd.ValidArgsFunction = completeTenantNames

	adminTenantsCreateCmd.Flags().StringVar(&tenantDesc, "description", "", "Tenant description")
	adminTenantsCreateCmd.Flags().StringVar(&tenantData, "data", "", "JSON tenant data")
	adminTenantsUpdateCmd.Flags().StringVar(&tenantDesc, "description", "", "Tenant description")
	adminTenantsUpdateCmd.Flags().StringVar(&tenantData, "data", "", "JSON tenant data")
	adminTenantsDeleteCmd.Flags().BoolVarP(&tenantsDeleteForce, "force", "f", false, "Skip confirmation prompt")

	// Tenant logs
	adminTenantsCmd.AddCommand(adminTenantsLogsCmd)
	adminTenantsLogsCmd.Flags().StringVar(&logsOrder, "order", "", "Sort order: asc or desc")
	adminTenantsLogsCmd.Flags().StringVar(&logsUsers, "users", "", "Filter by users (comma-separated)")
	adminTenantsLogsCmd.Flags().StringVar(&logsTypes, "types", "", "Filter by event types (comma-separated)")
	adminTenantsLogsCmd.Flags().StringVar(&logsFeatures, "features", "", "Filter by features (comma-separated)")
	adminTenantsLogsCmd.Flags().StringVar(&logsProjects, "projects", "", "Filter by projects (comma-separated)")
	adminTenantsLogsCmd.Flags().StringVar(&logsStart, "start", "", "Start date-time (ISO 8601)")
	adminTenantsLogsCmd.Flags().StringVar(&logsEnd, "end", "", "End date-time (ISO 8601)")
	adminTenantsLogsCmd.Flags().Int64Var(&logsCursor, "cursor", 0, "Cursor for pagination")
	adminTenantsLogsCmd.Flags().IntVar(&logsCount, "count", 50, "Number of logs to retrieve")
	adminTenantsLogsCmd.Flags().BoolVar(&logsTotal, "total", false, "Include total count in response")
}
