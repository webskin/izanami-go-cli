package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/errors"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"github.com/webskin/izanami-go-cli/internal/output"
)

var (
	sessionsDeleteForce bool
)

// sessionsCmd represents the sessions command
var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage authentication sessions",
	Long: `Manage saved authentication sessions.

Sessions store your JWT tokens from login. Sessions are referenced by profiles,
and you control which session is used by switching profiles.

Examples:
  # List all sessions
  iz sessions list

  # Delete a session
  iz sessions delete old-session

  # Switch profiles (which reference sessions)
  iz profiles use prod`,
}

// sessionsListCmd lists all sessions
var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		sessions, err := izanami.LoadSessions()
		if err != nil {
			return err
		}

		if len(sessions.Sessions) == 0 {
			fmt.Fprintln(cmd.OutOrStderr(), errors.MsgNoSavedSessions)
			return nil
		}

		// Format for display
		type SessionDisplay struct {
			Name      string `json:"name"`
			URL       string `json:"url"`
			Username  string `json:"username"`
			CreatedAt string `json:"created_at"`
			Age       string `json:"age"`
		}

		var displays []SessionDisplay
		for name, session := range sessions.Sessions {
			age := formatAge(time.Since(session.CreatedAt))

			displays = append(displays, SessionDisplay{
				Name:      name,
				URL:       session.URL,
				Username:  session.Username,
				CreatedAt: session.CreatedAt.Format("2006-01-02 15:04:05"),
				Age:       age,
			})
		}

		return output.Print(displays, output.Format(outputFormat))
	},
}

// sessionsDeleteCmd deletes a session
var sessionsDeleteCmd = &cobra.Command{
	Use:   "delete <session-name>",
	Short: "Delete a saved session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]

		// Confirm deletion unless --force is used
		if !sessionsDeleteForce {
			if !confirmDeletion(cmd, "session", sessionName) {
				return nil
			}
		}

		sessions, err := izanami.LoadSessions()
		if err != nil {
			return err
		}

		if err := sessions.DeleteSession(sessionName); err != nil {
			return err
		}

		if err := sessions.Save(); err != nil {
			return fmt.Errorf("%s: %w", errors.MsgFailedToSaveSessions, err)
		}

		fmt.Fprintf(cmd.OutOrStderr(), "✅ Deleted session: %s\n", sessionName)

		return nil
	},
}

// logoutCmd logs out of the current session
var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout from the current profile's session",
	Long: `Logout from the session referenced by the active profile.

This will remove the saved token but keep the session entry.
You will need to login again to use this session.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get active profile
		activeProfileName, err := izanami.GetActiveProfileName()
		if err != nil {
			return fmt.Errorf("no active profile: %w (use 'iz profiles use <name>' to set one)", err)
		}

		// Get profile
		profile, err := izanami.GetProfile(activeProfileName)
		if err != nil {
			return fmt.Errorf("failed to load active profile: %w", err)
		}

		// Check if profile has a session reference
		if profile.Session == "" {
			return fmt.Errorf("active profile '%s' does not reference a session", activeProfileName)
		}

		// Load sessions
		sessions, err := izanami.LoadSessions()
		if err != nil {
			return err
		}

		// Get the session
		session, err := sessions.GetSession(profile.Session)
		if err != nil {
			return fmt.Errorf("session '%s' not found: %w", profile.Session, err)
		}

		// Remove the token but keep the session
		session.JwtToken = ""
		session.CreatedAt = time.Time{} // Zero time

		if err := sessions.Save(); err != nil {
			return fmt.Errorf("%s: %w", errors.MsgFailedToSaveSessions, err)
		}

		fmt.Fprintf(cmd.OutOrStderr(), "✅ Logged out from session: %s\n", profile.Session)
		fmt.Fprintf(cmd.OutOrStderr(), "   Use 'iz login %s %s' to login again\n", session.URL, session.Username)

		return nil
	},
}

// formatAge formats a duration as a human-readable age
func formatAge(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	} else if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	} else if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else {
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

func init() {
	rootCmd.AddCommand(sessionsCmd)
	rootCmd.AddCommand(logoutCmd)

	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsDeleteCmd)

	// Delete flags
	sessionsDeleteCmd.Flags().BoolVarP(&sessionsDeleteForce, "force", "f", false, "Skip confirmation prompt")
}
