package cmd

import (
	"github.com/spf13/cobra"
)

var (
	// Admin authentication flags
	adminPATUsername          string
	adminJwtToken             string
	adminPersonalAccessToken  string
)

// adminCmd represents the admin command
var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Administrative operations",
	Long: `Perform administrative operations in Izanami.

Admin operations require elevated privileges and are typically used for:
  - Managing tenants and projects
  - Managing users and API keys
  - Configuring webhooks
  - Importing/exporting data
  - Global search

These operations require authentication via JWT token (from 'iz login') or Personal Access Token.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Load config first (parent's PersistentPreRunE)
		if rootCmd.PersistentPreRunE != nil {
			if err := rootCmd.PersistentPreRunE(cmd, args); err != nil {
				return err
			}
		}

		// Apply admin-specific authentication flags
		if adminPATUsername != "" {
			cfg.PersonalAccessTokenUsername = adminPATUsername
		}
		if adminJwtToken != "" {
			cfg.JwtToken = adminJwtToken
		}
		if adminPersonalAccessToken != "" {
			cfg.PersonalAccessToken = adminPersonalAccessToken
		}

		// Validate admin authentication
		if err := cfg.ValidateAdminAuth(); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(adminCmd)

	// Admin authentication flags (persistent for all admin commands)
	adminCmd.PersistentFlags().StringVar(&adminPATUsername, "personal-access-token-username", "", "Username for PAT authentication (env: IZ_PERSONAL_ACCESS_TOKEN_USERNAME)")
	adminCmd.PersistentFlags().StringVar(&adminJwtToken, "jwt-token", "", "JWT token for admin authentication (env: IZ_JWT_TOKEN)")
	adminCmd.PersistentFlags().StringVar(&adminPersonalAccessToken, "personal-access-token", "", "Personal access token for admin authentication (env: IZ_PERSONAL_ACCESS_TOKEN)")

	// Features (admin operations)
	adminCmd.AddCommand(featuresCmd)
	featuresCmd.AddCommand(featuresListCmd)
	featuresCmd.AddCommand(featuresGetCmd)
	featuresCmd.AddCommand(featuresCreateCmd)
	featuresCmd.AddCommand(featuresUpdateCmd)
	featuresCmd.AddCommand(featuresDeleteCmd)

	// Contexts (admin operations)
	adminCmd.AddCommand(contextsCmd)
}
