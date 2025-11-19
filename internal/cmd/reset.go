package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// resetCmd represents the reset command
var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset all CLI data (config and sessions)",
	Long: `Reset all CLI configuration and session data by deleting files.

This will:
  1. Backup existing files with timestamps
  2. Delete config file (~/.config/iz/config.yaml)
  3. Delete sessions file (~/.izsessions)

Backups are created with format: {filename}.backup.{timestamp}
Example: config.yaml.backup.20250119_143025

After reset, you will need to run 'iz login' and reconfigure.

Note: This does not affect environment variables or command-line flags.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")

		configPath := izanami.GetConfigPath()
		sessionsPath := izanami.GetSessionsPath()

		// Check which files exist
		configExists := izanami.ConfigExists()
		sessionsExists := fileExists(sessionsPath)

		if !configExists && !sessionsExists {
			return fmt.Errorf("no configuration or session files found - nothing to reset")
		}

		// Show what will be deleted
		fmt.Fprintln(cmd.OutOrStdout(), "The following files will be backed up and deleted:")
		if configExists {
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", configPath)
		}
		if sessionsExists {
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", sessionsPath)
		}
		fmt.Fprintln(cmd.OutOrStdout())

		// Ask for confirmation unless --force is used
		if !force {
			fmt.Fprint(cmd.OutOrStdout(), "Are you sure? (y/N): ")
			reader := bufio.NewReader(cmd.InOrStdin())
			response, err := reader.ReadString('\n')
			if err != nil && err != io.EOF {
				// Only return error for actual read failures, not EOF
				return fmt.Errorf("failed to read input: %w", err)
			}
			response = strings.ToLower(strings.TrimSpace(response))

			if response != "y" && response != "yes" {
				fmt.Fprintln(cmd.OutOrStdout(), "Cancelled")
				return nil
			}
		}

		// Create timestamp for backups
		timestamp := time.Now().Format("20060102_150405")

		// Backup and delete config file
		if configExists {
			backupPath := fmt.Sprintf("%s.backup.%s", configPath, timestamp)
			if err := backupFile(configPath, backupPath); err != nil {
				return fmt.Errorf("failed to backup config file: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Config backed up to: %s\n", backupPath)

			if err := os.Remove(configPath); err != nil {
				return fmt.Errorf("failed to delete config file: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Config deleted: %s\n", configPath)
		}

		// Backup and delete sessions file
		if sessionsExists {
			backupPath := fmt.Sprintf("%s.backup.%s", sessionsPath, timestamp)
			if err := backupFile(sessionsPath, backupPath); err != nil {
				return fmt.Errorf("failed to backup sessions file: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Sessions backed up to: %s\n", backupPath)

			if err := os.Remove(sessionsPath); err != nil {
				return fmt.Errorf("failed to delete sessions file: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Sessions deleted: %s\n", sessionsPath)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "\n✓ Reset complete!")
		fmt.Fprintln(cmd.OutOrStdout(), "\nNext steps:")
		fmt.Fprintln(cmd.OutOrStdout(), "  1. Run 'iz login' to authenticate")
		fmt.Fprintln(cmd.OutOrStdout(), "  2. Run 'iz config init' to create a new config (optional)")
		fmt.Fprintln(cmd.OutOrStdout(), "\nYour backups are preserved in case you need to restore.")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
	resetCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// backupFile creates a backup copy of a file
func backupFile(srcPath, dstPath string) error {
	// Read source file
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	// Ensure backup directory exists
	backupDir := filepath.Dir(dstPath)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return err
	}

	// Write backup with same permissions as original
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	return os.WriteFile(dstPath, data, srcInfo.Mode())
}
