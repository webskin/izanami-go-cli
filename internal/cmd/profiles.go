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
			fmt.Fprintln(cmd.OutOrStdout(), "  iz profiles init sandbox|build|prod")
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
  iz profiles add sandbox --session sandbox-session --tenant dev-tenant

  # Direct URL (no session)
  iz profiles add sandbox --url http://localhost:9000 --tenant dev-tenant`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := args[0]

		// Get flag values
		session, _ := cmd.Flags().GetString("session")
		url, _ := cmd.Flags().GetString("url")
		tenant, _ := cmd.Flags().GetString("tenant")
		project, _ := cmd.Flags().GetString("project")
		context, _ := cmd.Flags().GetString("context")
		clientID, _ := cmd.Flags().GetString("client-id")
		clientSecret, _ := cmd.Flags().GetString("client-secret")
		interactive, _ := cmd.Flags().GetBool("interactive")

		profile := &izanami.Profile{}

		// Interactive prompts if --interactive or no flags provided
		if interactive || (session == "" && url == "" && tenant == "" && project == "" && context == "" && clientID == "" && clientSecret == "") {
			fmt.Fprintf(cmd.OutOrStdout(), "Creating profile '%s'\n\n", profileName)
			reader := bufio.NewReader(cmd.InOrStdin())

			// Session or URL
			fmt.Fprint(cmd.OutOrStdout(), "Session name (or leave empty to specify URL): ")
			session, _ = reader.ReadString('\n')
			session = strings.TrimSpace(session)

			if session == "" {
				fmt.Fprint(cmd.OutOrStdout(), "Server URL: ")
				url, _ = reader.ReadString('\n')
				url = strings.TrimSpace(url)
			}

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
			}
		}

		// Validate: must have either session or URL
		if session == "" && url == "" {
			return fmt.Errorf("either --session or --url must be specified")
		}

		// Build profile
		profile.Session = session
		profile.BaseURL = url
		profile.Tenant = tenant
		profile.Project = project
		profile.Context = context
		profile.ClientID = clientID
		profile.ClientSecret = clientSecret

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

// profileInitCmd represents the profile init command
var profileInitCmd = &cobra.Command{
	Use:   "init <sandbox|build|prod> [name]",
	Short: "Create profile from template",
	Long: `Create a new profile using a predefined template.

Available templates:
  sandbox - Development/testing environment (localhost:9000, dev tenant)
  build   - Build/staging environment (staging server, build tenant)
  prod    - Production environment (production server, prod tenant)

You can optionally specify a custom name for the profile.
If no name is provided, the template name is used.

Examples:
  iz profiles init sandbox
  iz profiles init prod my-production
  iz profiles init build staging-env`,
	Args:      cobra.RangeArgs(1, 2),
	ValidArgs: []string{"sandbox", "build", "prod"},
	RunE: func(cmd *cobra.Command, args []string) error {
		template := args[0]
		profileName := template
		if len(args) > 1 {
			profileName = args[1]
		}

		// Validate template
		if template != "sandbox" && template != "build" && template != "prod" {
			return fmt.Errorf("invalid template '%s'. Valid templates: sandbox, build, prod", template)
		}

		profile := &izanami.Profile{}

		// Apply template defaults
		switch template {
		case "sandbox":
			profile.BaseURL = "http://localhost:9000"
			profile.Tenant = "sandbox-tenant"
			profile.Project = "test"
			profile.Context = "dev"

		case "build":
			// User needs to provide URL
			fmt.Fprint(cmd.OutOrStdout(), "Build server URL: ")
			var url string
			fmt.Scanln(&url)
			profile.BaseURL = strings.TrimSpace(url)
			profile.Tenant = "build-tenant"
			profile.Project = "integration"
			profile.Context = "staging"

		case "prod":
			// User needs to provide URL
			fmt.Fprint(cmd.OutOrStdout(), "Production server URL: ")
			var url string
			fmt.Scanln(&url)
			profile.BaseURL = strings.TrimSpace(url)
			profile.Tenant = "production"
			profile.Project = "main"
			profile.Context = "prod"
		}

		// Allow override with session
		fmt.Fprintf(cmd.OutOrStdout(), "\nUse existing session instead of URL? (leave empty to use URL): ")
		var session string
		fmt.Scanln(&session)
		if strings.TrimSpace(session) != "" {
			profile.Session = strings.TrimSpace(session)
			profile.BaseURL = "" // Clear URL if using session
		}

		// Save profile
		if err := izanami.AddProfile(profileName, profile); err != nil {
			return fmt.Errorf("failed to create profile: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "\n✓ Profile '%s' created from template '%s'\n", profileName, template)

		// Check if this is the first profile
		profiles, _, err := izanami.ListProfiles()
		if err == nil && len(profiles) == 1 {
			fmt.Fprintln(cmd.OutOrStdout(), "✓ Set as active profile (first profile created)")
		}

		fmt.Fprintln(cmd.OutOrStdout(), "\nProfile details:")
		printProfile(cmd.OutOrStdout(), profile, false)

		fmt.Fprintf(cmd.OutOrStdout(), "\nSwitch to this profile with:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  iz profiles use %s\n", profileName)

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

		// Prompt for client-id
		fmt.Fprintf(cmd.OutOrStderr(), "Client ID: ")
		var clientID string
		if _, err := fmt.Scanln(&clientID); err != nil {
			return fmt.Errorf("failed to read client ID: %w", err)
		}
		clientID = strings.TrimSpace(clientID)
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
						var response string
						fmt.Scanln(&response)
						if strings.ToLower(strings.TrimSpace(response)) != "y" {
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
								var response string
								fmt.Scanln(&response)
								if strings.ToLower(strings.TrimSpace(response)) != "y" {
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

func init() {
	rootCmd.AddCommand(profileCmd)

	// Add subcommands
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileCurrentCmd)
	profileCmd.AddCommand(profileShowCmd)
	profileCmd.AddCommand(profileUseCmd)
	profileCmd.AddCommand(profileAddCmd)
	profileCmd.AddCommand(profileInitCmd)
	profileCmd.AddCommand(profileSetCmd)
	profileCmd.AddCommand(profileUnsetCmd)
	profileCmd.AddCommand(profileDeleteCmd)
	profileCmd.AddCommand(profileClientKeysCmd)

	// Add client-keys subcommands
	profileClientKeysCmd.AddCommand(profileClientKeysAddCmd)

	// Dynamic completion for profile keys (same keys for set/unset)
	profileSetCmd.ValidArgsFunction = completeProfileKeys
	profileUnsetCmd.ValidArgsFunction = completeProfileKeys

	// Flags for profile delete
	profileDeleteCmd.Flags().BoolVarP(&profileDeleteForce, "force", "f", false, "Skip confirmation prompt")

	// Flags for profile add
	profileAddCmd.Flags().String("session", "", "Reference to existing session")
	profileAddCmd.Flags().String("url", "", "Server URL (alternative to --session)")
	profileAddCmd.Flags().String("tenant", "", "Default tenant")
	profileAddCmd.Flags().String("project", "", "Default project")
	profileAddCmd.Flags().String("context", "", "Default context")
	profileAddCmd.Flags().String("client-id", "", "Client ID for feature checks")
	profileAddCmd.Flags().String("client-secret", "", "Client secret for feature checks")
	profileAddCmd.Flags().BoolP("interactive", "i", false, "Force interactive mode")

	// Flags for profile show
	profileShowCmd.Flags().Bool("show-secrets", false, "Show sensitive values (client secrets)")

	// Flags for client-keys add
	profileClientKeysAddCmd.Flags().String("tenant", "", "Tenant name (required)")
	profileClientKeysAddCmd.Flags().StringSlice("projects", []string{}, "Project names (comma-separated)")
	profileClientKeysAddCmd.MarkFlagRequired("tenant")
}

// printProfile prints profile details in a formatted way
func printProfile(w io.Writer, profile *izanami.Profile, showSecrets bool) {
	if profile.Session != "" {
		fmt.Fprintf(w, "  Session:       %s\n", profile.Session)
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
		fmt.Fprintf(w, "  URL:           %s\n", url)
	}
	if profile.Tenant != "" {
		fmt.Fprintf(w, "  Tenant:        %s\n", profile.Tenant)
	}
	if profile.Project != "" {
		fmt.Fprintf(w, "  Project:       %s\n", profile.Project)
	}
	if profile.Context != "" {
		fmt.Fprintf(w, "  Context:       %s\n", profile.Context)
	}
	if profile.ClientID != "" {
		fmt.Fprintf(w, "  Client ID:     %s\n", profile.ClientID)
	}
	if profile.ClientSecret != "" {
		if showSecrets {
			fmt.Fprintf(w, "  Client Secret: %s\n", profile.ClientSecret)
		} else {
			fmt.Fprintf(w, "  Client Secret: <redacted>\n")
		}
	}
}
