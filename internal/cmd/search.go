package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"github.com/webskin/izanami-go-cli/internal/output"
)

var (
	searchFilters []string
)

var adminSearchCmd = &cobra.Command{
	Use:         "search <query>",
	Short:       "Global search across resources",
	Annotations: map[string]string{"route": "GET /api/admin/tenants/:tenant/search"},
	Long:        `Search across all resources in Izanami (or within a specific tenant).

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

		// For JSON output, use Identity mapper for raw JSON
		if outputFormat == "json" {
			raw, err := izanami.Search(client, ctx, cfg.Tenant, args[0], searchFilters, izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseSearchResults mapper and convert to table view
		results, err := izanami.Search(client, ctx, cfg.Tenant, args[0], searchFilters, izanami.ParseSearchResults)
		if err != nil {
			return err
		}

		tableViews := izanami.SearchResultsToTableView(results)
		return output.PrintTo(cmd.OutOrStdout(), tableViews, output.Format(outputFormat))
	},
}

func init() {
	// Search
	adminCmd.AddCommand(adminSearchCmd)
	adminSearchCmd.Flags().StringSliceVar(&searchFilters, "filter", []string{}, "Filter by resource type (PROJECT, FEATURE, KEY, TAG, SCRIPT, GLOBAL_CONTEXT, LOCAL_CONTEXT, WEBHOOK)")
}
