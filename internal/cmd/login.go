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
Izanami, and save the JWT token for future use. The session is automatically
linked to a profile, and the profile becomes active.

Examples:
  # Login to local Izanami
  iz login http://localhost:9000 RESERVED_ADMIN_USER

  # Login to production (will prompt for profile name if new URL)
  iz login https://izanami.prod.com admin`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		baseURL := args[0]
		username := args[1]

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

		// Determine profile name first (create or find existing)
		profileName, profileCreated, profileUpdated := determineProfileName(baseURL, username)

		// Generate deterministic session name: <profile-name>-<username>-session
		sessionName := fmt.Sprintf("%s-%s-session", profileName, username)

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

		// Check if session with same URL+username exists and overwrite
		for name, existingSession := range sessions.Sessions {
			if existingSession.URL == baseURL && existingSession.Username == username {
				// Delete old session if it has a different name
				if name != sessionName {
					delete(sessions.Sessions, name)
					fmt.Fprintf(os.Stderr, "   Replacing existing session: %s\n", name)
				}
				break
			}
		}

		sessions.AddSession(sessionName, session)

		if err := sessions.Save(); err != nil {
			return fmt.Errorf("failed to save session: %w", err)
		}

		// Update profile with session reference
		if err := updateProfileWithSession(profileName, baseURL, username, sessionName); err != nil {
			fmt.Fprintf(os.Stderr, "\n   Warning: failed to update profile: %v\n", err)
		}

		// Success messages
		fmt.Fprintf(os.Stderr, "✅ Successfully logged in as %s\n", username)
		fmt.Fprintf(os.Stderr, "   Session saved as: %s\n", sessionName)

		// Profile messages
		if profileCreated {
			fmt.Fprintf(os.Stderr, "\n✓ Profile '%s' created\n", profileName)
		} else if profileUpdated {
			fmt.Fprintf(os.Stderr, "\n   Using existing profile: %s (session updated)\n", profileName)
		}

		if profileName != "" {
			activeProfile, _ := izanami.GetActiveProfileName()
			if activeProfile == profileName {
				fmt.Fprintf(os.Stderr, "   Active profile: %s\n", profileName)
			}
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

// determineProfileName determines the profile name to use for login
// Returns (profileName, wasCreated, wasUpdated)
func determineProfileName(baseURL, username string) (string, bool, bool) {
	// Check if any profiles exist
	profiles, _, err := izanami.ListProfiles()
	if err != nil {
		// If we can't load profiles, use default name
		fmt.Fprintf(os.Stderr, "\n   Warning: could not load profiles: %v\n", err)
		return extractSessionName(baseURL, username), false, false
	}

	hasProfiles := len(profiles) > 0
	var profileName string
	var wasCreated, wasUpdated bool

	if !hasProfiles {
		// First time - no profiles exist yet
		fmt.Fprintf(os.Stderr, "\nNo profiles exist yet. Let's create one!\n")
		suggestedName := extractSessionName(baseURL, username)
		profileName = promptForProfileName(suggestedName)
		wasCreated = true
	} else {
		// Check if URL matches existing profile
		existingProfileName, _, err := izanami.FindProfileByBaseURL(baseURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n   Warning: could not check for existing profiles: %v\n", err)
			return extractSessionName(baseURL, username), false, false
		}

		if existingProfileName != "" {
			// Found existing profile with same URL - use it
			profileName = existingProfileName
			wasUpdated = true
		} else {
			// New URL - prompt for profile name
			suggestedName := extractSessionName(baseURL, username)
			profileName = promptForProfileName(suggestedName)
			wasCreated = true
		}
	}

	return profileName, wasCreated, wasUpdated
}

// updateProfileWithSession creates or updates a profile with session reference
func updateProfileWithSession(profileName, baseURL, username, sessionName string) error {
	// Try to load existing profile
	existingProfile, err := izanami.GetProfile(profileName)
	if err != nil {
		// Profile doesn't exist - create new one
		// Note: BaseURL is intentionally empty - it will be resolved from the session
		profile := &izanami.Profile{
			Session:  sessionName,
			Username: username,
		}

		if err := izanami.AddProfile(profileName, profile); err != nil {
			return fmt.Errorf("failed to save profile: %w", err)
		}
	} else {
		// Profile exists - update session reference
		existingProfile.Session = sessionName
		existingProfile.Username = username

		if err := izanami.AddProfile(profileName, existingProfile); err != nil {
			return fmt.Errorf("failed to update profile: %w", err)
		}
	}

	// Set as active profile
	if err := izanami.SetActiveProfile(profileName); err != nil {
		return fmt.Errorf("failed to set active profile: %w", err)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(loginCmd)

	loginCmd.Flags().StringVar(&loginSessionName, "name", "", "Custom name for this session")
	loginCmd.Flags().StringVar(&loginPassword, "password", "", "Password (not recommended, use prompt instead)")
}
