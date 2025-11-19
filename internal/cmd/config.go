package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// Global configuration keys and their descriptions
var globalConfigKeys = map[string]string{
	"timeout":       "Request timeout in seconds",
	"verbose":       "Verbose output (true/false)",
	"output-format": "Default output format (table/json)",
	"color":         "Color output (auto/always/never)",
}

// Profile-specific configuration keys and their descriptions
var profileConfigKeys = map[string]string{
	"base-url":                       "Izanami server URL",
	"tenant":                         "Default tenant name",
	"project":                        "Default project name",
	"context":                        "Default context path",
	"personal-access-token-username": "Username for PAT authentication",
	"personal-access-token":          "Personal access token",
}

// printValidConfigKeys prints all valid configuration keys categorized
func printValidConfigKeys() {
	// Sort keys for consistent output
	globalKeys := make([]string, 0, len(globalConfigKeys))
	for key := range globalConfigKeys {
		globalKeys = append(globalKeys, key)
	}
	sort.Strings(globalKeys)

	profileKeys := make([]string, 0, len(profileConfigKeys))
	for key := range profileConfigKeys {
		profileKeys = append(profileKeys, key)
	}
	sort.Strings(profileKeys)

	// Print global keys
	fmt.Println("Global configuration keys (apply to all profiles):")
	for _, key := range globalKeys {
		fmt.Printf("  %-33s - %s\n", key, globalConfigKeys[key])
	}
	fmt.Println()

	// Print profile-specific keys
	fmt.Println("Profile-specific keys (use 'iz profiles set' for active profile):")
	for _, key := range profileKeys {
		fmt.Printf("  %-33s - %s\n", key, profileConfigKeys[key])
	}
	fmt.Println()

	// Print usage
	fmt.Println("Usage:")
	fmt.Println("  iz config set <key> <value>           - Set global config")
	fmt.Println("  iz profiles set <key> <value>         - Set in active profile")
	fmt.Println("  iz profiles client-keys add           - Add client credentials")
}

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
	Long: `Manage CLI configuration settings.

Configuration can be set via:
  - Config file (~/.config/iz/config.yaml)
  - Environment variables (IZ_*)
  - Command-line flags

Use subcommands to view and modify configuration.`,
}

// buildConfigSetLongHelp builds the long help text for config set command
func buildConfigSetLongHelp() string {
	var sb strings.Builder
	sb.WriteString("Set a configuration value and persist it to the config file.\n\n")

	// Sort and print global keys
	globalKeys := make([]string, 0, len(globalConfigKeys))
	for key := range globalConfigKeys {
		globalKeys = append(globalKeys, key)
	}
	sort.Strings(globalKeys)

	sb.WriteString("Global configuration keys (apply to all profiles):\n")
	for _, key := range globalKeys {
		sb.WriteString(fmt.Sprintf("  %-33s - %s\n", key, globalConfigKeys[key]))
	}
	sb.WriteString("\n")

	// Sort and print profile-specific keys
	profileKeys := make([]string, 0, len(profileConfigKeys))
	for key := range profileConfigKeys {
		profileKeys = append(profileKeys, key)
	}
	sort.Strings(profileKeys)

	sb.WriteString("Profile-specific keys (use 'iz profiles set' for active profile):\n")
	for _, key := range profileKeys {
		sb.WriteString(fmt.Sprintf("  %-33s - %s\n", key, profileConfigKeys[key]))
	}
	sb.WriteString("\n")

	sb.WriteString("Note: For client credentials, use 'iz profiles client-keys add'\n\n")

	sb.WriteString("Examples:\n")
	sb.WriteString("  iz config set timeout 60\n")
	sb.WriteString("  iz config set output-format json\n")
	sb.WriteString("  iz profiles set base-url http://localhost:9000\n")
	sb.WriteString("  iz profiles set tenant my-tenant")

	return sb.String()
}

// configSetCmd represents the config set command
var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long:  buildConfigSetLongHelp(),
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			// No arguments provided - show valid keys categorized
			printValidConfigKeys()
			return fmt.Errorf("missing required arguments")
		}
		if len(args) == 1 {
			return fmt.Errorf("missing value for key '%s'\nUsage: iz config set <key> <value>", args[0])
		}
		if len(args) > 2 {
			return fmt.Errorf("too many arguments\nUsage: iz config set <key> <value>")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		if err := izanami.SetConfigValue(key, value); err != nil {
			// If invalid key, show valid keys
			if strings.Contains(err.Error(), "invalid config key") {
				fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
				printValidConfigKeys()
				return fmt.Errorf("") // Return empty error since we already printed the message
			}
			return err
		}

		fmt.Printf("✓ Set %s = %s\n", key, value)

		// Warn about sensitive data storage
		if izanami.SensitiveKeys[key] {
			fmt.Println("\n⚠️  SECURITY WARNING:")
			fmt.Println("   Tokens are stored in plaintext in the config file.")
			fmt.Println("   File permissions are set to 0600 (owner read/write only).")
			fmt.Println("   Never commit config.yaml to version control.")
		}

		return nil
	},
}

// configGetCmd represents the config get command
var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Long: `Get a configuration value and show its source.

The source indicates where the value comes from:
  - file    : Set in config file
  - env     : Set via environment variable
  - default : Using default value
  - not set : No value configured

Example:
  iz config get base-url`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]

		configValue, err := izanami.GetConfigValue(key)
		if err != nil {
			return err
		}

		// Check if it's a sensitive key and should be redacted
		value := configValue.Value
		if izanami.SensitiveKeys[key] && value != "" {
			value = "<redacted>"
		}

		if configValue.Source == "not set" {
			fmt.Printf("%s: (not set)\n", key)
		} else {
			fmt.Printf("%s: %s (source: %s)\n", key, value, configValue.Source)
		}

		return nil
	},
}

// configUnsetCmd represents the config unset command
var configUnsetCmd = &cobra.Command{
	Use:   "unset <key>",
	Short: "Remove a configuration value",
	Long: `Remove a configuration value from the config file.

Note: This only removes the value from the config file.
Environment variables and defaults may still provide a value.

Example:
  iz config unset project`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]

		if err := izanami.UnsetConfigValue(key); err != nil {
			return err
		}

		fmt.Printf("✓ Removed %s from config file\n", key)
		return nil
	},
}

// configListCmd represents the config list command
var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configuration values",
	Long: `List all configuration values with their sources.

The output shows:
  - KEY    : Configuration key name
  - VALUE  : Current value (sensitive values shown as <redacted>)
  - SOURCE : Where the value comes from (file/env/default/not set)

Use --show-secrets to display sensitive values.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		showSecrets, _ := cmd.Flags().GetBool("show-secrets")

		allValues, err := izanami.GetAllConfigValues()
		if err != nil {
			return err
		}

		// Create table
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"KEY", "VALUE", "SOURCE"})
		table.SetBorder(false)
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetCenterSeparator("")
		table.SetColumnSeparator("")
		table.SetRowSeparator("")
		table.SetHeaderLine(false)
		table.SetTablePadding("\t")
		table.SetNoWhiteSpace(true)

		// Create sorted list of keys (client-keys are profile-specific, not shown here)
		keys := make([]string, 0, len(allValues))
		for key := range allValues {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		// Add config values
		for _, key := range keys {
			configValue := allValues[key]
			value := configValue.Value

			// Redact sensitive values unless --show-secrets is set
			if izanami.SensitiveKeys[key] && !showSecrets && value != "" {
				value = izanami.RedactedValue
			}

			// Show empty values as "(not set)"
			if value == "" {
				value = "(not set)"
			}

			table.Append([]string{key, value, configValue.Source})
		}

		table.Render()

		// Add note about profile-specific settings
		fmt.Fprintln(os.Stderr, "\nNote: Client keys are profile-specific. Use 'iz profiles client-keys' to manage them.")
		return nil
	},
}

// configPathCmd represents the config path command
var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show config file path",
	Long: `Show the path to the configuration file.

This is useful for troubleshooting configuration issues.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := izanami.GetConfigPath()
		configDir := izanami.GetConfigDirPath()

		fmt.Printf("Config file: %s\n", configPath)
		fmt.Printf("Config directory: %s\n", configDir)

		// Check if config file exists
		if izanami.ConfigExists() {
			fmt.Printf("Status: exists\n")
		} else {
			fmt.Printf("Status: not created (run 'iz config init' to create)\n")
		}

		// Show search paths
		fmt.Printf("\nConfig file search paths:\n")
		fmt.Printf("  1. %s\n", configPath)
		fmt.Printf("  2. ./config.yaml (current directory)\n")

		return nil
	},
}

// configInitCmd represents the config init command
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration file",
	Long: `Create a sample configuration file at the default location.

The configuration file will be created at:
  - Linux/macOS: ~/.config/iz/config.yaml
  - Windows: %APPDATA%\iz\config.yaml

The file will contain example configuration with helpful comments.
You can then edit this file to add your Izanami credentials and settings.

Use --defaults to create a config file with only default values (non-interactive).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		defaults, _ := cmd.Flags().GetBool("defaults")

		// For now, we only support the default behavior
		// Future enhancement: add interactive mode
		if !defaults {
			// Non-interactive for now - just create the file
		}

		if err := izanami.InitConfigFile(); err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}

		configDir := getConfigDirForDisplay()
		configPath := filepath.Join(configDir, "config.yaml")

		fmt.Printf("✓ Configuration file created at: %s\n", configPath)
		fmt.Println("\nNext steps:")
		fmt.Println("  1. Edit the config file and uncomment/set your values")
		fmt.Println("  2. Or use environment variables (IZ_BASE_URL, IZ_CLIENT_ID, etc.)")
		fmt.Println("  3. Or use command-line flags (--url, --client-id, etc.)")

		fmt.Println("\n⚠️  SECURITY NOTICE:")
		fmt.Println("   - File permissions set to 0600 (owner read/write only)")
		fmt.Println("   - Tokens stored in plaintext - never commit to version control")
		fmt.Println("   - Add config.yaml to .gitignore if using git")

		return nil
	},
}

// configValidateCmd represents the config validate command
var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate current configuration",
	Long: `Validate the current configuration and check for errors.

This command checks:
  - Configuration file syntax
  - Required fields
  - Valid values for fields

Exit codes:
  0 - Configuration is valid
  1 - Configuration has errors`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !izanami.ConfigExists() {
			fmt.Println("No configuration file found")
			fmt.Printf("Run 'iz config init' to create one at: %s\n", izanami.GetConfigPath())
			os.Exit(1)
		}

		errors := izanami.ValidateConfigFile()

		if len(errors) == 0 {
			fmt.Println("✓ Configuration is valid")
			return nil
		}

		fmt.Printf("✗ Configuration has %d error(s):\n\n", len(errors))
		for _, err := range errors {
			if err.Field == "general" {
				fmt.Printf("  - %s\n", err.Message)
			} else {
				fmt.Printf("  - %s: %s\n", err.Field, err.Message)
			}
		}

		os.Exit(1)
		return nil
	},
}

// configResetCmd represents the config reset command
var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to defaults",
	Long: `Reset configuration by deleting the config file.

This will delete the configuration file. You will need to run
'iz config init' to create a new one.

Note: This does not affect environment variables or command-line flags.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !izanami.ConfigExists() {
			return fmt.Errorf("config file does not exist")
		}

		// Ask for confirmation
		fmt.Printf("This will delete: %s\n", izanami.GetConfigPath())
		fmt.Print("Are you sure? (y/N): ")

		var response string
		fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))

		if response != "y" && response != "yes" {
			fmt.Println("Cancelled")
			return nil
		}

		if err := izanami.ResetConfig(); err != nil {
			return err
		}

		fmt.Println("✓ Configuration file deleted")
		fmt.Printf("Run 'iz config init' to create a new config file\n")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)

	// Add subcommands to config
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configUnsetCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configValidateCmd)
	configCmd.AddCommand(configResetCmd)

	// Add flags
	configListCmd.Flags().Bool("show-secrets", false, "Show sensitive values (tokens, secrets)")
	configInitCmd.Flags().Bool("defaults", false, "Create config with defaults only (non-interactive)")
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
