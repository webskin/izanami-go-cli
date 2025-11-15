package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/errors"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"github.com/webskin/izanami-go-cli/internal/output"
)

// sessionsCmd represents the sessions command
var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage authentication sessions",
	Long: `Manage saved authentication sessions.

Sessions store your login information so you don't need to re-authenticate
every time. You can have multiple sessions for different Izanami instances
and switch between them.

Examples:
  # List all sessions
  iz sessions list

  # Switch to a different session
  iz sessions use prod

  # Delete a session
  iz sessions delete old-session`,
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
			fmt.Fprintln(os.Stderr, errors.MsgNoSavedSessions)
			return nil
		}

		// Format for display
		type SessionDisplay struct {
			Name      string `json:"name"`
			Active    string `json:"active"`
			URL       string `json:"url"`
			Username  string `json:"username"`
			CreatedAt string `json:"created_at"`
			Age       string `json:"age"`
		}

		var displays []SessionDisplay
		for name, session := range sessions.Sessions {
			active := ""
			if name == sessions.Active {
				active = "✓"
			}

			age := formatAge(time.Since(session.CreatedAt))

			displays = append(displays, SessionDisplay{
				Name:      name,
				Active:    active,
				URL:       session.URL,
				Username:  session.Username,
				CreatedAt: session.CreatedAt.Format("2006-01-02 15:04:05"),
				Age:       age,
			})
		}

		return output.Print(displays, output.Format(outputFormat))
	},
}

// sessionsUseCmd switches the active session
var sessionsUseCmd = &cobra.Command{
	Use:   "use <session-name>",
	Short: "Switch to a different session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]

		sessions, err := izanami.LoadSessions()
		if err != nil {
			return err
		}

		if err := sessions.SetActiveSession(sessionName); err != nil {
			return err
		}

		if err := sessions.Save(); err != nil {
			return fmt.Errorf("%s: %w", errors.MsgFailedToSaveSessions, err)
		}

		session, _ := sessions.GetSession(sessionName)
		fmt.Fprintf(os.Stderr, "✅ Switched to session: %s\n", sessionName)
		fmt.Fprintf(os.Stderr, "   URL: %s\n", session.URL)
		fmt.Fprintf(os.Stderr, "   User: %s\n", session.Username)

		return nil
	},
}

// sessionsDeleteCmd deletes a session
var sessionsDeleteCmd = &cobra.Command{
	Use:   "delete <session-name>",
	Short: "Delete a saved session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]

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

		fmt.Fprintf(os.Stderr, "✅ Deleted session: %s\n", sessionName)
		if sessions.Active != "" {
			fmt.Fprintf(os.Stderr, "   Active session is now: %s\n", sessions.Active)
		} else {
			fmt.Fprintf(os.Stderr, "   No active session. Use 'iz login' to create one.\n")
		}

		return nil
	},
}

// logoutCmd logs out of the current session
var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout from the current session",
	Long: `Logout from the currently active session.

This will remove the saved token but keep the session entry.
You will need to login again to use this session.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sessions, err := izanami.LoadSessions()
		if err != nil {
			return err
		}

		if sessions.Active == "" {
			return fmt.Errorf(errors.MsgNoActiveSession)
		}

		session, err := sessions.GetSession(sessions.Active)
		if err != nil {
			return err
		}

		// Remove the token but keep the session
		session.JwtToken = ""
		session.CreatedAt = time.Time{} // Zero time

		if err := sessions.Save(); err != nil {
			return fmt.Errorf("%s: %w", errors.MsgFailedToSaveSessions, err)
		}

		fmt.Fprintf(os.Stderr, "✅ Logged out from session: %s\n", sessions.Active)
		fmt.Fprintf(os.Stderr, "   Use 'iz login %s %s' to login again\n", session.URL, session.Username)

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
	sessionsCmd.AddCommand(sessionsUseCmd)
	sessionsCmd.AddCommand(sessionsDeleteCmd)
}
