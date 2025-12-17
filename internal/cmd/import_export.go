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
	exportOutput   string
	importConflict string
	importTimezone string
	importVersion  int
)

var adminExportCmd = &cobra.Command{
	Use:         "export",
	Short:       "Export tenant data",
	Annotations: map[string]string{"route": "POST /api/admin/tenants/:tenant/_export"},
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
	Use:         "import <file>",
	Short:       "Import tenant data",
	Annotations: map[string]string{"route": "POST /api/admin/tenants/:tenant/_import"},
	Long: `Import data into a tenant from a newline-delimited JSON file.

Version (required):
  - 1: Import Izanami v1 data into v2 server (async migration)
  - 2: Import Izanami v2 data (synchronous)

Conflict resolution strategies:
  - FAIL: Stop on first conflict (default)
  - SKIP: Skip conflicting items
  - OVERWRITE: Overwrite existing items

Examples:
  # Import Izanami v2 data
  iz admin import export.ndjson --version 2

  # Import Izanami v1 data (migration from v1 to v2)
  iz admin import v1-export.ndjson --version 1 --timezone "Europe/Paris"

  # Import and overwrite conflicts
  iz admin import export.ndjson --version 2 --conflict OVERWRITE`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()

		if importVersion == 2 {
			return runImportV2(cmd, client, ctx, args[0])
		} else if importVersion == 1 {
			return runImportV1(cmd, client, ctx, args[0])
		} else {
			return fmt.Errorf("invalid version: %d (must be 1 or 2)", importVersion)
		}
	},
}

func runImportV2(cmd *cobra.Command, client *izanami.Client, ctx context.Context, filePath string) error {
	req := izanami.ImportRequest{
		Conflict: importConflict,
	}

	result, err := client.ImportV2(ctx, cfg.Tenant, filePath, req)

	// Handle conflict case - result is populated even on conflict error
	if apiErr, ok := err.(*izanami.APIError); ok && apiErr.StatusCode == 409 {
		// JSON output: return the result directly
		if outputFormat == "json" {
			return output.PrintTo(cmd.OutOrStdout(), result, output.JSON)
		}

		// Table output: formatted display
		fmt.Fprintf(cmd.OutOrStderr(), "⚠️  Import completed with conflicts\n\n")

		if len(result.Messages) > 0 {
			fmt.Fprintf(cmd.OutOrStderr(), "Messages:\n")
			for _, msg := range result.Messages {
				fmt.Fprintf(cmd.OutOrStderr(), "  • %s\n", msg)
			}
		}

		if len(result.Conflicts) > 0 {
			fmt.Fprintf(cmd.OutOrStderr(), "\nConflicts:\n")
			for _, conflict := range result.Conflicts {
				if conflict.Description != "" {
					fmt.Fprintf(cmd.OutOrStderr(), "  • %s (%s): %s\n", conflict.Name, conflict.ID, conflict.Description)
				} else {
					fmt.Fprintf(cmd.OutOrStderr(), "  • %s (%s)\n", conflict.Name, conflict.ID)
				}
			}
		}

		fmt.Fprintf(cmd.OutOrStderr(), "\nUse --conflict OVERWRITE or --conflict SKIP to handle conflicts.\n")
		return nil // Not a fatal error, just informational
	}

	if err != nil {
		return err
	}

	// JSON output: return the result directly
	if outputFormat == "json" {
		return output.PrintTo(cmd.OutOrStdout(), result, output.JSON)
	}

	// Table output: formatted display
	fmt.Fprintf(cmd.OutOrStderr(), "✅ Import completed successfully\n")

	if len(result.Messages) > 0 {
		fmt.Fprintf(cmd.OutOrStderr(), "\nMessages:\n")
		for _, msg := range result.Messages {
			fmt.Fprintf(cmd.OutOrStderr(), "  • %s\n", msg)
		}
	}

	return nil
}

func runImportV1(cmd *cobra.Command, client *izanami.Client, ctx context.Context, filePath string) error {
	if importTimezone == "" {
		return fmt.Errorf("--timezone is required for v1 imports")
	}

	req := izanami.ImportRequest{
		Conflict: importConflict,
		Timezone: importTimezone,
	}

	result, err := client.ImportV1(ctx, cfg.Tenant, filePath, req)
	if err != nil {
		return err
	}

	// JSON output: return the result directly
	if outputFormat == "json" {
		return output.PrintTo(cmd.OutOrStdout(), result, output.JSON)
	}

	// Table output: formatted display
	fmt.Fprintf(cmd.OutOrStderr(), "Import job started: %s\n", result.ID)
	fmt.Fprintf(cmd.OutOrStderr(), "V1 imports run asynchronously. Use 'iz admin import-status %s' to check progress.\n", result.ID)

	return nil
}

var adminImportStatusCmd = &cobra.Command{
	Use:         "import-status <import-id>",
	Short:       "Check status of async V1 import",
	Annotations: map[string]string{"route": "GET /api/admin/tenants/:tenant/_import/v1/:id"},
	Long: `Check the status of an asynchronous V1 import operation.

V1 imports run in the background. Use this command to check if the import
has completed, and to see any errors or warnings.

Status values:
  - Pending: Import is still running
  - Success: Import completed successfully
  - Failed: Import failed (check errors)

Examples:
  iz admin import-status 550e8400-e29b-41d4-a716-446655440000`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		status, err := client.GetImportStatus(ctx, cfg.Tenant, args[0])
		if err != nil {
			return err
		}

		// JSON output: return the status directly
		if outputFormat == "json" {
			return output.PrintTo(cmd.OutOrStdout(), status, output.JSON)
		}

		// Table output: formatted display
		switch status.Status {
		case "Success":
			fmt.Fprintf(cmd.OutOrStderr(), "✅ Import completed successfully\n\n")
			fmt.Fprintf(cmd.OutOrStderr(), "Imported:\n")
			fmt.Fprintf(cmd.OutOrStderr(), "  • Features: %d\n", status.Features)
			fmt.Fprintf(cmd.OutOrStderr(), "  • Users: %d\n", status.Users)
			fmt.Fprintf(cmd.OutOrStderr(), "  • Scripts: %d\n", status.Scripts)
			fmt.Fprintf(cmd.OutOrStderr(), "  • Keys: %d\n", status.Keys)

			if len(status.IncompatibleScripts) > 0 {
				fmt.Fprintf(cmd.OutOrStderr(), "\n⚠️  Incompatible scripts (not imported):\n")
				for _, script := range status.IncompatibleScripts {
					fmt.Fprintf(cmd.OutOrStderr(), "  • %s\n", script)
				}
			}

		case "Failed":
			fmt.Fprintf(cmd.OutOrStderr(), "❌ Import failed\n\n")
			if len(status.Errors) > 0 {
				fmt.Fprintf(cmd.OutOrStderr(), "Errors:\n")
				for _, err := range status.Errors {
					fmt.Fprintf(cmd.OutOrStderr(), "  • %s\n", err)
				}
			}

		case "Pending":
			fmt.Fprintf(cmd.OutOrStderr(), "⏳ Import is still running...\n")
			fmt.Fprintf(cmd.OutOrStderr(), "Run this command again to check progress.\n")

		default:
			fmt.Fprintf(cmd.OutOrStderr(), "Status: %s\n", status.Status)
		}

		return nil
	},
}

func init() {
	// Import/Export
	adminCmd.AddCommand(adminExportCmd)
	adminCmd.AddCommand(adminImportCmd)
	adminCmd.AddCommand(adminImportStatusCmd)

	adminExportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file (default: stdout)")
	adminImportCmd.Flags().IntVar(&importVersion, "version", 0, "Import version: 1 for v1 data migration, 2 for v2 data")
	_ = adminImportCmd.MarkFlagRequired("version")
	adminImportCmd.Flags().StringVar(&importConflict, "conflict", "FAIL", "Conflict resolution: FAIL, SKIP, OVERWRITE")
	adminImportCmd.Flags().StringVar(&importTimezone, "timezone", "", "Timezone for time-based features (required for v1)")
}
