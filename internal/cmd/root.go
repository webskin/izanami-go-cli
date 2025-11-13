package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

var (
	// Global flags
	cfgFile      string
	baseURL      string
	clientID     string
	clientSecret string
	username     string
	token        string
	tenant       string
	project      string
	contextPath  string
	timeout      int
	verbose      bool
	outputFormat string

	// Global config
	cfg *izanami.Config
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "iz",
	Short: "Izanami CLI - Manage feature flags and contexts",
	Long: `A cross-platform command-line client for Izanami feature flag management.

Izanami is a feature flag and configuration management system. This CLI
allows you to interact with Izanami for both administration tasks and
standard feature flag operations.

Configuration can be provided via:
  - Config file: ~/.config/iz/config.yaml (or platform-equivalent)
  - Environment variables: IZ_BASE_URL, IZ_CLIENT_ID, IZ_CLIENT_SECRET, etc.
  - Command-line flags: --url, --client-id, --client-secret, etc.

Examples:
  # Check a feature flag
  iz features check my-feature --user user123 --context prod

  # List all features in a project
  iz features list --project my-project

  # Admin: Create a new project
  iz admin projects create new-project --description "My new project"

For more information, visit: https://github.com/MAIF/izanami`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for commands that don't need it
		skipCommands := []string{"completion", "version", "help", "login", "logout", "sessions"}
		for _, skip := range skipCommands {
			if cmd.Name() == skip || cmd.Parent() != nil && cmd.Parent().Name() == skip {
				return nil
			}
		}

		var err error

		// Priority 1: Try loading from active session
		cfg, _, err = izanami.LoadConfigFromSession()
		if err != nil {
			// Priority 2: Fall back to config file
			cfg, err = izanami.LoadConfig()
			if err != nil {
				return fmt.Errorf("no active session and failed to load config: %w (use 'iz login' to authenticate)", err)
			}
		}

		// Priority 3: Command-line flags override everything
		cfg.MergeWithFlags(baseURL, clientID, clientSecret, username, token, tenant, project, contextPath, timeout, verbose)

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&baseURL, "url", "", "Izanami base URL (env: IZ_BASE_URL)")
	rootCmd.PersistentFlags().StringVar(&clientID, "client-id", "", "Client ID for authentication (env: IZ_CLIENT_ID)")
	rootCmd.PersistentFlags().StringVar(&clientSecret, "client-secret", "", "Client secret for authentication (env: IZ_CLIENT_SECRET)")
	rootCmd.PersistentFlags().StringVar(&username, "username", "", "Username for admin authentication (env: IZ_USERNAME)")
	rootCmd.PersistentFlags().StringVar(&token, "token", "", "Personal access token for admin authentication (env: IZ_TOKEN)")
	rootCmd.PersistentFlags().StringVar(&tenant, "tenant", "", "Default tenant (env: IZ_TENANT)")
	rootCmd.PersistentFlags().StringVar(&project, "project", "", "Default project (env: IZ_PROJECT)")
	rootCmd.PersistentFlags().StringVar(&contextPath, "context", "", "Default context path (env: IZ_CONTEXT)")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 0, "Request timeout in seconds (default: 30)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "json", "Output format: json or table")
}

// GetConfig returns the global configuration
func GetConfig() *izanami.Config {
	return cfg
}

// GetOutputFormat returns the current output format
func GetOutputFormat() izanami.OutputFormat {
	if outputFormat == "table" {
		return izanami.OutputTable
	}
	return izanami.OutputJSON
}
