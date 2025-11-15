package cmd

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration file",
	Long: `Create a sample configuration file at the default location.

The configuration file will be created at:
  - Linux/macOS: ~/.config/iz/config.yaml
  - Windows: %APPDATA%\iz\config.yaml

The file will contain example configuration with helpful comments.
You can then edit this file to add your Izanami credentials and settings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := izanami.InitConfigFile(); err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}

		configDir := getConfigDirForDisplay()
		configPath := filepath.Join(configDir, "config.yaml")

		fmt.Printf("✓ Configuration file created at: %s\n", configPath)
		fmt.Println("\nNext steps:")
		fmt.Println("  1. Edit the config file and add your Izanami settings")
		fmt.Println("  2. Or use environment variables (IZ_BASE_URL, IZ_CLIENT_ID, etc.)")
		fmt.Println("  3. Or use command-line flags (--url, --client-id, etc.)")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

// getConfigDirForDisplay returns a user-friendly display of the config directory
func getConfigDirForDisplay() string {
	switch runtime.GOOS {
	case "windows":
		return "%APPDATA%\\iz"
	case "darwin", "linux":
		return "~/.config/iz"
	default:
		return "~/.config/iz"
	}
}
