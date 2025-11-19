package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"golang.org/x/term"
)

var (
	loginSessionName string
	loginPassword    string
)

// loginCmd represents the login command
var loginCmd = &cobra.Command{
	Use:   "login <url> <username>",
	Short: "Login to Izanami and save session",
	Long: `Login to an Izanami instance and save the authentication session.

The command will prompt for your password securely, authenticate with
Izanami, and save the JWT token for future use.

Examples:
  # Login to local Izanami
  iz login http://localhost:9000 RESERVED_ADMIN_USER

  # Login with a custom session name
  iz login https://izanami.prod.com admin --name prod

  # Switch to that session later
  iz sessions use prod`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		baseURL := args[0]
		username := args[1]

		// Generate session name if not provided
		sessionName := loginSessionName
		if sessionName == "" {
			// Extract hostname from URL for session name
			sessionName = extractSessionName(baseURL, username)
		}

		// Get password
		password := loginPassword
		if password == "" {
			fmt.Fprintf(os.Stderr, "Password: ")
			passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Fprintln(os.Stderr) // New line after password input
			if err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}
			password = string(passwordBytes)
		}

		if password == "" {
			return fmt.Errorf("password cannot be empty")
		}

		// Login to Izanami
		fmt.Fprintf(os.Stderr, "Authenticating with %s...\n", baseURL)

		token, err := performLogin(baseURL, username, password)
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		// Save session
		sessions, err := izanami.LoadSessions()
		if err != nil {
			return fmt.Errorf("failed to load sessions: %w", err)
		}

		session := &izanami.Session{
			URL:       baseURL,
			Username:  username,
			JwtToken:  token,
			CreatedAt: time.Now(),
		}

		sessions.AddSession(sessionName, session)
		sessions.SetActiveSession(sessionName)

		if err := sessions.Save(); err != nil {
			return fmt.Errorf("failed to save session: %w", err)
		}

		// Auto-create or update profile
		profileName, profileCreated, profileUpdated := handleProfileCreation(baseURL, username, sessionName)

		// Success messages
		fmt.Fprintf(os.Stderr, "✅ Successfully logged in as %s\n", username)
		fmt.Fprintf(os.Stderr, "   Session saved as: %s\n", sessionName)
		fmt.Fprintf(os.Stderr, "   Active session: %s\n", sessions.Active)

		// Profile messages
		if profileCreated {
			fmt.Fprintf(os.Stderr, "\n✓ Profile '%s' created\n", profileName)
		} else if profileUpdated {
			fmt.Fprintf(os.Stderr, "\n   Using existing profile: %s (session updated)\n", profileName)
		}

		if profileName != "" {
			fmt.Fprintf(os.Stderr, "   Active profile: %s\n", profileName)
		}

		fmt.Fprintf(os.Stderr, "\nYou can now run commands like:\n")
		fmt.Fprintf(os.Stderr, "  iz admin tenants list\n")
		fmt.Fprintf(os.Stderr, "  iz features list --tenant <tenant>\n")

		return nil
	},
}

// performLogin performs the actual login to Izanami
func performLogin(baseURL, username, password string) (string, error) {
	config := &izanami.Config{
		BaseURL: baseURL,
		Timeout: 30,
	}

	client, err := izanami.NewClientNoAuth(config)
	if err != nil {
		return "", err
	}

	ctx := context.Background()
	token, err := client.Login(ctx, username, password)
	if err != nil {
		return "", err
	}

	return token, nil
}

// extractSessionName generates a session name from URL and username
func extractSessionName(url, username string) string {
	// Remove protocol
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")

	// Remove port
	if idx := strings.Index(url, ":"); idx != -1 {
		url = url[:idx]
	}

	// Remove path
	if idx := strings.Index(url, "/"); idx != -1 {
		url = url[:idx]
	}

	// For localhost, use "local"
	if strings.HasPrefix(url, "localhost") || strings.HasPrefix(url, "127.0.0.1") {
		return "default"
	}

	// Use hostname as session name
	return strings.ReplaceAll(url, ".", "-")
}

// promptForProfileName prompts the user to enter a profile name
// Shows suggestions and uses suggestedName as default
func promptForProfileName(suggestedName string) string {
	fmt.Fprintf(os.Stderr, "\nProfile name suggestions: local, sandbox, build, prod\n")
	fmt.Fprintf(os.Stderr, "Enter profile name [%s]: ", suggestedName)

	var input string
	fmt.Scanln(&input)
	input = strings.TrimSpace(input)

	if input == "" {
		return suggestedName
	}

	return input
}

// handleProfileCreation creates or updates a profile after successful login
// Returns (profileName, wasCreated, wasUpdated)
func handleProfileCreation(baseURL, username, sessionName string) (string, bool, bool) {
	// Check if any profiles exist
	profiles, _, err := izanami.ListProfiles()
	if err != nil {
		// If we can't load profiles, log warning but don't fail
		fmt.Fprintf(os.Stderr, "\n   Warning: could not load profiles: %v\n", err)
		return "", false, false
	}

	hasProfiles := len(profiles) > 0
	var profileName string
	var wasCreated, wasUpdated bool

	if !hasProfiles {
		// First time - no profiles exist yet
		fmt.Fprintf(os.Stderr, "\nNo profiles exist yet. Let's create one!\n")
		profileName = promptForProfileName(sessionName)
		wasCreated = true
	} else {
		// Check if URL matches existing profile
		existingProfileName, existingProfile, err := izanami.FindProfileByBaseURL(baseURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n   Warning: could not check for existing profiles: %v\n", err)
			return "", false, false
		}

		if existingProfileName != "" {
			// Found existing profile with same URL - update it
			profileName = existingProfileName
			existingProfile.Session = sessionName
			existingProfile.Username = username

			if err := izanami.AddProfile(profileName, existingProfile); err != nil {
				fmt.Fprintf(os.Stderr, "\n   Warning: failed to update profile: %v\n", err)
				return profileName, false, false
			}

			wasUpdated = true
		} else {
			// New URL - prompt for profile name
			profileName = promptForProfileName(sessionName)
			wasCreated = true
		}
	}

	// Create or update profile (if we need to create)
	if wasCreated {
		profile := &izanami.Profile{
			Session:  sessionName,
			BaseURL:  baseURL,
			Username: username,
		}

		if err := izanami.AddProfile(profileName, profile); err != nil {
			fmt.Fprintf(os.Stderr, "\n   Warning: failed to save profile: %v\n", err)
			return profileName, false, false
		}
	}

	// Set as active profile
	if profileName != "" {
		if err := izanami.SetActiveProfile(profileName); err != nil {
			fmt.Fprintf(os.Stderr, "\n   Warning: failed to set active profile: %v\n", err)
			// Don't return error - profile was still created/updated
		}
	}

	return profileName, wasCreated, wasUpdated
}

func init() {
	rootCmd.AddCommand(loginCmd)

	loginCmd.Flags().StringVar(&loginSessionName, "name", "", "Custom name for this session")
	loginCmd.Flags().StringVar(&loginPassword, "password", "", "Password (not recommended, use prompt instead)")
}
