package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"golang.org/x/term"
)

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

// configSetCmd represents the config set command
var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a configuration value and persist it to the config file.

Valid configuration keys:
  base-url              - Izanami server URL
  tenant                - Default tenant name
  project               - Default project name
  context               - Default context path
  client-id             - Client ID for authentication
  client-secret         - Client secret for authentication
  username              - Username for authentication
  personal-access-token - Personal access token
  timeout               - Request timeout in seconds
  output-format         - Default output format (table/json)
  color                 - Color output (auto/always/never)

Examples:
  iz config set base-url http://localhost:9000
  iz config set tenant my-tenant
  iz config set output-format json`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		if err := izanami.SetConfigValue(key, value); err != nil {
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

		// Load full config to get client-keys
		config, err := izanami.LoadConfig()
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

		// Create sorted list of keys (excluding client-keys, client-id, client-secret which we'll handle specially)
		keys := make([]string, 0, len(allValues))
		for key := range allValues {
			// Skip client-keys (shown in expanded format below)
			// Skip client-id and client-secret (now part of client-keys hierarchy)
			if key != "client-keys" && key != "client-id" && key != "client-secret" {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)

		// Add regular config values
		for _, key := range keys {
			configValue := allValues[key]
			value := configValue.Value

			// Redact sensitive values unless --show-secrets is set
			if izanami.SensitiveKeys[key] && !showSecrets && value != "" {
				value = "<redacted>"
			}

			// Show empty values as "(not set)"
			if value == "" {
				value = "(not set)"
			}

			table.Append([]string{key, value, configValue.Source})
		}

		// Add client-keys in expanded format
		if config.ClientKeys != nil && len(config.ClientKeys) > 0 {
			// Sort tenant names
			tenants := make([]string, 0, len(config.ClientKeys))
			for tenant := range config.ClientKeys {
				tenants = append(tenants, tenant)
			}
			sort.Strings(tenants)

			for _, tenant := range tenants {
				tenantConfig := config.ClientKeys[tenant]

				// Show tenant-level credentials
				if tenantConfig.ClientID != "" {
					clientID := tenantConfig.ClientID
					clientSecret := tenantConfig.ClientSecret
					if !showSecrets {
						if clientID != "" {
							clientID = "<redacted>"
						}
						if clientSecret != "" {
							clientSecret = "<redacted>"
						}
					}
					table.Append([]string{
						fmt.Sprintf("client-keys/%s/client-id", tenant),
						clientID,
						"file",
					})
					table.Append([]string{
						fmt.Sprintf("client-keys/%s/client-secret", tenant),
						clientSecret,
						"file",
					})
				}

				// Show project-level credentials
				if tenantConfig.Projects != nil && len(tenantConfig.Projects) > 0 {
					projectNames := make([]string, 0, len(tenantConfig.Projects))
					for project := range tenantConfig.Projects {
						projectNames = append(projectNames, project)
					}
					sort.Strings(projectNames)

					for _, project := range projectNames {
						projectConfig := tenantConfig.Projects[project]
						clientID := projectConfig.ClientID
						clientSecret := projectConfig.ClientSecret
						if !showSecrets {
							if clientID != "" {
								clientID = "<redacted>"
							}
							if clientSecret != "" {
								clientSecret = "<redacted>"
							}
						}
						table.Append([]string{
							fmt.Sprintf("client-keys/%s/%s/client-id", tenant, project),
							clientID,
							"file",
						})
						table.Append([]string{
							fmt.Sprintf("client-keys/%s/%s/client-secret", tenant, project),
							clientSecret,
							"file",
						})
					}
				}
			}
		} else {
			// No client-keys configured
			table.Append([]string{"client-keys", "(not set)", "file"})
		}

		table.Render()
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

// configClientKeysCmd represents the config client-keys command
var configClientKeysCmd = &cobra.Command{
	Use:   "client-keys",
	Short: "Manage client API keys",
	Long: `Manage client API keys (client-id/client-secret) for feature evaluation.

Client keys can be stored at the tenant level or per-project level in your
config file for convenient reuse across commands.`,
}

// configClientKeysAddCmd represents the config client-keys add command
var configClientKeysAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add client credentials for a tenant or project",
	Long: `Add client API credentials (client-id and client-secret) to the config file.

Credentials can be stored:
  - At the tenant level (for all projects in that tenant)
  - At the project level (for specific projects only)

The 'iz features check' command will automatically use these credentials with
the following precedence:
  1. --client-id/--client-secret flags (highest priority)
  2. IZ_CLIENT_ID/IZ_CLIENT_SECRET environment variables
  3. Stored credentials from config file (this command)

Examples:
  # Add tenant-wide credentials
  iz config client-keys add --tenant my-tenant

  # Add project-specific credentials
  iz config client-keys add --tenant my-tenant --project proj1 --project proj2

Security:
  Credentials are stored in plaintext in ~/.config/iz/config.yaml
  File permissions are automatically set to 0600 (owner read/write only)
  Never commit config.yaml to version control`,
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, _ := cmd.Flags().GetString("tenant")
		projects, _ := cmd.Flags().GetStringSlice("project")

		// Validate tenant is provided
		if tenant == "" {
			return fmt.Errorf("--tenant is required")
		}

		// Prompt for client-id
		fmt.Fprintf(os.Stderr, "Client ID: ")
		var clientID string
		if _, err := fmt.Scanln(&clientID); err != nil {
			return fmt.Errorf("failed to read client ID: %w", err)
		}
		clientID = strings.TrimSpace(clientID)
		if clientID == "" {
			return fmt.Errorf("client ID cannot be empty")
		}

		// Prompt for client-secret (hidden)
		fmt.Fprintf(os.Stderr, "Client Secret: ")
		secretBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(os.Stderr) // New line after password input
		if err != nil {
			return fmt.Errorf("failed to read client secret: %w", err)
		}
		clientSecret := strings.TrimSpace(string(secretBytes))
		if clientSecret == "" {
			return fmt.Errorf("client secret cannot be empty")
		}

		// Check if credentials already exist and prompt for confirmation
		config, err := izanami.LoadConfig()
		if err == nil && config.ClientKeys != nil {
			if tenantConfig, exists := config.ClientKeys[tenant]; exists {
				if len(projects) == 0 {
					// Check tenant-level credentials
					if tenantConfig.ClientID != "" {
						fmt.Fprintf(os.Stderr, "\n⚠️  Tenant '%s' already has credentials configured.\n", tenant)
						fmt.Fprintf(os.Stderr, "Overwrite existing credentials? (y/N): ")
						var response string
						fmt.Scanln(&response)
						if strings.ToLower(strings.TrimSpace(response)) != "y" {
							fmt.Fprintln(os.Stderr, "Aborted.")
							return nil
						}
					}
				} else {
					// Check project-level credentials
					if tenantConfig.Projects != nil {
						for _, project := range projects {
							if projConfig, projExists := tenantConfig.Projects[project]; projExists && projConfig.ClientID != "" {
								fmt.Fprintf(os.Stderr, "\n⚠️  Project '%s/%s' already has credentials configured.\n", tenant, project)
								fmt.Fprintf(os.Stderr, "Overwrite existing credentials? (y/N): ")
								var response string
								fmt.Scanln(&response)
								if strings.ToLower(strings.TrimSpace(response)) != "y" {
									fmt.Fprintln(os.Stderr, "Aborted.")
									return nil
								}
								break
							}
						}
					}
				}
			}
		}

		// Save credentials
		if err := izanami.AddClientKeys(tenant, projects, clientID, clientSecret); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}

		// Success message
		if len(projects) == 0 {
			fmt.Fprintf(os.Stderr, "\n✓ Client credentials saved for tenant '%s'\n", tenant)
		} else {
			fmt.Fprintf(os.Stderr, "\n✓ Client credentials saved for tenant '%s', projects: %s\n", tenant, strings.Join(projects, ", "))
		}

		fmt.Fprintln(os.Stderr, "\n⚠️  SECURITY WARNING:")
		fmt.Fprintln(os.Stderr, "   Credentials are stored in plaintext in the config file.")
		fmt.Fprintln(os.Stderr, "   File permissions are set to 0600 (owner read/write only).")
		fmt.Fprintln(os.Stderr, "   Never commit config.yaml to version control.")

		fmt.Fprintf(os.Stderr, "\nYou can now use these credentials with:\n")
		fmt.Fprintf(os.Stderr, "  iz features check --tenant %s <feature-id>\n", tenant)

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
	configCmd.AddCommand(configClientKeysCmd)

	// Add client-keys subcommands
	configClientKeysCmd.AddCommand(configClientKeysAddCmd)

	// Add flags
	configListCmd.Flags().Bool("show-secrets", false, "Show sensitive values (tokens, secrets)")
	configInitCmd.Flags().Bool("defaults", false, "Create config with defaults only (non-interactive)")

	// Add flags for client-keys add
	configClientKeysAddCmd.Flags().String("tenant", "", "Tenant name (required)")
	configClientKeysAddCmd.Flags().StringSlice("project", []string{}, "Project name(s) - can be specified multiple times")
	configClientKeysAddCmd.MarkFlagRequired("tenant")
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
