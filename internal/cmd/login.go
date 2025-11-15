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

		fmt.Fprintf(os.Stderr, "✅ Successfully logged in as %s\n", username)
		fmt.Fprintf(os.Stderr, "   Session saved as: %s\n", sessionName)
		fmt.Fprintf(os.Stderr, "   Active session: %s\n", sessions.Active)
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

func init() {
	rootCmd.AddCommand(loginCmd)

	loginCmd.Flags().StringVar(&loginSessionName, "name", "", "Custom name for this session")
	loginCmd.Flags().StringVar(&loginPassword, "password", "", "Password (not recommended, use prompt instead)")
}
