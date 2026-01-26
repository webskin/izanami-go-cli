package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"github.com/webskin/izanami-go-cli/internal/output"
)

// healthCmd represents the health command
var healthCmd = &cobra.Command{
	Use:         "health",
	Short:       "Check Izanami server health",
	Annotations: map[string]string{"route": "GET /api/_health"},
	Long: `Check the health status of the Izanami server.

This command verifies that the Izanami instance is reachable and operational.
It returns the server status and version information.

Exit codes:
  0 - Server is healthy (status: UP)
  1 - Server is unhealthy or unreachable`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil || cfg.BaseURL == "" {
			return fmt.Errorf("base URL is required (set IZ_BASE_URL or --url)")
		}

		// Create a minimal client just for health check (no auth required)
		tempCfg := &izanami.Config{
			BaseURL: cfg.BaseURL,
			Timeout: cfg.Timeout,
			Verbose: cfg.Verbose,
		}

		client, err := izanami.NewAdminClientNoAuth(tempCfg)
		if err != nil {
			return err
		}

		ctx := context.Background()

		// For JSON output, use Identity mapper for raw JSON
		if outputFormat == "json" {
			raw, err := izanami.Health(client, ctx, izanami.Identity)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStderr(), "Error: %v\n", err)
				os.Exit(1)
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseHealthStatus mapper
		health, err := izanami.Health(client, ctx, izanami.ParseHealthStatus)
		if err != nil {
			fmt.Fprintf(cmd.OutOrStderr(), "Error: %v\n", err)
			os.Exit(1)
		}

		if !health.Database {
			fmt.Fprintf(cmd.OutOrStderr(), "Server is unhealthy: database check failed\n")
			output.PrintTo(cmd.OutOrStdout(), health, output.Format(outputFormat))
			os.Exit(1)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Status:  UP\n")
		fmt.Fprintf(cmd.OutOrStdout(), "Database: %v\n", health.Database)
		if health.Version != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Version: %s\n", health.Version)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "URL:     %s\n", cfg.BaseURL)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(healthCmd)
}
