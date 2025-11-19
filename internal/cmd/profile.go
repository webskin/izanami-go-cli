package cmd

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"golang.org/x/term"
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
			fmt.Println("No profiles configured")
			fmt.Println("\nCreate a profile with:")
			fmt.Println("  iz profiles add <name>")
			fmt.Println("  iz profiles init sandbox|build|prod")
			return nil
		}

		// Create table
		table := tablewriter.NewWriter(os.Stdout)
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

			url := profile.BaseURL
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
			fmt.Printf("\nActive profile: %s\n", activeProfile)
		} else {
			fmt.Println("\nNo active profile set")
			fmt.Println("Switch to a profile with: iz profiles use <name>")
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
			fmt.Println("No active profile set")
			fmt.Println("\nSet a profile with:")
			fmt.Println("  iz profiles use <name>")
			return nil
		}

		profile, err := izanami.GetProfile(activeProfileName)
		if err != nil {
			return err
		}

		fmt.Printf("Active Profile: %s\n\n", activeProfileName)
		printProfile(profile, false)

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

		fmt.Printf("Profile: %s\n\n", profileName)
		printProfile(profile, showSecrets)

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

		fmt.Printf("✓ Switched to profile '%s'\n", profileName)

		// Show brief profile info
		profile, err := izanami.GetProfile(profileName)
		if err == nil {
			fmt.Println()
			if profile.Session != "" {
				fmt.Printf("  Session: %s\n", profile.Session)
			}
			if profile.BaseURL != "" {
				fmt.Printf("  URL:     %s\n", profile.BaseURL)
			}
			if profile.Tenant != "" {
				fmt.Printf("  Tenant:  %s\n", profile.Tenant)
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
			fmt.Printf("Creating profile '%s'\n\n", profileName)

			// Session or URL
			fmt.Print("Session name (or leave empty to specify URL): ")
			fmt.Scanln(&session)
			session = strings.TrimSpace(session)

			if session == "" {
				fmt.Print("Server URL: ")
				fmt.Scanln(&url)
				url = strings.TrimSpace(url)
			}

			// Tenant
			fmt.Print("Default tenant (optional): ")
			fmt.Scanln(&tenant)
			tenant = strings.TrimSpace(tenant)

			// Project
			fmt.Print("Default project (optional): ")
			fmt.Scanln(&project)
			project = strings.TrimSpace(project)

			// Context
			fmt.Print("Default context (optional): ")
			fmt.Scanln(&context)
			context = strings.TrimSpace(context)

			// Client credentials
			fmt.Print("\nConfigure client credentials for feature checks? (y/N): ")
			var addCreds string
			fmt.Scanln(&addCreds)
			if strings.ToLower(strings.TrimSpace(addCreds)) == "y" {
				fmt.Print("Client ID: ")
				fmt.Scanln(&clientID)
				clientID = strings.TrimSpace(clientID)

				fmt.Print("Client Secret: ")
				secretBytes, err := term.ReadPassword(int(syscall.Stdin))
				fmt.Println()
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

		fmt.Printf("\n✓ Profile '%s' created successfully\n", profileName)

		// Check if this is the first/only profile
		profiles, _, err := izanami.ListProfiles()
		if err == nil && len(profiles) == 1 {
			fmt.Printf("✓ Set as active profile (first profile created)\n")
		}

		fmt.Printf("\nSwitch to this profile with:\n")
		fmt.Printf("  iz profiles use %s\n", profileName)

		if clientSecret != "" {
			fmt.Println("\n⚠️  SECURITY WARNING:")
			fmt.Println("   Credentials are stored in plaintext in the config file.")
			fmt.Println("   File permissions are set to 0600 (owner read/write only).")
			fmt.Println("   Never commit config.yaml to version control.")
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
			fmt.Print("Build server URL: ")
			var url string
			fmt.Scanln(&url)
			profile.BaseURL = strings.TrimSpace(url)
			profile.Tenant = "build-tenant"
			profile.Project = "integration"
			profile.Context = "staging"

		case "prod":
			// User needs to provide URL
			fmt.Print("Production server URL: ")
			var url string
			fmt.Scanln(&url)
			profile.BaseURL = strings.TrimSpace(url)
			profile.Tenant = "production"
			profile.Project = "main"
			profile.Context = "prod"
		}

		// Allow override with session
		fmt.Printf("\nUse existing session instead of URL? (leave empty to use URL): ")
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

		fmt.Printf("\n✓ Profile '%s' created from template '%s'\n", profileName, template)

		// Check if this is the first profile
		profiles, _, err := izanami.ListProfiles()
		if err == nil && len(profiles) == 1 {
			fmt.Printf("✓ Set as active profile (first profile created)\n")
		}

		fmt.Printf("\nProfile details:\n")
		printProfile(profile, false)

		fmt.Printf("\nSwitch to this profile with:\n")
		fmt.Printf("  iz profiles use %s\n", profileName)

		return nil
	},
}

// profileSetCmd represents the profile set command
var profileSetCmd = &cobra.Command{
	Use:   "set <name> <key> <value>",
	Short: "Update profile setting",
	Long: `Update a specific setting in a profile.

Valid keys:
  session   - Session name to reference
  base-url  - Server URL (clears session if set)
  tenant    - Default tenant
  project   - Default project
  context   - Default context

Examples:
  iz profiles set sandbox tenant new-tenant
  iz profiles set prod base-url https://izanami.example.com
  iz profiles set build session build-session`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := args[0]
		key := args[1]
		value := args[2]

		// Get existing profile
		profile, err := izanami.GetProfile(profileName)
		if err != nil {
			return err
		}

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
		default:
			return fmt.Errorf("invalid key '%s'. Valid keys: session, base-url, tenant, project, context", key)
		}

		// Save updated profile
		if err := izanami.AddProfile(profileName, profile); err != nil {
			return fmt.Errorf("failed to update profile: %w", err)
		}

		fmt.Printf("✓ Updated %s.%s = %s\n", profileName, key, value)

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
			fmt.Printf("⚠️  Warning: '%s' is currently the active profile\n", profileName)
		}

		// Confirm deletion
		fmt.Printf("Delete profile '%s'? (y/N): ", profileName)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			fmt.Println("Cancelled")
			return nil
		}

		if err := izanami.DeleteProfile(profileName); err != nil {
			return err
		}

		fmt.Printf("✓ Profile '%s' deleted\n", profileName)

		if activeProfile == profileName {
			fmt.Println("\nNo active profile set. Switch to another profile with:")
			fmt.Println("  iz profiles use <name>")
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
	profileCmd.AddCommand(profileInitCmd)
	profileCmd.AddCommand(profileSetCmd)
	profileCmd.AddCommand(profileDeleteCmd)

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
}

// printProfile prints profile details in a formatted way
func printProfile(profile *izanami.Profile, showSecrets bool) {
	if profile.Session != "" {
		fmt.Printf("  Session:       %s\n", profile.Session)
	}
	if profile.BaseURL != "" {
		fmt.Printf("  URL:           %s\n", profile.BaseURL)
	}
	if profile.Tenant != "" {
		fmt.Printf("  Tenant:        %s\n", profile.Tenant)
	}
	if profile.Project != "" {
		fmt.Printf("  Project:       %s\n", profile.Project)
	}
	if profile.Context != "" {
		fmt.Printf("  Context:       %s\n", profile.Context)
	}
	if profile.ClientID != "" {
		fmt.Printf("  Client ID:     %s\n", profile.ClientID)
	}
	if profile.ClientSecret != "" {
		if showSecrets {
			fmt.Printf("  Client Secret: %s\n", profile.ClientSecret)
		} else {
			fmt.Printf("  Client Secret: <redacted>\n")
		}
	}
}
