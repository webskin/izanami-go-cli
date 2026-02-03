package cmd

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// profileSettableKeys defines keys that can be set via 'iz profiles set'
// Maps key name to description
var profileSettableKeys = map[string]string{
	"leader-url":                     "Izanami leader URL (admin API)",
	"tenant":                         "Default tenant name",
	"project":                        "Default project name",
	"context":                        "Default context path",
	"session":                        "Session name to reference (clears leader-url)",
	"personal-access-token":          "Personal access token",
	"personal-access-token-username": "Username for PAT authentication",
	"default-worker":                 "Default worker name for feature checks",
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
		table.SetHeader([]string{"", "Name", "Session", "URL", "Tenant", "Project", "Workers"})
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

			// Resolve URL: try profile.LeaderURL first, then session.URL
			url := profile.LeaderURL
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

			workers := "-"
			if len(profile.Workers) > 0 {
				workerNames := make([]string, 0, len(profile.Workers))
				for wn := range profile.Workers {
					workerNames = append(workerNames, wn)
				}
				sort.Strings(workerNames)
				if len(workerNames) > 1 && profile.DefaultWorker != "" {
					for i, wn := range workerNames {
						if wn == profile.DefaultWorker {
							workerNames[i] = wn + " (default)"
							break
						}
					}
				}
				workers = strings.Join(workerNames, ", ")
			}

			table.Append([]string{activeMarker, name, session, url, tenant, project, workers})
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
			if profile.LeaderURL != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  URL:     %s\n", profile.LeaderURL)
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

  # Full non-interactive setup
  iz profiles add myprofile \
    --url http://localhost:9000 \
    --tenant test-tenant \
    --project test-project \
    --context PROD

  # Create and immediately activate
  iz profiles add prod --url https://prod.example.com --active

  # Then add client credentials for feature checks
  iz profiles client-keys add --tenant test-tenant
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := args[0]

		// Get flag values
		url, _ := cmd.Flags().GetString("url")
		tenant, _ := cmd.Flags().GetString("tenant")
		project, _ := cmd.Flags().GetString("project")
		context, _ := cmd.Flags().GetString("context")
		interactive, _ := cmd.Flags().GetBool("interactive")
		active, _ := cmd.Flags().GetBool("active")

		profile := &izanami.Profile{}

		// Interactive prompts if --interactive or no flags provided
		if interactive || (url == "" && tenant == "" && project == "" && context == "") {
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
		}

		// Validate: must have URL
		if url == "" {
			return fmt.Errorf("--url is required")
		}

		// Build profile
		profile.LeaderURL = url
		profile.Tenant = tenant
		profile.Project = project
		profile.Context = context

		// Save profile
		if err := izanami.AddProfile(profileName, profile); err != nil {
			return fmt.Errorf("failed to add profile: %w", err)
		}

		// Activate if requested (and not already active from auto-activate)
		if active {
			currentActive, _ := izanami.GetActiveProfileName()
			if currentActive != profileName {
				if err := izanami.SetActiveProfile(profileName); err != nil {
					return fmt.Errorf("profile created but failed to set as active: %w", err)
				}
			}
		}

		isActive := false
		if currentActive, _ := izanami.GetActiveProfileName(); currentActive == profileName {
			isActive = true
		}

		fmt.Fprintf(cmd.OutOrStdout(), "\n✓ Profile '%s' created successfully\n", profileName)

		if isActive {
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Set as active profile\n")
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "\nSwitch to this profile with:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  iz profiles use %s\n", profileName)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "\nTo add client credentials for feature checks:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  iz profiles client-keys add --tenant <tenant>\n")
		fmt.Fprintf(cmd.OutOrStdout(), "\nTo add workers for split deployments:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  iz profiles workers add <name> --url <worker-url>\n")

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
	sb.WriteString("  iz profiles set leader-url https://izanami.example.com\n")
	sb.WriteString("  iz profiles set session sandbox-session\n")
	sb.WriteString("  iz profiles set client-id my-client-id\n")
	sb.WriteString("  iz profiles set default-worker eu-west\n")
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
			profile.LeaderURL = "" // Clear URL when setting session
		case "leader-url":
			profile.LeaderURL = value
			profile.Session = "" // Clear session when setting URL
		case "default-worker":
			profile.DefaultWorker = value
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
  leader-url                     Izanami leader URL (admin API)
  tenant                         Default tenant name
  project                        Default project name
  context                        Default context path
  session                        Session name reference
  personal-access-token          Personal access token
  personal-access-token-username Username for PAT authentication
  default-worker                 Default worker name

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
		case "leader-url":
			profile.LeaderURL = ""
		case "default-worker":
			profile.DefaultWorker = ""
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
	profileAddCmd.Flags().BoolP("interactive", "i", false, "Force interactive mode")
	profileAddCmd.Flags().Bool("active", false, "Set as the active profile after creation")

	// Flags for profile show
	profileShowCmd.Flags().Bool("show-secrets", false, "Show sensitive values (client secrets)")

	// Flags for client-keys add
	profileClientKeysAddCmd.Flags().String("tenant", "", "Tenant name (required)")
	profileClientKeysAddCmd.Flags().StringSlice("projects", []string{}, "Project names (comma-separated)")
	profileClientKeysAddCmd.Flags().String("client-id", "", "Client ID (omit for interactive prompt)")
	profileClientKeysAddCmd.Flags().String("client-secret", "", "Client secret (omit for interactive prompt)")
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

	// Resolve URL: try profile.LeaderURL first, then session.URL
	url := profile.LeaderURL
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
		fmt.Fprintf(w, "  Leader URL:     %s\n", url)
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
	if len(profile.ClientKeys) > 0 {
		fmt.Fprintf(w, "  Client Keys:    %s\n", formatClientKeysCount(profile.ClientKeys))
	}
	if len(profile.Workers) > 0 {
		workerNames := make([]string, 0, len(profile.Workers))
		for wn := range profile.Workers {
			workerNames = append(workerNames, wn)
		}
		sort.Strings(workerNames)
		if len(workerNames) > 1 && profile.DefaultWorker != "" {
			for i, wn := range workerNames {
				if wn == profile.DefaultWorker {
					workerNames[i] = wn + " (default)"
					break
				}
			}
		}
		fmt.Fprintf(w, "  Workers:        %s\n", strings.Join(workerNames, ", "))
	} else if profile.DefaultWorker != "" {
		fmt.Fprintf(w, "  Default Worker: %s\n", profile.DefaultWorker)
	}
}
