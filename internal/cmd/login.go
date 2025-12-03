package cmd

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/auth"
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
	loginTimeout     time.Duration
	loginPollInterval time.Duration
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
  The CLI opens a browser for OIDC login, then automatically polls the server
  until authentication completes - no manual token copying needed!

  If the server doesn't support automatic polling, you can use --token flag
  to provide the JWT token directly.

Examples:
  # Login with username/password
  iz login http://localhost:9000 RESERVED_ADMIN_USER

  # Login via OIDC (opens browser, waits for authentication)
  iz login --oidc --url https://izanami.prod.com

  # Login via OIDC with custom timeout
  iz login --oidc --url https://izanami.prod.com --timeout 10m

  # Login via OIDC with token directly (for scripting or fallback)
  iz login --oidc --url https://izanami.prod.com --token "eyJhbGciOiJIUzI1NiIs..."

  # Login via OIDC without opening browser (prints URL only)
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

		// Verbose: Log login attempt details
		if verbose {
			fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Login attempt:\n")
			fmt.Fprintf(cmd.OutOrStderr(), "[verbose]   URL: %s\n", baseURL)
			fmt.Fprintf(cmd.OutOrStderr(), "[verbose]   Username: %s\n", username)
		}

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
			if verbose {
				fmt.Fprintf(cmd.OutOrStderr(), "[verbose]   Password: <redacted> (%d chars)\n", len(password))
			}
		} else {
			if verbose {
				fmt.Fprintf(cmd.OutOrStderr(), "[verbose]   Password: <redacted from flag> (%d chars)\n", len(password))
			}
		}

		if password == "" {
			return fmt.Errorf("password cannot be empty")
		}

		// Login to Izanami
		fmt.Fprintf(cmd.OutOrStderr(), "Authenticating with %s...\n", baseURL)
		if verbose {
			fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Sending POST request to %s/api/admin/login\n", baseURL)
		}

		token, err := performLogin(baseURL, username, password)
		if err != nil {
			if verbose {
				fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Login failed: %v\n", err)
			}
			return fmt.Errorf("login failed: %w", err)
		}

		if verbose {
			fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Login successful\n")
			fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Token received: <redacted> (%d chars)\n", len(token))
		}

		// Determine profile name first (create or find existing)
		profileName, profileCreated, profileUpdated := determineProfileName(cmd.InOrStdin(), cmd.OutOrStderr(), baseURL, username)

		// Generate deterministic session name: <profile-name>-<username>-session
		sessionName := fmt.Sprintf("%s-%s-session", profileName, username)

		if verbose {
			fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Profile: %s (created: %v, updated: %v)\n", profileName, profileCreated, profileUpdated)
			fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Session name: %s\n", sessionName)
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

// runOIDCLogin handles the OIDC authentication flow with automatic token polling.
//
// # Authentication Flow
//
// This function implements a state-based token polling mechanism that works
// with any OIDC provider without requiring special configuration.
//
// ## Flow Decision
//
//  1. If --token flag provided: Skip browser, use token directly
//  2. If server supports /api/admin/cli-login: Use automatic polling flow
//  3. If server doesn't support it: Error with hint to use --token flag
//
// ## Automatic Polling Flow
//
//  1. Generate cryptographically secure state (32 bytes, base64url)
//  2. Open browser to /api/admin/cli-login?state={state}
//  3. Server redirects to OIDC provider with state prefixed as "cli:{state}"
//  4. User authenticates in browser
//  5. Server detects CLI flow, stores token for pickup
//  6. CLI polls /api/admin/cli-token?state={state} every 2 seconds
//  7. On success: Save session, display success message
//  8. On timeout/error: Display error with fallback instructions
//
// ## Security Features
//
//   - 256-bit state entropy prevents guessing attacks
//   - Single-use tokens (deleted from server after retrieval)
//   - Rate limiting on polling (60 requests/minute per state)
//   - Short TTLs (5 min pending auth, 2 min token pickup window)
//
// See features/feat-0003-oidc-login.md for full specification.
func runOIDCLogin(cmd *cobra.Command, args []string) error {
	// Verbose: Log OIDC login start
	if verbose {
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] OIDC login flow initiated\n")
	}

	// Get base URL from args or global flag
	var oidcBaseURL string
	if len(args) > 0 {
		oidcBaseURL = args[0]
		if verbose {
			fmt.Fprintf(cmd.OutOrStderr(), "[verbose] URL source: command argument\n")
		}
	} else if baseURL != "" {
		// Use global --url flag
		oidcBaseURL = baseURL
		if verbose {
			fmt.Fprintf(cmd.OutOrStderr(), "[verbose] URL source: --url flag\n")
		}
	} else if cfg != nil && cfg.BaseURL != "" {
		// Fall back to config
		oidcBaseURL = cfg.BaseURL
		if verbose {
			fmt.Fprintf(cmd.OutOrStderr(), "[verbose] URL source: config file\n")
		}
	}

	if oidcBaseURL == "" {
		return fmt.Errorf("base URL is required (use --url flag, provide as argument, or set IZ_BASE_URL)")
	}

	// Normalize URL - remove trailing slash for consistent URL building
	oidcBaseURL = strings.TrimSuffix(oidcBaseURL, "/")

	if verbose {
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Base URL: %s\n", oidcBaseURL)
	}

	// If token provided via flag, skip browser flow entirely
	// This is useful for scripting or when automatic polling isn't available
	if loginToken != "" {
		if verbose {
			fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Token provided via --token flag, skipping browser flow\n")
			fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Token: <redacted> (%d chars)\n", len(loginToken))
		}
		return saveOIDCSession(cmd, oidcBaseURL, loginToken)
	}

	// Check if server supports CLI OIDC authentication (state-based polling)
	// This endpoint was added in Izanami server to support CLI tools
	ctx := context.Background()
	if verbose {
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Checking server support for CLI OIDC authentication...\n")
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Probing: GET %s/api/admin/cli-login?state=check\n", oidcBaseURL)
	}
	if !auth.CheckServerSupport(ctx, oidcBaseURL) {
		if verbose {
			fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Server does not support CLI OIDC (endpoint returned 404)\n")
		}
		// Server doesn't support CLI auth - provide helpful error with workaround
		return fmt.Errorf(`server does not support CLI OIDC authentication

The server at %s does not have the /api/admin/cli-login endpoint
required for automatic token polling.

Workaround: Authenticate manually and provide the token:
  1. Visit: %s/api/admin/openid-connect
  2. After login, copy the JWT from browser cookies (DevTools > Application > Cookies > 'token')
  3. Run: iz login --oidc --url %s --token "your-jwt-token"`, oidcBaseURL, oidcBaseURL, oidcBaseURL)
	}

	if verbose {
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Server supports CLI OIDC authentication\n")
	}

	// Generate cryptographically secure state for this authentication session
	// This state correlates the browser authentication with this CLI session
	state, err := auth.GenerateState()
	if err != nil {
		return fmt.Errorf("failed to generate authentication state: %w", err)
	}

	if verbose {
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Generated state: %s (256-bit entropy)\n", state)
	}

	// Build CLI login URL with state parameter
	// The server will store this state and redirect to OIDC provider
	loginURL := fmt.Sprintf("%s/api/admin/cli-login?state=%s", oidcBaseURL, state)

	if verbose {
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Login URL: %s\n", loginURL)
	}

	// IMPORTANT: Initiate login via HTTP request BEFORE opening browser
	// This creates the pending auth on the server, avoiding a race condition where
	// polling starts before the browser request creates the pending auth.
	if verbose {
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Initiating login to create pending auth...\n")
	}

	redirectURL, err := initiateCliLogin(ctx, loginURL)
	if err != nil {
		return fmt.Errorf("failed to initiate CLI login: %w", err)
	}

	if verbose {
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Pending auth created, redirect URL: %s\n", redirectURL)
	}

	// Open browser to the OIDC provider (redirect URL) unless --no-browser flag is set
	browserURL := redirectURL
	if browserURL == "" {
		// Fallback to original login URL if no redirect (shouldn't happen)
		browserURL = loginURL
	}

	if !loginNoBrowser {
		if verbose {
			fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Opening browser (--no-browser: false)\n")
		}
		fmt.Fprintln(cmd.OutOrStderr(), "Opening browser for OIDC authentication...")
		if err := utils.OpenBrowser(browserURL); err != nil {
			// Browser open failed - not fatal, user can manually visit URL
			fmt.Fprintf(cmd.OutOrStderr(), "Warning: Could not open browser: %v\n", err)
			if verbose {
				fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Browser open error: %v\n", err)
			}
		} else if verbose {
			fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Browser opened successfully\n")
		}
	} else if verbose {
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Browser opening disabled (--no-browser: true)\n")
	}

	// Print URL for manual access (in case browser doesn't open or user prefers it)
	fmt.Fprintf(cmd.OutOrStderr(), "\nIf browser doesn't open, visit:\n  %s\n\n", browserURL)

	// Start spinner to indicate we're waiting for authentication
	// The spinner animates if terminal supports it, otherwise shows static message
	spinner := auth.NewSpinner(cmd.OutOrStderr(), "Waiting for authentication")
	spinner.Start()

	// Create token poller with configured interval
	// The poller will repeatedly check /api/admin/cli-token until token is ready
	pollInterval := loginPollInterval
	if pollInterval <= 0 {
		pollInterval = auth.DefaultPollInterval
	}
	poller := auth.NewTokenPoller(oidcBaseURL, state, pollInterval)

	// Wait for token with timeout
	// The poller handles rate limiting and transient errors automatically
	timeout := loginTimeout
	if timeout <= 0 {
		timeout = auth.DefaultTimeout
	}

	if verbose {
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Polling configuration:\n")
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose]   Endpoint: %s/api/admin/cli-token?state=%s\n", oidcBaseURL, state)
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose]   Poll interval: %v\n", pollInterval)
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose]   Timeout: %v\n", timeout)
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Starting polling loop...\n")
	}

	token, err := poller.WaitForToken(ctx, timeout)

	// Stop spinner before showing result
	if err != nil {
		// Authentication failed - show error with helpful fallback instructions
		if verbose {
			fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Polling failed: %v\n", err)
		}
		spinner.Error(fmt.Sprintf("Authentication failed: %v", err))
		fmt.Fprintf(cmd.OutOrStderr(), "\nTip: You can manually provide a token using:\n")
		fmt.Fprintf(cmd.OutOrStderr(), "  iz login --oidc --url %s --token \"your-jwt-token\"\n", oidcBaseURL)
		return err
	}

	// Authentication successful!
	if verbose {
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Polling successful, token received\n")
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Token: <redacted> (%d chars)\n", len(token))
	}
	spinner.Success("Authentication complete!")

	// Save the session with the received token
	return saveOIDCSession(cmd, oidcBaseURL, token)
}

// saveOIDCSession saves the OIDC session with the provided token
func saveOIDCSession(cmd *cobra.Command, baseURL, token string) error {
	if verbose {
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Saving OIDC session...\n")
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Base URL: %s\n", baseURL)
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Token: <redacted> (%d chars)\n", len(token))
	}

	// Decode username from JWT
	username := decodeJWTUsername(token)

	if verbose {
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Decoded username from JWT: %s\n", username)
	}

	// Determine profile name
	profileName, profileCreated, profileUpdated := determineProfileName(cmd.InOrStdin(), cmd.OutOrStderr(), baseURL, username)

	// Generate session name
	sessionName := loginSessionName
	if sessionName == "" {
		sessionName = fmt.Sprintf("%s-%s-oidc", profileName, username)
	}

	if verbose {
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Profile: %s (created: %v, updated: %v)\n", profileName, profileCreated, profileUpdated)
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Session name: %s\n", sessionName)
	}

	// Load existing sessions
	sessions, err := izanami.LoadSessions()
	if err != nil {
		if verbose {
			fmt.Fprintf(cmd.OutOrStderr(), "[verbose] No existing sessions found, creating new session store\n")
		}
		sessions = &izanami.Sessions{Sessions: make(map[string]*izanami.Session)}
	} else if verbose {
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Loaded %d existing sessions\n", len(sessions.Sessions))
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
				if verbose {
					fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Found existing session with same URL+username: %s\n", name)
				}
				delete(sessions.Sessions, name)
				fmt.Fprintf(cmd.OutOrStderr(), "   Replacing existing session: %s\n", name)
			}
			break
		}
	}

	sessions.AddSession(sessionName, session)

	if verbose {
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Saving session to disk...\n")
	}

	if err := sessions.Save(); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	if verbose {
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Session saved successfully\n")
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

// initiateCliLogin makes an HTTP request to the CLI login endpoint to create
// the pending auth on the server. This must be called BEFORE opening the browser
// to avoid a race condition where polling starts before the pending auth exists.
//
// Returns the redirect URL (to the OIDC provider) that should be opened in the browser.
func initiateCliLogin(ctx context.Context, loginURL string) (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		// Don't follow redirects - we want to capture the redirect URL
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, loginURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to initiate login: %w", err)
	}
	defer resp.Body.Close()

	// We expect a 302 redirect to the OIDC provider
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusTemporaryRedirect ||
		resp.StatusCode == http.StatusMovedPermanently || resp.StatusCode == http.StatusSeeOther {
		location := resp.Header.Get("Location")
		if location != "" {
			return location, nil
		}
		return "", fmt.Errorf("redirect response missing Location header")
	}

	// If we get 400, the state format might be invalid
	if resp.StatusCode == http.StatusBadRequest {
		return "", fmt.Errorf("invalid state format (server returned 400)")
	}

	// Any other response is unexpected
	return "", fmt.Errorf("unexpected response from server (status %d)", resp.StatusCode)
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
	loginCmd.Flags().StringVar(&loginToken, "token", "", "JWT token for OIDC (skip browser/polling flow)")
	loginCmd.Flags().BoolVar(&loginNoBrowser, "no-browser", false, "Don't open browser, just print URL")
	loginCmd.Flags().DurationVar(&loginTimeout, "timeout", 5*time.Minute, "OIDC authentication timeout")
	loginCmd.Flags().DurationVar(&loginPollInterval, "poll-interval", 2*time.Second, "Token polling interval")
}
