package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"golang.org/x/term"
)

var (
	// Global flags
	cfgFile      string
	profileName  string
	baseURL      string
	tenant       string
	project      string
	contextPath  string
	timeout      int
	verbose      bool
	outputFormat string
	compactJSON  bool

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
  - Profiles: Named environment configurations (local, sandbox, build, prod)
  - Config file: ~/.config/iz/config.yaml (or platform-equivalent)
  - Environment variables: IZ_BASE_URL, IZ_TENANT, IZ_PROJECT, etc.
  - Command-line flags: --url, --tenant, --project, etc.

Examples:
  # Use a profile for all commands
  iz profiles use sandbox
  iz features check my-feature

  # Or override active profile temporarily
  iz features check my-feature --profile prod

  # List all features in a project
  iz features list --project my-project

  # Admin: Create a new project
  iz admin projects create new-project --description "My new project"

For more information, visit: https://github.com/MAIF/izanami`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for commands that don't need it
		skipCommands := []string{"completion", "version", "help", "login", "logout", "sessions", "config", "profiles", "reset"}
		for _, skip := range skipCommands {
			if cmd.Name() == skip || cmd.Parent() != nil && cmd.Parent().Name() == skip {
				return nil
			}
		}

		var err error

		// Load config with profile support
		// Priority order:
		// 1. Command-line flags (applied later)
		// 2. Environment variables (handled by viper)
		// 3. Profile settings (if --profile specified or active profile exists)
		// 4. Session settings (loaded via profile's session reference)
		// 5. Top-level config (fallback)

		// Load config via profile system (profiles load their referenced sessions)
		if profileName != "" {
			// Load with specific profile from --profile flag
			cfg, err = izanami.LoadConfigWithProfile(profileName)
		} else {
			// Load with active profile (if any)
			cfg, err = izanami.LoadConfigWithProfile("")
		}

		if err != nil {
			return fmt.Errorf("failed to load config: %w (use 'iz login' to authenticate)", err)
		}

		// Command-line flags override everything (highest priority)
		cfg.MergeWithFlags(izanami.FlagValues{
			BaseURL: baseURL,
			Tenant:  tenant,
			Project: project,
			Context: contextPath,
			Timeout: timeout,
			Verbose: verbose,
		})

		// Configure color output based on config setting
		configureColorOutput(cfg.Color)

		// Log authentication mode in verbose mode
		if cfg.Verbose {
			logAuthenticationMode(cmd, cfg)
			// Also log active profile if any
			if profileName != "" {
				fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Using profile: %s (from --profile flag)\n", profileName)
			} else {
				activeProfile, err := izanami.GetActiveProfileName()
				if err == nil && activeProfile != "" {
					fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Using profile: %s (active profile)\n", activeProfile)
				}
			}
		}

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
	rootCmd.PersistentFlags().StringVarP(&profileName, "profile", "p", "", "Use specific profile (overrides active profile)")
	rootCmd.PersistentFlags().StringVar(&baseURL, "url", "", "Izanami base URL (env: IZ_BASE_URL)")
	rootCmd.PersistentFlags().StringVar(&tenant, "tenant", "", "Default tenant (env: IZ_TENANT)")
	rootCmd.PersistentFlags().StringVar(&project, "project", "", "Default project (env: IZ_PROJECT)")
	rootCmd.PersistentFlags().StringVar(&contextPath, "context", "", "Default context path (env: IZ_CONTEXT)")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 0, "Request timeout in seconds (default: 30)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "Output format: json or table")
	rootCmd.PersistentFlags().BoolVar(&compactJSON, "compact", false, "Output compact JSON (no pretty-printing)")
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

// logAuthenticationMode logs the available authentication modes
func logAuthenticationMode(cmd *cobra.Command, cfg *izanami.Config) {
	var adminAuth, clientAuth string

	// Check admin authentication (for admin operations)
	if cfg.PersonalAccessToken != "" {
		adminAuth = "Personal Access Token (PAT)"
	} else if cfg.JwtToken != "" {
		adminAuth = "JWT Cookie (session)"
	} else {
		adminAuth = "none"
	}

	// Check client authentication (for feature checks)
	if cfg.ClientID != "" && cfg.ClientSecret != "" {
		clientAuth = "Client API Key"
	} else {
		clientAuth = "none"
	}

	fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Authentication - Admin operations: %s, Feature checks: %s\n", adminAuth, clientAuth)
}

// configureColorOutput configures color output based on the color setting
func configureColorOutput(colorSetting string) {
	switch colorSetting {
	case "never":
		// Disable all colors
		color.NoColor = true
	case "always":
		// Force colors even if not a TTY
		color.NoColor = false
	case "auto", "":
		// Auto-detect: enable colors only if stdout is a terminal
		color.NoColor = !term.IsTerminal(int(os.Stdout.Fd()))
	}
}
