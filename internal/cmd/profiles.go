package cmd

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
	"syscall"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"golang.org/x/term"
)

// profileSettableKeys defines keys that can be set via 'iz profiles set'
// Maps key name to description
var profileSettableKeys = map[string]string{
	"base-url":                       "Izanami server URL",
	"client-base-url":                "Base URL for client operations (features/events)",
	"tenant":                         "Default tenant name",
	"project":                        "Default project name",
	"context":                        "Default context path",
	"session":                        "Session name to reference (clears base-url)",
	"personal-access-token":          "Personal access token",
	"personal-access-token-username": "Username for PAT authentication",
	"client-id":                      "Client ID for API authentication",
	"client-secret":                  "Client secret for API authentication",
}

var (
	profileDeleteForce bool
)

// profileCmd represents the profiles command
var profileCmd = &cobra.Command{
	Use:   "profiles",
	Short: "Manage environment profiles",
	Long: `Manage environment profiles for different Izanami servers.

Profiles allow you to maintain separate configurations for different
environments (local, sandbox, build, prod) and easily switch between them.

Each profile can include:
  - Server URL or reference to a login session (for JWT auth)
  - Personal Access Token (PAT) for long-lived authentication
  - Default tenant, project, and context
  - Client credentials for feature checks

Note: JWT tokens (short-lived) are only stored in sessions via 'iz login',
not in profiles. Use session references in profiles for JWT authentication.

Use subcommands to create, list, switch, and manage profiles.`,
}

// profileListCmd represents the profile list command
var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles",
	Long: `List all configured profiles.

The active profile is marked with an asterisk (*).

Example:
  iz profiles list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles, activeProfile, err := izanami.ListProfiles()
		if err != nil {
			return err
		}

		if len(profiles) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No profiles configured")
			fmt.Fprintln(cmd.OutOrStdout(), "\nCreate a profile with:")
			fmt.Fprintln(cmd.OutOrStdout(), "  iz profiles add <name>")
			return nil
		}

		// Create table
		table := tablewriter.NewWriter(cmd.OutOrStdout())
		table.SetHeader([]string{"", "Name", "Session", "URL", "Tenant", "Project"})
		table.SetBorder(false)
		table.SetColumnSeparator("")
		table.SetHeaderLine(false)
		table.SetAutoWrapText(false)

		// Collect and sort profile names
		names := make([]string, 0, len(profiles))
		for name := range profiles {
			names = append(names, name)
		}
		// Sort alphabetically
		// Using a simple bubble sort for simplicity
		for i := 0; i < len(names); i++ {
			for j := i + 1; j < len(names); j++ {
				if names[i] > names[j] {
					names[i], names[j] = names[j], names[i]
				}
			}
		}

		for _, name := range names {
			profile := profiles[name]
			activeMarker := " "
			if name == activeProfile {
				activeMarker = "*"
			}

			session := profile.Session
			if session == "" {
				session = "-"
			}

			// Resolve URL: try profile.BaseURL first, then session.URL
			url := profile.BaseURL
			if url == "" && profile.Session != "" {
				// Profile references a session - get URL from session
				sessions, err := izanami.LoadSessions()
				if err == nil {
					sessionData, err := sessions.GetSession(profile.Session)
					if err == nil && sessionData.URL != "" {
						url = sessionData.URL
					}
				}
			}
			if url == "" {
				url = "-"
			}

			tenant := profile.Tenant
			if tenant == "" {
				tenant = "-"
			}

			project := profile.Project
			if project == "" {
				project = "-"
			}

			table.Append([]string{activeMarker, name, session, url, tenant, project})
		}

		table.Render()

		if activeProfile != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "\nActive profile: %s\n", activeProfile)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "\nNo active profile set")
			fmt.Fprintln(cmd.OutOrStdout(), "Switch to a profile with: iz profiles use <name>")
		}

		return nil
	},
}

// profileCurrentCmd represents the profile current command
var profileCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show current active profile",
	Long: `Show the currently active profile with all its settings.

Example:
  iz profiles current`,
	RunE: func(cmd *cobra.Command, args []string) error {
		activeProfileName, err := izanami.GetActiveProfileName()
		if err != nil {
			return err
		}

		if activeProfileName == "" {
			fmt.Fprintln(cmd.OutOrStdout(), "No active profile set")
			fmt.Fprintln(cmd.OutOrStdout(), "\nSet a profile with:")
			fmt.Fprintln(cmd.OutOrStdout(), "  iz profiles use <name>")
			return nil
		}

		profile, err := izanami.GetProfile(activeProfileName)
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Active Profile: %s\n\n", activeProfileName)
		printProfile(cmd.OutOrStdout(), profile, false)

		return nil
	},
}

// profileShowCmd represents the profile show command
var profileShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show profile details",
	Long: `Show detailed configuration for a specific profile.

Example:
  iz profiles show sandbox`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := args[0]

		profile, err := izanami.GetProfile(profileName)
		if err != nil {
			return err
		}

		showSecrets, _ := cmd.Flags().GetBool("show-secrets")

		fmt.Fprintf(cmd.OutOrStdout(), "Profile: %s\n\n", profileName)
		printProfile(cmd.OutOrStdout(), profile, showSecrets)

		return nil
	},
}

// profileUseCmd represents the profile use command
var profileUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Switch active profile",
	Long: `Switch to a different profile.

The active profile's settings will be used as defaults for all commands.

Example:
  iz profiles use sandbox`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := args[0]

		if err := izanami.SetActiveProfile(profileName); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "✓ Switched to profile '%s'\n", profileName)

		// Show brief profile info
		profile, err := izanami.GetProfile(profileName)
		if err == nil {
			fmt.Fprintln(cmd.OutOrStdout())
			if profile.Session != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  Session: %s\n", profile.Session)
			}
			if profile.BaseURL != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  URL:     %s\n", profile.BaseURL)
			}
			if profile.Tenant != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  Tenant:  %s\n", profile.Tenant)
			}
		}

		return nil
	},
}

// profileAddCmd represents the profile add command
var profileAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new profile",
	Long: `Add a new profile with interactive prompts.

You can specify settings via flags or be prompted interactively.

Examples:
  # Interactive mode
  iz profiles add sandbox

  # With flags
  iz profiles add sandbox --url http://localhost:9000 --tenant dev-tenant

  # Full non-interactive setup with client credentials
  iz profiles add myprofile \
    --url http://localhost:9000 \
    --tenant test-tenant \
    --project test-project \
    --context PROD \
    --client-id my-client-id \
    --client-secret my-secret \
    --client-base-url http://worker.localhost:9000
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := args[0]

		// Get flag values
		url, _ := cmd.Flags().GetString("url")
		tenant, _ := cmd.Flags().GetString("tenant")
		project, _ := cmd.Flags().GetString("project")
		context, _ := cmd.Flags().GetString("context")
		clientID, _ := cmd.Flags().GetString("client-id")
		clientSecret, _ := cmd.Flags().GetString("client-secret")
		clientBaseURL, _ := cmd.Flags().GetString("client-base-url")
		interactive, _ := cmd.Flags().GetBool("interactive")

		profile := &izanami.Profile{}

		// Interactive prompts if --interactive or no flags provided
		if interactive || (url == "" && tenant == "" && project == "" && context == "" && clientID == "" && clientSecret == "" && clientBaseURL == "") {
			fmt.Fprintf(cmd.OutOrStdout(), "Creating profile '%s'\n\n", profileName)
			reader := bufio.NewReader(cmd.InOrStdin())

			// Server URL
			fmt.Fprint(cmd.OutOrStdout(), "Server URL: ")
			url, _ = reader.ReadString('\n')
			url = strings.TrimSpace(url)

			// Tenant
			fmt.Fprint(cmd.OutOrStdout(), "Default tenant (optional): ")
			tenant, _ = reader.ReadString('\n')
			tenant = strings.TrimSpace(tenant)

			// Project
			fmt.Fprint(cmd.OutOrStdout(), "Default project (optional): ")
			project, _ = reader.ReadString('\n')
			project = strings.TrimSpace(project)

			// Context
			fmt.Fprint(cmd.OutOrStdout(), "Default context (optional): ")
			context, _ = reader.ReadString('\n')
			context = strings.TrimSpace(context)

			// Client credentials
			fmt.Fprint(cmd.OutOrStdout(), "\nConfigure client credentials for feature checks? (y/N): ")
			addCreds, _ := reader.ReadString('\n')
			if strings.ToLower(strings.TrimSpace(addCreds)) == "y" {
				fmt.Fprint(cmd.OutOrStdout(), "Client ID: ")
				clientID, _ = reader.ReadString('\n')
				clientID = strings.TrimSpace(clientID)

				fmt.Fprint(cmd.OutOrStdout(), "Client Secret: ")
				secretBytes, err := term.ReadPassword(int(syscall.Stdin))
				fmt.Fprintln(cmd.OutOrStdout())
				if err != nil {
					return fmt.Errorf("failed to read client secret: %w", err)
				}
				clientSecret = strings.TrimSpace(string(secretBytes))

				fmt.Fprint(cmd.OutOrStdout(), "Client Base URL (optional, for separate client endpoint): ")
				clientBaseURL, _ = reader.ReadString('\n')
				clientBaseURL = strings.TrimSpace(clientBaseURL)
			}
		}

		// Validate: must have URL
		if url == "" {
			return fmt.Errorf("--url is required")
		}

		// Build profile
		profile.BaseURL = url
		profile.Tenant = tenant
		profile.Project = project
		profile.Context = context
		profile.ClientID = clientID
		profile.ClientSecret = clientSecret
		profile.ClientBaseURL = clientBaseURL

		// Save profile
		if err := izanami.AddProfile(profileName, profile); err != nil {
			return fmt.Errorf("failed to add profile: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "\n✓ Profile '%s' created successfully\n", profileName)

		// Check if this is the first/only profile
		profiles, _, err := izanami.ListProfiles()
		if err == nil && len(profiles) == 1 {
			fmt.Fprintln(cmd.OutOrStdout(), "✓ Set as active profile (first profile created)")
		}

		fmt.Fprintf(cmd.OutOrStdout(), "\nSwitch to this profile with:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  iz profiles use %s\n", profileName)

		if clientSecret != "" {
			fmt.Fprintln(cmd.OutOrStdout(), "\n⚠️  SECURITY WARNING:")
			fmt.Fprintln(cmd.OutOrStdout(), "   Credentials are stored in plaintext in the config file.")
			fmt.Fprintln(cmd.OutOrStdout(), "   File permissions are set to 0600 (owner read/write only).")
			fmt.Fprintln(cmd.OutOrStdout(), "   Never commit config.yaml to version control.")
		}

		return nil
	},
}

// buildProfileSetLongHelp builds the long help text for profiles set command
func buildProfileSetLongHelp() string {
	var sb strings.Builder
	sb.WriteString("Update a specific setting in the active profile.\n\n")
	sb.WriteString("First switch to the profile you want to modify with 'iz profiles use <name>',\n")
	sb.WriteString("then use this command to update individual settings.\n\n")

	// Sort keys for consistent output
	keys := make([]string, 0, len(profileSettableKeys))
	for key := range profileSettableKeys {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	sb.WriteString("Profile-specific keys (settable via 'iz profiles set'):\n")
	for _, key := range keys {
		sb.WriteString(fmt.Sprintf("  %-33s - %s\n", key, profileSettableKeys[key]))
	}

	sb.WriteString("\nExamples:\n")
	sb.WriteString("  # First switch to the profile you want to modify\n")
	sb.WriteString("  iz profiles use sandbox\n\n")
	sb.WriteString("  # Then set values on the active profile\n")
	sb.WriteString("  iz profiles set tenant new-tenant\n")
	sb.WriteString("  iz profiles set base-url https://izanami.example.com\n")
	sb.WriteString("  iz profiles set session sandbox-session\n")
	sb.WriteString("  iz profiles set client-id my-client-id\n")
	sb.WriteString("  iz profiles set client-base-url https://worker.example.com\n")
	sb.WriteString("  iz profiles set personal-access-token my-pat-token\n")

	return sb.String()
}

// printValidProfileKeys prints all valid profile keys
func printValidProfileKeys(w io.Writer) {
	// Sort keys for consistent output
	keys := make([]string, 0, len(profileSettableKeys))
	for key := range profileSettableKeys {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	fmt.Fprintln(w, "Profile-specific keys (settable via 'iz profiles set'):")
	for _, key := range keys {
		fmt.Fprintf(w, "  %-33s - %s\n", key, profileSettableKeys[key])
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  iz profiles set <key> <value>")
}

// profileSetCmd represents the profile set command
var profileSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Update active profile setting",
	Long:  buildProfileSetLongHelp(),
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			// No arguments provided - show valid keys
			printValidProfileKeys(cmd.OutOrStdout())
			return fmt.Errorf("missing required arguments")
		}
		if len(args) == 1 {
			return fmt.Errorf("missing value for key '%s'\nUsage: iz profiles set <key> <value>", args[0])
		}
		if len(args) > 2 {
			return fmt.Errorf("too many arguments\nUsage: iz profiles set <key> <value>")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		// Validate key
		if _, valid := profileSettableKeys[key]; !valid {
			// Build valid keys list for error message
			keys := make([]string, 0, len(profileSettableKeys))
			for k := range profileSettableKeys {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return fmt.Errorf("invalid key '%s'.\n\nValid keys:\n  %s", key, strings.Join(keys, "\n  "))
		}

		// Get active profile name
		profileName, err := izanami.GetActiveProfileName()
		if err != nil {
			return err
		}
		if profileName == "" {
			return fmt.Errorf("no active profile. Use 'iz profiles use <name>' to select a profile first")
		}

		// Get existing profile
		profile, err := izanami.GetProfile(profileName)
		if err != nil {
			return err
		}

		// Track if we're setting a sensitive value
		isSensitive := false

		// Update the specified field
		switch key {
		case "session":
			profile.Session = value
			profile.BaseURL = "" // Clear URL when setting session
		case "base-url":
			profile.BaseURL = value
			profile.Session = "" // Clear session when setting URL
		case "client-base-url":
			profile.ClientBaseURL = value
		case "tenant":
			profile.Tenant = value
		case "project":
			profile.Project = value
		case "context":
			profile.Context = value
		case "personal-access-token":
			profile.PersonalAccessToken = value
			isSensitive = true
		case "personal-access-token-username":
			profile.PersonalAccessTokenUsername = value
		case "client-id":
			profile.ClientID = value
		case "client-secret":
			profile.ClientSecret = value
			isSensitive = true
		}

		// Save updated profile
		if err := izanami.AddProfile(profileName, profile); err != nil {
			return fmt.Errorf("failed to update profile: %w", err)
		}

		// Display value (redact sensitive values)
		displayValue := value
		if isSensitive {
			displayValue = "<redacted>"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✓ Updated %s.%s = %s\n", profileName, key, displayValue)

		// Security warning for sensitive values
		if isSensitive {
			fmt.Fprintln(cmd.OutOrStdout(), "\n⚠️  SECURITY WARNING:")
			fmt.Fprintln(cmd.OutOrStdout(), "   Credentials are stored in plaintext in the config file.")
			fmt.Fprintln(cmd.OutOrStdout(), "   File permissions are set to 0600 (owner read/write only).")
			fmt.Fprintln(cmd.OutOrStdout(), "   Never commit config.yaml to version control.")
		}

		return nil
	},
}

// profileUnsetCmd represents the profile unset command
var profileUnsetCmd = &cobra.Command{
	Use:   "unset <key>",
	Short: "Remove a value from active profile",
	Long: `Remove a configuration value from the active profile.

This clears the specified key in the active profile.

Valid keys:
  base-url                       Izanami server URL
  tenant                         Default tenant name
  project                        Default project name
  context                        Default context path
  session                        Session name reference
  personal-access-token          Personal access token
  personal-access-token-username Username for PAT authentication
  client-id                      Client ID for API authentication
  client-secret                  Client secret for API authentication

Examples:
  iz profiles unset project
  iz profiles unset personal-access-token`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]

		// Validate key
		if _, valid := profileSettableKeys[key]; !valid {
			// Build valid keys list for error message
			keys := make([]string, 0, len(profileSettableKeys))
			for k := range profileSettableKeys {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return fmt.Errorf("invalid key '%s'.\n\nValid keys:\n  %s", key, strings.Join(keys, "\n  "))
		}

		// Get active profile name
		profileName, err := izanami.GetActiveProfileName()
		if err != nil {
			return err
		}
		if profileName == "" {
			return fmt.Errorf("no active profile. Use 'iz profiles use <name>' to select a profile first")
		}

		// Get existing profile
		profile, err := izanami.GetProfile(profileName)
		if err != nil {
			return err
		}

		// Clear the specified field
		switch key {
		case "session":
			profile.Session = ""
		case "base-url":
			profile.BaseURL = ""
		case "client-base-url":
			profile.ClientBaseURL = ""
		case "tenant":
			profile.Tenant = ""
		case "project":
			profile.Project = ""
		case "context":
			profile.Context = ""
		case "personal-access-token":
			profile.PersonalAccessToken = ""
		case "personal-access-token-username":
			profile.PersonalAccessTokenUsername = ""
		case "client-id":
			profile.ClientID = ""
		case "client-secret":
			profile.ClientSecret = ""
		}

		// Save updated profile
		if err := izanami.AddProfile(profileName, profile); err != nil {
			return fmt.Errorf("failed to update profile: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "✓ Removed %s from profile '%s'\n", key, profileName)
		return nil
	},
}

// profileDeleteCmd represents the profile delete command
var profileDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a profile",
	Long: `Delete a profile from the configuration.

If you delete the active profile, no profile will be active.

Example:
  iz profiles delete sandbox`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := args[0]

		// Get active profile to warn if deleting it
		activeProfile, err := izanami.GetActiveProfileName()
		if err == nil && activeProfile == profileName {
			fmt.Fprintf(cmd.OutOrStdout(), "⚠️  Warning: '%s' is currently the active profile\n", profileName)
		}

		// Confirm deletion unless --force is used
		if !profileDeleteForce {
			if !confirmDeletion(cmd, "profile", profileName) {
				return nil
			}
		}

		if err := izanami.DeleteProfile(profileName); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "✓ Profile '%s' deleted\n", profileName)

		if activeProfile == profileName {
			fmt.Fprintln(cmd.OutOrStdout(), "\nNo active profile set. Switch to another profile with:")
			fmt.Fprintln(cmd.OutOrStdout(), "  iz profiles use <name>")
		}

		return nil
	},
}

// profileClientKeysCmd represents the profiles client-keys command
var profileClientKeysCmd = &cobra.Command{
	Use:   "client-keys",
	Short: "Manage client API keys in active profile",
	Long: `Manage client API keys (client-id/client-secret) for feature evaluation.

Client keys are stored in the active profile and are environment-specific.
Each profile (local, sandbox, build, prod) can have different client keys
for different tenants and projects.

Keys can be stored:
  - At the tenant level (for all projects in that tenant)
  - At the project level (for specific projects only)`,
}

// profileClientKeysAddCmd represents the profiles client-keys add command
var profileClientKeysAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add client credentials to active profile",
	Long: `Add client API credentials (client-id and client-secret) to the active profile.

Credentials are stored in the active profile and are environment-specific.
Different profiles (local, sandbox, build, prod) can have different credentials.

Credentials can be stored:
  - At the tenant level (for all projects in that tenant)
  - At the project level (for specific projects only)

The 'iz features check' command will automatically use these credentials with
the following precedence:
  1. --client-id/--client-secret flags (highest priority)
  2. IZ_CLIENT_ID/IZ_CLIENT_SECRET environment variables
  3. Stored credentials from active profile (this command)

Note: If you need a separate base URL for client operations (features/events),
set it at the profile level using one of:
  - Flag: --client-base-url (on features check / events watch commands)
  - Environment variable: IZ_CLIENT_BASE_URL
  - Profile setting: iz profiles set client-base-url <url>

Examples:
  # First, switch to the profile you want to configure
  iz profiles use sandbox

  # Add tenant-wide credentials to the active profile
  iz profiles client-keys add --tenant my-tenant

  # Add project-specific credentials to the active profile
  iz profiles client-keys add --tenant my-tenant --projects proj1,proj2

Security:
  Credentials are stored in plaintext in ~/.config/iz/config.yaml
  File permissions are automatically set to 0600 (owner read/write only)
  Never commit config.yaml to version control`,
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, _ := cmd.Flags().GetString("tenant")
		projects, _ := cmd.Flags().GetStringSlice("projects")

		// Validate tenant is provided
		if tenant == "" {
			return fmt.Errorf("--tenant is required")
		}

		// Get active profile name
		profileName, err := izanami.GetActiveProfileName()
		if err != nil {
			return err
		}
		if profileName == "" {
			return fmt.Errorf("no active profile. Use 'iz profiles use <name>' to select a profile first")
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Adding credentials to profile: %s\n\n", profileName)

		reader := bufio.NewReader(cmd.InOrStdin())

		// Prompt for client-id
		fmt.Fprintf(cmd.OutOrStderr(), "Client ID: ")
		var clientID string
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			return fmt.Errorf("failed to read client ID: %w", err)
		}
		clientID = strings.TrimSpace(line)
		if clientID == "" {
			return fmt.Errorf("client ID cannot be empty")
		}

		// Prompt for client-secret (hidden)
		fmt.Fprintf(cmd.OutOrStderr(), "Client Secret: ")
		secretBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(cmd.OutOrStderr()) // New line after password input
		if err != nil {
			return fmt.Errorf("failed to read client secret: %w", err)
		}
		clientSecret := strings.TrimSpace(string(secretBytes))
		if clientSecret == "" {
			return fmt.Errorf("client secret cannot be empty")
		}

		// Check if credentials already exist in the active profile and prompt for confirmation
		profile, err := izanami.GetProfile(profileName)
		if err == nil && profile.ClientKeys != nil {
			if tenantConfig, exists := profile.ClientKeys[tenant]; exists {
				if len(projects) == 0 {
					// Check tenant-level credentials
					if tenantConfig.ClientID != "" {
						fmt.Fprintf(cmd.OutOrStderr(), "\n⚠️  Profile '%s' already has credentials for tenant '%s'.\n", profileName, tenant)
						fmt.Fprintf(cmd.OutOrStderr(), "Overwrite existing credentials? (y/N): ")
						line, _ = reader.ReadString('\n')
						if strings.ToLower(strings.TrimSpace(line)) != "y" {
							fmt.Fprintln(cmd.OutOrStderr(), "Aborted.")
							return nil
						}
					}
				} else {
					// Check project-level credentials
					if tenantConfig.Projects != nil {
						for _, project := range projects {
							if projConfig, projExists := tenantConfig.Projects[project]; projExists && projConfig.ClientID != "" {
								fmt.Fprintf(cmd.OutOrStderr(), "\n⚠️  Profile '%s' already has credentials for '%s/%s'.\n", profileName, tenant, project)
								fmt.Fprintf(cmd.OutOrStderr(), "Overwrite existing credentials? (y/N): ")
								line, _ = reader.ReadString('\n')
								if strings.ToLower(strings.TrimSpace(line)) != "y" {
									fmt.Fprintln(cmd.OutOrStderr(), "Aborted.")
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
			fmt.Fprintf(cmd.OutOrStderr(), "\n✓ Client credentials saved to profile '%s' for tenant '%s'\n", profileName, tenant)
		} else {
			fmt.Fprintf(cmd.OutOrStderr(), "\n✓ Client credentials saved to profile '%s' for tenant '%s', projects: %s\n", profileName, tenant, strings.Join(projects, ", "))
		}

		fmt.Fprintln(cmd.OutOrStderr(), "\n⚠️  SECURITY WARNING:")
		fmt.Fprintln(cmd.OutOrStderr(), "   Credentials are stored in plaintext in the config file.")
		fmt.Fprintln(cmd.OutOrStderr(), "   File permissions are set to 0600 (owner read/write only).")
		fmt.Fprintln(cmd.OutOrStderr(), "   Never commit config.yaml to version control.")

		fmt.Fprintf(cmd.OutOrStderr(), "\nYou can now use these credentials with:\n")
		fmt.Fprintf(cmd.OutOrStderr(), "  iz features check --tenant %s <feature-id>\n", tenant)

		return nil
	},
}

// profileClientKeysListCmd represents the profiles client-keys list command
var profileClientKeysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List client credentials in active profile",
	Long: `List all client API credentials stored in the active profile.

Shows credentials organized by tenant, with project-specific overrides indented.
Credentials are redacted by default; use --show-secrets to display them.

Example:
  iz profiles client-keys list
  iz profiles client-keys list --show-secrets`,
	RunE: func(cmd *cobra.Command, args []string) error {
		showSecrets, _ := cmd.Flags().GetBool("show-secrets")

		// Get active profile
		profileName, err := izanami.GetActiveProfileName()
		if err != nil || profileName == "" {
			return fmt.Errorf("no active profile. Use 'iz profiles use <name>' first")
		}

		profile, err := izanami.GetProfile(profileName)
		if err != nil {
			return err
		}

		if len(profile.ClientKeys) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No client keys configured in profile '"+profileName+"'")
			fmt.Fprintln(cmd.OutOrStdout(), "\nTo add client keys: iz profiles client-keys add --tenant <tenant>")
			return nil
		}

		// Build table
		table := tablewriter.NewWriter(cmd.OutOrStdout())
		table.SetHeader([]string{"TENANT", "SCOPE", "CLIENT-ID", "CLIENT-SECRET"})
		table.SetBorder(false)
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetCenterSeparator("")
		table.SetColumnSeparator("")
		table.SetRowSeparator("")
		table.SetHeaderLine(false)
		table.SetTablePadding("\t")
		table.SetNoWhiteSpace(true)

		// Sort tenant names for consistent output
		tenants := make([]string, 0, len(profile.ClientKeys))
		for t := range profile.ClientKeys {
			tenants = append(tenants, t)
		}
		sort.Strings(tenants)

		for _, tenant := range tenants {
			cfg := profile.ClientKeys[tenant]
			secret := cfg.ClientSecret
			if !showSecrets && secret != "" {
				secret = izanami.RedactedValue
			}

			// Tenant-level row (if tenant has credentials at tenant level)
			if cfg.ClientID != "" {
				table.Append([]string{tenant, "(tenant)", cfg.ClientID, secret})
			}

			// Project-level rows
			if cfg.Projects != nil {
				projects := make([]string, 0, len(cfg.Projects))
				for p := range cfg.Projects {
					projects = append(projects, p)
				}
				sort.Strings(projects)

				for _, proj := range projects {
					pcfg := cfg.Projects[proj]
					psecret := pcfg.ClientSecret
					if !showSecrets && psecret != "" {
						psecret = izanami.RedactedValue
					}
					table.Append([]string{tenant, proj, pcfg.ClientID, psecret})
				}
			}
		}

		table.Render()
		return nil
	},
}

// profileClientKeysDeleteCmd represents the profiles client-keys delete command
var profileClientKeysDeleteCmd = &cobra.Command{
	Use:   "delete <client-id>",
	Short: "Delete client credentials from active profile",
	Long: `Delete client credentials from the active profile.

Requires --tenant to specify the tenant. Use --project to delete
project-level credentials, otherwise deletes tenant-level credentials.

Examples:
  # Delete tenant-level credentials
  iz profiles client-keys delete --tenant my-tenant my-client-id

  # Delete project-level credentials
  iz profiles client-keys delete --tenant my-tenant --project proj1 proj1-client-id`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		clientID := args[0]
		tenant, _ := cmd.Flags().GetString("tenant")
		project, _ := cmd.Flags().GetString("project")

		// Get active profile
		profileName, err := izanami.GetActiveProfileName()
		if err != nil || profileName == "" {
			return fmt.Errorf("no active profile. Use 'iz profiles use <name>' first")
		}

		profile, err := izanami.GetProfile(profileName)
		if err != nil {
			return err
		}

		// Check tenant exists
		cfg, exists := profile.ClientKeys[tenant]
		if !exists {
			return fmt.Errorf("tenant '%s' not found in profile '%s'", tenant, profileName)
		}

		if project == "" {
			// Delete tenant-level credentials
			if cfg.ClientID != clientID {
				return fmt.Errorf("client-id '%s' not found at tenant level for '%s'", clientID, tenant)
			}
			cfg.ClientID = ""
			cfg.ClientSecret = ""
			profile.ClientKeys[tenant] = cfg
		} else {
			// Delete project-level credentials
			if cfg.Projects == nil {
				return fmt.Errorf("project '%s' not found in tenant '%s'", project, tenant)
			}
			pcfg, exists := cfg.Projects[project]
			if !exists {
				return fmt.Errorf("project '%s' not found in tenant '%s'", project, tenant)
			}
			if pcfg.ClientID != clientID {
				return fmt.Errorf("client-id '%s' not found for project '%s/%s'", clientID, tenant, project)
			}
			delete(cfg.Projects, project)
			profile.ClientKeys[tenant] = cfg
		}

		// Clean up: remove tenant entry if empty
		cfg = profile.ClientKeys[tenant]
		if cfg.ClientID == "" && len(cfg.Projects) == 0 {
			delete(profile.ClientKeys, tenant)
		}

		// Save profile
		if err := izanami.AddProfile(profileName, profile); err != nil {
			return fmt.Errorf("failed to save profile: %w", err)
		}

		if project == "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted credentials for tenant '%s' (client-id: %s)\n", tenant, clientID)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted credentials for project '%s/%s' (client-id: %s)\n", tenant, project, clientID)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(profileCmd)

	// Add subcommands
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileCurrentCmd)
	profileCmd.AddCommand(profileShowCmd)
	profileCmd.AddCommand(profileUseCmd)
	profileCmd.AddCommand(profileAddCmd)
	profileCmd.AddCommand(profileSetCmd)
	profileCmd.AddCommand(profileUnsetCmd)
	profileCmd.AddCommand(profileDeleteCmd)
	profileCmd.AddCommand(profileClientKeysCmd)

	// Add client-keys subcommands
	profileClientKeysCmd.AddCommand(profileClientKeysAddCmd)
	profileClientKeysCmd.AddCommand(profileClientKeysListCmd)
	profileClientKeysCmd.AddCommand(profileClientKeysDeleteCmd)

	// Dynamic completion for profile keys (same keys for set/unset)
	profileUseCmd.ValidArgsFunction = completeProfileNames
	profileShowCmd.ValidArgsFunction = completeProfileNames
	profileDeleteCmd.ValidArgsFunction = completeProfileNames
	profileSetCmd.ValidArgsFunction = completeProfileKeys
	profileUnsetCmd.ValidArgsFunction = completeProfileKeys

	// Flags for profile delete
	profileDeleteCmd.Flags().BoolVarP(&profileDeleteForce, "force", "f", false, "Skip confirmation prompt")

	// Flags for profile add
	profileAddCmd.Flags().String("url", "", "Server URL")
	profileAddCmd.Flags().String("tenant", "", "Default tenant")
	profileAddCmd.Flags().String("project", "", "Default project")
	profileAddCmd.Flags().String("context", "", "Default context")
	profileAddCmd.Flags().String("client-id", "", "Client ID for feature checks")
	profileAddCmd.Flags().String("client-secret", "", "Client secret for feature checks")
	profileAddCmd.Flags().String("client-base-url", "", "Base URL for client operations (features/events)")
	profileAddCmd.Flags().BoolP("interactive", "i", false, "Force interactive mode")

	// Flags for profile show
	profileShowCmd.Flags().Bool("show-secrets", false, "Show sensitive values (client secrets)")

	// Flags for client-keys add
	profileClientKeysAddCmd.Flags().String("tenant", "", "Tenant name (required)")
	profileClientKeysAddCmd.Flags().StringSlice("projects", []string{}, "Project names (comma-separated)")
	profileClientKeysAddCmd.MarkFlagRequired("tenant")

	// Flags for client-keys list
	profileClientKeysListCmd.Flags().Bool("show-secrets", false, "Show sensitive values (client secrets)")

	// Flags for client-keys delete
	profileClientKeysDeleteCmd.Flags().String("tenant", "", "Tenant name (required)")
	profileClientKeysDeleteCmd.Flags().String("project", "", "Project name (optional, for project-level credentials)")
	profileClientKeysDeleteCmd.MarkFlagRequired("tenant")
}

// printProfile prints profile details in a formatted way
func printProfile(w io.Writer, profile *izanami.Profile, showSecrets bool) {
	if profile.Session != "" {
		fmt.Fprintf(w, "  Session:        %s\n", profile.Session)
	}

	// Resolve URL: try profile.BaseURL first, then session.URL
	url := profile.BaseURL
	if url == "" && profile.Session != "" {
		// Profile references a session - get URL from session
		sessions, err := izanami.LoadSessions()
		if err == nil {
			sessionData, err := sessions.GetSession(profile.Session)
			if err == nil && sessionData.URL != "" {
				url = sessionData.URL
			}
		}
	}
	if url != "" {
		fmt.Fprintf(w, "  URL:            %s\n", url)
	}
	if profile.ClientBaseURL != "" {
		fmt.Fprintf(w, "  Client URL:     %s\n", profile.ClientBaseURL)
	}
	if profile.Tenant != "" {
		fmt.Fprintf(w, "  Tenant:         %s\n", profile.Tenant)
	}
	if profile.Project != "" {
		fmt.Fprintf(w, "  Project:        %s\n", profile.Project)
	}
	if profile.Context != "" {
		fmt.Fprintf(w, "  Context:        %s\n", profile.Context)
	}
	if profile.ClientID != "" {
		fmt.Fprintf(w, "  Client ID:      %s\n", profile.ClientID)
	}
	if profile.ClientSecret != "" {
		if showSecrets {
			fmt.Fprintf(w, "  Client Secret:  %s\n", profile.ClientSecret)
		} else {
			fmt.Fprintf(w, "  Client Secret:  <redacted>\n")
		}
	}
}
