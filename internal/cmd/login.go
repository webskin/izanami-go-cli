package cmd

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"github.com/webskin/izanami-go-cli/internal/utils"
	"golang.org/x/term"
)

var (
	loginSessionName string
	loginPassword    string
	loginOIDC        bool
	loginToken       string
	loginNoBrowser   bool
)

// loginCmd represents the login command
var loginCmd = &cobra.Command{
	Use:   "login [url] [username]",
	Short: "Login to Izanami and save session",
	Long: `Login to an Izanami instance and save the authentication session.

The command will prompt for your password securely, authenticate with
Izanami, and save the JWT token for future use. The session is automatically
linked to a profile, and the profile becomes active.

OIDC Authentication:
  Use --oidc flag to authenticate via your organization's identity provider.
  This opens a browser for OIDC login, then you copy the JWT token back to the CLI.

Examples:
  # Login with username/password
  iz login http://localhost:9000 RESERVED_ADMIN_USER

  # Login via OIDC (opens browser)
  iz login --oidc --url https://izanami.prod.com

  # Login via OIDC with token directly (for scripting)
  iz login --oidc --url https://izanami.prod.com --token "eyJhbGciOiJIUzI1NiIs..."

  # Login via OIDC without opening browser
  iz login --oidc --url https://izanami.prod.com --no-browser`,
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// OIDC flow
		if loginOIDC {
			return runOIDCLogin(cmd, args)
		}

		// Traditional username/password flow requires both args
		if len(args) < 2 {
			return fmt.Errorf("username/password login requires: iz login <url> <username>\nFor OIDC login, use: iz login --oidc --url <url>")
		}

		baseURL := args[0]
		username := args[1]

		// Get password
		password := loginPassword
		if password == "" {
			fmt.Fprintf(cmd.OutOrStderr(), "Password: ")
			passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Fprintln(cmd.OutOrStderr()) // New line after password input
			if err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}
			password = string(passwordBytes)
		}

		if password == "" {
			return fmt.Errorf("password cannot be empty")
		}

		// Login to Izanami
		fmt.Fprintf(cmd.OutOrStderr(), "Authenticating with %s...\n", baseURL)

		token, err := performLogin(baseURL, username, password)
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		// Determine profile name first (create or find existing)
		profileName, profileCreated, profileUpdated := determineProfileName(cmd.InOrStdin(), cmd.OutOrStderr(), baseURL, username)

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
					fmt.Fprintf(cmd.OutOrStderr(), "   Replacing existing session: %s\n", name)
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
			fmt.Fprintf(cmd.OutOrStderr(), "\n   Warning: failed to update profile: %v\n", err)
		}

		// Success messages
		fmt.Fprintf(cmd.OutOrStderr(), "✅ Successfully logged in as %s\n", username)
		fmt.Fprintf(cmd.OutOrStderr(), "   Session saved as: %s\n", sessionName)

		// Profile messages
		if profileCreated {
			fmt.Fprintf(cmd.OutOrStderr(), "\n✓ Profile '%s' created\n", profileName)
		} else if profileUpdated {
			fmt.Fprintf(cmd.OutOrStderr(), "\n   Using existing profile: %s (session updated)\n", profileName)
		}

		if profileName != "" {
			activeProfile, _ := izanami.GetActiveProfileName()
			if activeProfile == profileName {
				fmt.Fprintf(cmd.OutOrStderr(), "   Active profile: %s\n", profileName)
			}
		}

		fmt.Fprintf(cmd.OutOrStderr(), "\nYou can now run commands like:\n")
		fmt.Fprintf(cmd.OutOrStderr(), "  iz admin tenants list\n")
		fmt.Fprintf(cmd.OutOrStderr(), "  iz features list --tenant <tenant>\n")

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
func promptForProfileName(r io.Reader, w io.Writer, suggestedName string) string {
	fmt.Fprintf(w, "\nProfile name suggestions: local, sandbox, build, prod\n")
	fmt.Fprintf(w, "Enter profile name [%s]: ", suggestedName)

	reader := bufio.NewReader(r)
	input, err := reader.ReadString('\n')
	if err != nil && err.Error() != "EOF" {
		return suggestedName
	}
	input = strings.TrimSpace(input)

	if input == "" {
		return suggestedName
	}

	return input
}

// determineProfileName determines the profile name to use for login
// Returns (profileName, wasCreated, wasUpdated)
func determineProfileName(r io.Reader, w io.Writer, baseURL, username string) (string, bool, bool) {
	// Check if any profiles exist
	profiles, _, err := izanami.ListProfiles()
	if err != nil {
		// If we can't load profiles, use default name
		fmt.Fprintf(w, "\n   Warning: could not load profiles: %v\n", err)
		return extractSessionName(baseURL, username), false, false
	}

	hasProfiles := len(profiles) > 0
	var profileName string
	var wasCreated, wasUpdated bool

	if !hasProfiles {
		// First time - no profiles exist yet
		fmt.Fprintf(w, "\nNo profiles exist yet. Let's create one!\n")
		suggestedName := extractSessionName(baseURL, username)
		profileName = promptForProfileName(r, w, suggestedName)
		wasCreated = true
	} else {
		// Check if URL matches existing profile
		existingProfileName, _, err := izanami.FindProfileByBaseURL(baseURL)
		if err != nil {
			fmt.Fprintf(w, "\n   Warning: could not check for existing profiles: %v\n", err)
			return extractSessionName(baseURL, username), false, false
		}

		if existingProfileName != "" {
			// Found existing profile with same URL - use it
			profileName = existingProfileName
			wasUpdated = true
		} else {
			// New URL - prompt for profile name
			suggestedName := extractSessionName(baseURL, username)
			profileName = promptForProfileName(r, w, suggestedName)
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

// runOIDCLogin handles the OIDC authentication flow
//
// TODO: Future enhancement - Automatic callback flow (requires Izanami server changes)
// Once the Izanami server supports dynamic redirect_uri parameter, implement:
// 1. Start a local HTTP callback server on localhost (random or specified port)
// 2. Build OIDC URL with redirect_uri=http://localhost:<port>/callback
// 3. Open browser to OIDC URL
// 4. Wait for callback with JWT token (with timeout)
// 5. Extract token from callback request and save session
// 6. Shutdown callback server
//
// This would eliminate the need for manual token copy-paste.
// See feat-0003-oidc-login.md for detailed implementation plan.
//
// New flags needed:
//   --callback-port int    Local callback server port (default: random available port)
//   --timeout duration     Authentication timeout (default: 5m)
//
// Security considerations:
//   - Callback server must only bind to localhost (127.0.0.1)
//   - Use state parameter to prevent CSRF attacks
//   - Consider PKCE (Proof Key for Code Exchange) for additional security
func runOIDCLogin(cmd *cobra.Command, args []string) error {
	// Get base URL from args or global flag
	var oidcBaseURL string
	if len(args) > 0 {
		oidcBaseURL = args[0]
	} else if baseURL != "" {
		// Use global --url flag
		oidcBaseURL = baseURL
	} else if cfg != nil && cfg.BaseURL != "" {
		// Fall back to config
		oidcBaseURL = cfg.BaseURL
	}

	if oidcBaseURL == "" {
		return fmt.Errorf("base URL is required (use --url flag, provide as argument, or set IZ_BASE_URL)")
	}

	// Normalize URL
	oidcBaseURL = strings.TrimSuffix(oidcBaseURL, "/")

	// If token provided via flag, skip browser flow
	if loginToken != "" {
		return saveOIDCSession(cmd, oidcBaseURL, loginToken)
	}

	// Build OIDC URL
	oidcURL := oidcBaseURL + "/api/admin/openid-connect"

	// Open browser (unless --no-browser)
	if !loginNoBrowser {
		fmt.Fprintln(cmd.OutOrStderr(), "Opening browser for OIDC authentication...")
		if err := utils.OpenBrowser(oidcURL); err != nil {
			fmt.Fprintf(cmd.OutOrStderr(), "Warning: Could not open browser: %v\n", err)
		}
	}

	// Print URL for manual access
	fmt.Fprintf(cmd.OutOrStderr(), "\nIf browser doesn't open, visit:\n  %s\n\n", oidcURL)
	fmt.Fprintln(cmd.OutOrStderr(), "After authenticating, copy the JWT token from your browser cookies.")
	fmt.Fprintln(cmd.OutOrStderr(), "(In browser DevTools > Application > Cookies > look for 'token')")
	fmt.Fprint(cmd.OutOrStderr(), "\nPaste the token here: ")

	// TODO: Future enhancement - Replace manual token paste with callback server
	// When callback flow is implemented, this section would become:
	//   result, err := callbackServer.WaitForCallback(timeout)
	//   if err != nil { return err }
	//   token = result.Token

	// Read token from stdin (current manual flow)
	reader := bufio.NewReader(cmd.InOrStdin())
	token, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read token: %w", err)
	}
	token = strings.TrimSpace(token)

	if token == "" {
		return fmt.Errorf("no token provided")
	}

	// Save session
	return saveOIDCSession(cmd, oidcBaseURL, token)
}

// saveOIDCSession saves the OIDC session with the provided token
func saveOIDCSession(cmd *cobra.Command, baseURL, token string) error {
	// Decode username from JWT
	username := decodeJWTUsername(token)

	// Determine profile name
	profileName, profileCreated, profileUpdated := determineProfileName(cmd.InOrStdin(), cmd.OutOrStderr(), baseURL, username)

	// Generate session name
	sessionName := loginSessionName
	if sessionName == "" {
		sessionName = fmt.Sprintf("%s-%s-oidc", profileName, username)
	}

	// Load existing sessions
	sessions, err := izanami.LoadSessions()
	if err != nil {
		sessions = &izanami.Sessions{Sessions: make(map[string]*izanami.Session)}
	}

	// Create session
	session := &izanami.Session{
		URL:       baseURL,
		Username:  username,
		JwtToken:  token,
		CreatedAt: time.Now(),
	}

	// Check if session with same URL+username exists and overwrite
	for name, existingSession := range sessions.Sessions {
		if existingSession.URL == baseURL && existingSession.Username == username {
			if name != sessionName {
				delete(sessions.Sessions, name)
				fmt.Fprintf(cmd.OutOrStderr(), "   Replacing existing session: %s\n", name)
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
		fmt.Fprintf(cmd.OutOrStderr(), "\n   Warning: failed to update profile: %v\n", err)
	}

	// Success messages
	fmt.Fprintf(cmd.OutOrStderr(), "\n✅ Successfully logged in as %s (via OIDC)\n", username)
	fmt.Fprintf(cmd.OutOrStderr(), "   Session saved as: %s\n", sessionName)

	// Profile messages
	if profileCreated {
		fmt.Fprintf(cmd.OutOrStderr(), "\n✓ Profile '%s' created\n", profileName)
	} else if profileUpdated {
		fmt.Fprintf(cmd.OutOrStderr(), "\n   Using existing profile: %s (session updated)\n", profileName)
	}

	if profileName != "" {
		activeProfile, _ := izanami.GetActiveProfileName()
		if activeProfile == profileName {
			fmt.Fprintf(cmd.OutOrStderr(), "   Active profile: %s\n", profileName)
		}
	}

	fmt.Fprintf(cmd.OutOrStderr(), "\nYou can now run commands like:\n")
	fmt.Fprintf(cmd.OutOrStderr(), "  iz admin tenants list\n")
	fmt.Fprintf(cmd.OutOrStderr(), "  iz features list --tenant <tenant>\n")

	return nil
}

// decodeJWTUsername extracts the username from a JWT token without validation
func decodeJWTUsername(token string) string {
	// JWT is base64(header).base64(payload).signature
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "oidc-user"
	}

	// Decode payload (second part)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Try standard base64 with padding
		payload, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return "oidc-user"
		}
	}

	// Parse claims
	var claims struct {
		Sub      string `json:"sub"`
		Name     string `json:"name"`
		Username string `json:"username"`
		Email    string `json:"email"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "oidc-user"
	}

	// Return first non-empty field
	if claims.Username != "" {
		return claims.Username
	}
	if claims.Name != "" {
		return claims.Name
	}
	if claims.Sub != "" {
		return claims.Sub
	}
	if claims.Email != "" {
		// Use email prefix as username
		if at := strings.Index(claims.Email, "@"); at > 0 {
			return claims.Email[:at]
		}
		return claims.Email
	}

	return "oidc-user"
}

// generateOIDCSessionName generates a session name from the base URL
func generateOIDCSessionName(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "oidc-session"
	}
	return u.Host + "-oidc"
}

func init() {
	rootCmd.AddCommand(loginCmd)

	loginCmd.Flags().StringVar(&loginSessionName, "name", "", "Custom name for this session")
	loginCmd.Flags().StringVar(&loginPassword, "password", "", "Password (not recommended, use prompt instead)")

	// OIDC flags
	loginCmd.Flags().BoolVar(&loginOIDC, "oidc", false, "Use OIDC authentication")
	loginCmd.Flags().StringVar(&loginToken, "token", "", "JWT token for OIDC (skip browser flow)")
	loginCmd.Flags().BoolVar(&loginNoBrowser, "no-browser", false, "Don't open browser, just print URL")

	// TODO: Future enhancement - Add callback server flags when Izanami supports dynamic redirect_uri
	// loginCmd.Flags().IntVar(&loginCallbackPort, "callback-port", 0, "Local callback server port (0 = random)")
	// loginCmd.Flags().DurationVar(&loginTimeout, "timeout", 5*time.Minute, "OIDC authentication timeout")
}
