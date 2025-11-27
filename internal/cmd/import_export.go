package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

var (
	exportOutput   string
	importConflict string
	importTimezone string
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
			fmt.Fprintf(cmd.OutOrStderr(), "Export written to: %s\n", exportOutput)
		} else {
			fmt.Fprint(cmd.OutOrStdout(), data)
		}

		return nil
	},
}

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

		fmt.Fprintf(cmd.OutOrStderr(), "Import started: %s\n", status.ID)
		fmt.Fprintf(cmd.OutOrStderr(), "Status: %s\n", status.Status)
		if status.Message != "" {
			fmt.Fprintf(cmd.OutOrStderr(), "Message: %s\n", status.Message)
		}

		return nil
	},
}

func init() {
	// Import/Export
	adminCmd.AddCommand(adminExportCmd)
	adminCmd.AddCommand(adminImportCmd)

	adminExportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file (default: stdout)")
	adminImportCmd.Flags().StringVar(&importConflict, "conflict", "FAIL", "Conflict resolution: FAIL, SKIP, OVERWRITE")
	adminImportCmd.Flags().StringVar(&importTimezone, "timezone", "", "Timezone for time-based features")
}
