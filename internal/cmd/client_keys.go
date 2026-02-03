package cmd

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
	"syscall"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"golang.org/x/term"
)

// ============================================================================
// SHARED HELPERS
// ============================================================================

// formatClientKeysCount returns a summary of configured client keys.
// Returns "" for nil/empty maps, or "N tenant(s) configured" otherwise.
func formatClientKeysCount(clientKeys map[string]izanami.TenantClientKeysConfig) string {
	if len(clientKeys) == 0 {
		return ""
	}
	return fmt.Sprintf("%d tenant(s) configured", len(clientKeys))
}

// promptClientCredentials interactively prompts for client-id and client-secret.
// The secret is read with terminal echo disabled.
func promptClientCredentials(cmd *cobra.Command, reader *bufio.Reader) (string, string, error) {
	fmt.Fprintf(cmd.OutOrStderr(), "Client ID: ")
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", "", fmt.Errorf("failed to read client ID: %w", err)
	}
	clientID := strings.TrimSpace(line)
	if clientID == "" {
		return "", "", fmt.Errorf("client ID cannot be empty")
	}

	fmt.Fprintf(cmd.OutOrStderr(), "Client Secret: ")
	secretBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(cmd.OutOrStderr()) // New line after password input
	if err != nil {
		return "", "", fmt.Errorf("failed to read client secret: %w", err)
	}
	clientSecret := strings.TrimSpace(string(secretBytes))
	if clientSecret == "" {
		return "", "", fmt.Errorf("client secret cannot be empty")
	}

	return clientID, clientSecret, nil
}

// confirmOverwrite prompts the user to confirm overwriting existing credentials.
// Returns true if the user confirms, false otherwise.
func confirmOverwrite(cmd *cobra.Command, reader *bufio.Reader, label string) (bool, error) {
	fmt.Fprintf(cmd.OutOrStderr(), "\nAlready has credentials for %s.\n", label)
	fmt.Fprintf(cmd.OutOrStderr(), "Overwrite existing credentials? (y/N): ")
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return false, fmt.Errorf("failed to read input: %w", err)
	}
	if strings.ToLower(strings.TrimSpace(line)) != "y" {
		fmt.Fprintln(cmd.OutOrStderr(), "Aborted.")
		return false, nil
	}
	return true, nil
}

// checkExistingKeys returns true if credentials already exist for the given tenant/projects.
func checkExistingKeys(clientKeys map[string]izanami.TenantClientKeysConfig, tenant string, projects []string) bool {
	if clientKeys == nil {
		return false
	}
	tenantConfig, exists := clientKeys[tenant]
	if !exists {
		return false
	}
	if len(projects) == 0 {
		return tenantConfig.ClientID != ""
	}
	if tenantConfig.Projects == nil {
		return false
	}
	for _, project := range projects {
		if projConfig, projExists := tenantConfig.Projects[project]; projExists && projConfig.ClientID != "" {
			return true
		}
	}
	return false
}

// printClientKeysSummary prints a formatted summary of a ClientKeys map.
func printClientKeysSummary(w io.Writer, clientKeys map[string]izanami.TenantClientKeysConfig, showSecrets bool) {
	if len(clientKeys) == 0 {
		fmt.Fprintln(w, "  No client keys configured")
		return
	}

	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"TENANT", "SCOPE", "CLIENT-ID", "CLIENT-SECRET"})
	table.SetBorder(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetTablePadding("\t")
	table.SetNoWhiteSpace(true)

	tenants := make([]string, 0, len(clientKeys))
	for t := range clientKeys {
		tenants = append(tenants, t)
	}
	sort.Strings(tenants)

	for _, tenant := range tenants {
		cfg := clientKeys[tenant]
		secret := cfg.ClientSecret
		if !showSecrets && secret != "" {
			secret = izanami.RedactedValue
		}

		if cfg.ClientID != "" {
			table.Append([]string{tenant, "(tenant)", cfg.ClientID, secret})
		}

		if cfg.Projects != nil {
			projects := make([]string, 0, len(cfg.Projects))
			for p := range cfg.Projects {
				projects = append(projects, p)
			}
			sort.Strings(projects)

			for _, proj := range projects {
				pcfg := cfg.Projects[proj]
				psecret := pcfg.ClientSecret
				if !showSecrets && psecret != "" {
					psecret = izanami.RedactedValue
				}
				table.Append([]string{tenant, proj, pcfg.ClientID, psecret})
			}
		}
	}

	table.Render()
}

// printSecurityWarning prints the standard security warning for stored credentials.
func printSecurityWarning(w io.Writer) {
	fmt.Fprintln(w, "\nSECURITY WARNING:")
	fmt.Fprintln(w, "   Credentials are stored in plaintext in the config file.")
	fmt.Fprintln(w, "   File permissions are set to 0600 (owner read/write only).")
	fmt.Fprintln(w, "   Never commit config.yaml to version control.")
}

// ============================================================================
// PROFILE CLIENT-KEYS COMMANDS
// ============================================================================

// profileClientKeysCmd represents the profiles client-keys command
var profileClientKeysCmd = &cobra.Command{
	Use:   "client-keys",
	Short: "Manage client API keys in active profile",
	Long: `Manage client API keys (client-id/client-secret) for feature evaluation.

Client keys are stored in the active profile and are environment-specific.
Each profile (local, sandbox, build, prod) can have different client keys
for different tenants and projects.

Keys can be stored:
  - At the tenant level (for all projects in that tenant)
  - At the project level (for specific projects only)`,
}

// profileClientKeysAddCmd represents the profiles client-keys add command
var profileClientKeysAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add client credentials to active profile",
	Long: `Add client API credentials (client-id and client-secret) to the active profile.

Credentials are stored in the active profile and are environment-specific.
Different profiles (local, sandbox, build, prod) can have different credentials.

Credentials can be stored:
  - At the tenant level (for all projects in that tenant)
  - At the project level (for specific projects only)

The 'iz features check' command will automatically use these credentials with
the following precedence:
  1. --client-id/--client-secret flags (highest priority)
  2. IZ_CLIENT_ID/IZ_CLIENT_SECRET environment variables
  3. Worker's client-keys (if using a named worker)
  4. Stored credentials from active profile (this command)

Note: If you use a separate server for client operations (features/events),
configure worker-level credentials using:
  iz profiles workers client-keys add --tenant <tenant>

Examples:
  # First, switch to the profile you want to configure
  iz profiles use sandbox

  # Add tenant-wide credentials to the active profile (interactive)
  iz profiles client-keys add --tenant my-tenant

  # Add project-specific credentials to the active profile
  iz profiles client-keys add --tenant my-tenant --projects proj1,proj2

  # Non-interactive: provide client-id, prompt for secret (recommended)
  iz profiles client-keys add --tenant my-tenant --client-id xxx

  # Fully non-interactive (for scripts/CI)
  iz profiles client-keys add --tenant my-tenant --client-id xxx --client-secret yyy

Security:
  Credentials are stored in plaintext in ~/.config/iz/config.yaml
  File permissions are automatically set to 0600 (owner read/write only)
  Never commit config.yaml to version control`,
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, _ := cmd.Flags().GetString("tenant")
		projects, _ := cmd.Flags().GetStringSlice("projects")
		clientID, _ := cmd.Flags().GetString("client-id")
		clientSecret, _ := cmd.Flags().GetString("client-secret")

		if tenant == "" {
			return fmt.Errorf("--tenant is required")
		}

		if clientSecret != "" && clientID == "" {
			return fmt.Errorf("--client-id is required when --client-secret is provided")
		}

		profileName, err := izanami.GetActiveProfileName()
		if err != nil {
			return err
		}
		if profileName == "" {
			return fmt.Errorf("no active profile. Use 'iz profiles use <name>' to select a profile first")
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Adding credentials to profile: %s\n\n", profileName)

		reader := bufio.NewReader(cmd.InOrStdin())

		if clientID == "" {
			// Fully interactive: prompt for both
			clientID, clientSecret, err = promptClientCredentials(cmd, reader)
			if err != nil {
				return err
			}
		} else if clientSecret == "" {
			// --client-id provided, prompt only for secret (hidden input)
			fmt.Fprintf(cmd.OutOrStderr(), "Client Secret: ")
			secretBytes, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Fprintln(cmd.OutOrStderr())
			if err != nil {
				return fmt.Errorf("failed to read client secret: %w", err)
			}
			clientSecret = strings.TrimSpace(string(secretBytes))
			if clientSecret == "" {
				return fmt.Errorf("client secret cannot be empty")
			}
		}
		// else: both provided via flags, no prompts needed

		// Check if credentials already exist and prompt for confirmation
		// Skip interactive confirm when --client-id was provided (user explicitly passed values)
		if !cmd.Flags().Changed("client-id") {
			profile, err := izanami.GetProfile(profileName)
			if err == nil && checkExistingKeys(profile.ClientKeys, tenant, projects) {
				label := fmt.Sprintf("tenant '%s'", tenant)
				if len(projects) > 0 {
					label = fmt.Sprintf("'%s/%s'", tenant, strings.Join(projects, ","))
				}
				ok, err := confirmOverwrite(cmd, reader, label)
				if err != nil {
					return err
				}
				if !ok {
					return nil
				}
			}
		}

		if err := izanami.AddClientKeys(tenant, projects, clientID, clientSecret); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}

		if len(projects) == 0 {
			fmt.Fprintf(cmd.OutOrStderr(), "\nClient credentials saved to profile '%s' for tenant '%s'\n", profileName, tenant)
		} else {
			fmt.Fprintf(cmd.OutOrStderr(), "\nClient credentials saved to profile '%s' for tenant '%s', projects: %s\n", profileName, tenant, strings.Join(projects, ", "))
		}

		printSecurityWarning(cmd.OutOrStderr())

		fmt.Fprintf(cmd.OutOrStderr(), "\nYou can now use these credentials with:\n")
		fmt.Fprintf(cmd.OutOrStderr(), "  iz features check --tenant %s <feature-id>\n", tenant)

		return nil
	},
}

// profileClientKeysListCmd represents the profiles client-keys list command
var profileClientKeysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List client credentials in active profile",
	Long: `List all client API credentials stored in the active profile.

Shows credentials organized by tenant, with project-specific overrides indented.
Credentials are redacted by default; use --show-secrets to display them.

Example:
  iz profiles client-keys list
  iz profiles client-keys list --show-secrets`,
	RunE: func(cmd *cobra.Command, args []string) error {
		showSecrets, _ := cmd.Flags().GetBool("show-secrets")

		profileName, err := izanami.GetActiveProfileName()
		if err != nil || profileName == "" {
			return fmt.Errorf("no active profile. Use 'iz profiles use <name>' first")
		}

		profile, err := izanami.GetProfile(profileName)
		if err != nil {
			return err
		}

		if len(profile.ClientKeys) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No client keys configured in profile '"+profileName+"'")
			fmt.Fprintln(cmd.OutOrStdout(), "\nTo add client keys: iz profiles client-keys add --tenant <tenant>")
			return nil
		}

		printClientKeysSummary(cmd.OutOrStdout(), profile.ClientKeys, showSecrets)
		return nil
	},
}

// profileClientKeysDeleteCmd represents the profiles client-keys delete command
var profileClientKeysDeleteCmd = &cobra.Command{
	Use:   "delete <client-id>",
	Short: "Delete client credentials from active profile",
	Long: `Delete client credentials from the active profile.

Requires --tenant to specify the tenant. Use --project to delete
project-level credentials, otherwise deletes tenant-level credentials.

Examples:
  # Delete tenant-level credentials
  iz profiles client-keys delete --tenant my-tenant my-client-id

  # Delete project-level credentials
  iz profiles client-keys delete --tenant my-tenant --project proj1 proj1-client-id`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		clientID := args[0]
		tenant, _ := cmd.Flags().GetString("tenant")
		project, _ := cmd.Flags().GetString("project")

		profileName, err := izanami.GetActiveProfileName()
		if err != nil || profileName == "" {
			return fmt.Errorf("no active profile. Use 'iz profiles use <name>' first")
		}

		profile, err := izanami.GetProfile(profileName)
		if err != nil {
			return err
		}

		cfg, exists := profile.ClientKeys[tenant]
		if !exists {
			return fmt.Errorf("tenant '%s' not found in profile '%s'", tenant, profileName)
		}

		if project == "" {
			if cfg.ClientID != clientID {
				return fmt.Errorf("client-id '%s' not found at tenant level for '%s'", clientID, tenant)
			}
			cfg.ClientID = ""
			cfg.ClientSecret = ""
			profile.ClientKeys[tenant] = cfg
		} else {
			if cfg.Projects == nil {
				return fmt.Errorf("project '%s' not found in tenant '%s'", project, tenant)
			}
			pcfg, exists := cfg.Projects[project]
			if !exists {
				return fmt.Errorf("project '%s' not found in tenant '%s'", project, tenant)
			}
			if pcfg.ClientID != clientID {
				return fmt.Errorf("client-id '%s' not found for project '%s/%s'", clientID, tenant, project)
			}
			delete(cfg.Projects, project)
			profile.ClientKeys[tenant] = cfg
		}

		// Clean up: remove tenant entry if empty
		cfg = profile.ClientKeys[tenant]
		if cfg.ClientID == "" && len(cfg.Projects) == 0 {
			delete(profile.ClientKeys, tenant)
		}

		if err := izanami.AddProfile(profileName, profile); err != nil {
			return fmt.Errorf("failed to save profile: %w", err)
		}

		if project == "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted credentials for tenant '%s' (client-id: %s)\n", tenant, clientID)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted credentials for project '%s/%s' (client-id: %s)\n", tenant, project, clientID)
		}
		return nil
	},
}

// ============================================================================
// WORKER CLIENT-KEYS COMMANDS
// ============================================================================

var (
	workerClientKeysWorker string // --worker flag for worker client-keys commands
)

// workerClientKeysCmd is the parent command for worker client-keys management
var workerClientKeysCmd = &cobra.Command{
	Use:   "client-keys",
	Short: "Manage client API keys for workers",
	Long: `Manage client API keys (client-id/client-secret) for worker instances.

Worker-level client keys override profile-level keys when a worker is selected.
If --worker is omitted, targets the default worker (from profile's default-worker).

Keys can be stored:
  - At the tenant level (for all projects in that tenant)
  - At the project level (for specific projects only)

Credential resolution precedence for feature checks:
  1. --client-id/--client-secret flags (highest priority)
  2. IZ_CLIENT_ID/IZ_CLIENT_SECRET environment variables
  3. Worker's client-keys (this command)
  4. Profile's client-keys (iz profiles client-keys)`,
}

// workerClientKeysAddCmd adds client credentials to a worker
var workerClientKeysAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add client credentials to a worker",
	Long: `Add client API credentials (client-id and client-secret) to a named worker.

When --worker is omitted, targets the default worker (from profile's default-worker).

Examples:
  # Add tenant-wide credentials to the default worker (interactive)
  iz profiles workers client-keys add --tenant my-tenant

  # Add project-specific credentials to a named worker
  iz profiles workers client-keys add --tenant my-tenant --projects proj1,proj2 --worker eu-west

  # Non-interactive: provide client-id, prompt for secret (recommended)
  iz profiles workers client-keys add --tenant my-tenant --client-id xxx

  # Fully non-interactive (for scripts/CI)
  iz profiles workers client-keys add --tenant my-tenant --client-id xxx --client-secret yyy

Security:
  Credentials are stored in plaintext in ~/.config/iz/config.yaml
  File permissions are automatically set to 0600 (owner read/write only)
  Never commit config.yaml to version control`,
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, _ := cmd.Flags().GetString("tenant")
		projects, _ := cmd.Flags().GetStringSlice("projects")
		clientID, _ := cmd.Flags().GetString("client-id")
		clientSecret, _ := cmd.Flags().GetString("client-secret")

		if tenant == "" {
			return fmt.Errorf("--tenant is required")
		}

		if clientSecret != "" && clientID == "" {
			return fmt.Errorf("--client-id is required when --client-secret is provided")
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Adding credentials to worker")
		if workerClientKeysWorker != "" {
			fmt.Fprintf(cmd.OutOrStderr(), " '%s'", workerClientKeysWorker)
		} else {
			fmt.Fprintf(cmd.OutOrStderr(), " (default)")
		}
		fmt.Fprintln(cmd.OutOrStderr())
		fmt.Fprintln(cmd.OutOrStderr())

		reader := bufio.NewReader(cmd.InOrStdin())

		var err error
		if clientID == "" {
			// Fully interactive: prompt for both
			clientID, clientSecret, err = promptClientCredentials(cmd, reader)
			if err != nil {
				return err
			}
		} else if clientSecret == "" {
			// --client-id provided, prompt only for secret (hidden input)
			fmt.Fprintf(cmd.OutOrStderr(), "Client Secret: ")
			secretBytes, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Fprintln(cmd.OutOrStderr())
			if err != nil {
				return fmt.Errorf("failed to read client secret: %w", err)
			}
			clientSecret = strings.TrimSpace(string(secretBytes))
			if clientSecret == "" {
				return fmt.Errorf("client secret cannot be empty")
			}
		}
		// else: both provided via flags, no prompts needed

		// Check if credentials already exist and prompt for confirmation
		// Skip interactive confirm when --client-id was provided (user explicitly passed values)
		if !cmd.Flags().Changed("client-id") {
			existingKeys, workerName, err := izanami.ListWorkerClientKeys(workerClientKeysWorker)
			if err == nil && checkExistingKeys(existingKeys, tenant, projects) {
				label := fmt.Sprintf("worker '%s', tenant '%s'", workerName, tenant)
				if len(projects) > 0 {
					label = fmt.Sprintf("worker '%s', '%s/%s'", workerName, tenant, strings.Join(projects, ","))
				}
				ok, err := confirmOverwrite(cmd, reader, label)
				if err != nil {
					return err
				}
				if !ok {
					return nil
				}
			}
		}

		if err := izanami.AddWorkerClientKeys(workerClientKeysWorker, tenant, projects, clientID, clientSecret); err != nil {
			return err
		}

		// Resolve the actual worker name for display
		if workerClientKeysWorker == "" {
			_, workerName, _ := izanami.ListWorkerClientKeys("")
			if workerName != "" {
				workerClientKeysWorker = workerName
			}
		}

		if len(projects) == 0 {
			fmt.Fprintf(cmd.OutOrStderr(), "\nClient credentials saved to worker '%s' for tenant '%s'\n", workerClientKeysWorker, tenant)
		} else {
			fmt.Fprintf(cmd.OutOrStderr(), "\nClient credentials saved to worker '%s' for tenant '%s', projects: %s\n", workerClientKeysWorker, tenant, strings.Join(projects, ", "))
		}

		printSecurityWarning(cmd.OutOrStderr())

		return nil
	},
}

// workerClientKeysListCmd lists client credentials for a worker
var workerClientKeysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List client credentials for a worker",
	Long: `List all client API credentials stored for a worker.

When --worker is omitted, lists credentials for the default worker.
Credentials are redacted by default; use --show-secrets to display them.

Examples:
  iz profiles workers client-keys list
  iz profiles workers client-keys list --worker eu-west
  iz profiles workers client-keys list --show-secrets`,
	RunE: func(cmd *cobra.Command, args []string) error {
		showSecrets, _ := cmd.Flags().GetBool("show-secrets")

		clientKeys, workerName, err := izanami.ListWorkerClientKeys(workerClientKeysWorker)
		if err != nil {
			return err
		}

		if len(clientKeys) == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "No client keys configured for worker '%s'\n", workerName)
			fmt.Fprintln(cmd.OutOrStdout(), "\nTo add client keys: iz profiles workers client-keys add --tenant <tenant>")
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Client keys for worker '%s':\n", workerName)
		printClientKeysSummary(cmd.OutOrStdout(), clientKeys, showSecrets)
		return nil
	},
}

// workerClientKeysDeleteCmd deletes client credentials from a worker
var workerClientKeysDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete client credentials from a worker",
	Long: `Delete client credentials from a named worker.

When --worker is omitted, targets the default worker.
Requires --tenant to specify the tenant. Use --project to delete
project-level credentials, otherwise deletes tenant-level credentials.

Examples:
  # Delete tenant-level credentials from default worker
  iz profiles workers client-keys delete --tenant my-tenant

  # Delete project-level credentials from a named worker
  iz profiles workers client-keys delete --tenant my-tenant --project proj1 --worker eu-west`,
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, _ := cmd.Flags().GetString("tenant")
		project, _ := cmd.Flags().GetString("project")

		if tenant == "" {
			return fmt.Errorf("--tenant is required")
		}

		if err := izanami.DeleteWorkerClientKeys(workerClientKeysWorker, tenant, project); err != nil {
			return err
		}

		workerDisplay := workerClientKeysWorker
		if workerDisplay == "" {
			workerDisplay = "(default)"
		}

		if project == "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted credentials for tenant '%s' from worker %s\n", tenant, workerDisplay)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted credentials for '%s/%s' from worker %s\n", tenant, project, workerDisplay)
		}
		return nil
	},
}

func init() {
	// Register worker client-keys under profileWorkersCmd
	profileWorkersCmd.AddCommand(workerClientKeysCmd)

	workerClientKeysCmd.AddCommand(workerClientKeysAddCmd)
	workerClientKeysCmd.AddCommand(workerClientKeysListCmd)
	workerClientKeysCmd.AddCommand(workerClientKeysDeleteCmd)

	// Shared --worker flag for all worker client-keys subcommands
	workerClientKeysAddCmd.Flags().StringVar(&workerClientKeysWorker, "worker", "", "Worker name (default: profile's default-worker)")
	workerClientKeysListCmd.Flags().StringVar(&workerClientKeysWorker, "worker", "", "Worker name (default: profile's default-worker)")
	workerClientKeysDeleteCmd.Flags().StringVar(&workerClientKeysWorker, "worker", "", "Worker name (default: profile's default-worker)")

	// Worker name completions
	workerClientKeysAddCmd.RegisterFlagCompletionFunc("worker", completeWorkerNames)
	workerClientKeysListCmd.RegisterFlagCompletionFunc("worker", completeWorkerNames)
	workerClientKeysDeleteCmd.RegisterFlagCompletionFunc("worker", completeWorkerNames)

	// Flags for worker client-keys add
	workerClientKeysAddCmd.Flags().String("tenant", "", "Tenant name (required)")
	workerClientKeysAddCmd.Flags().StringSlice("projects", []string{}, "Project names (comma-separated)")
	workerClientKeysAddCmd.Flags().String("client-id", "", "Client ID (omit for interactive prompt)")
	workerClientKeysAddCmd.Flags().String("client-secret", "", "Client secret (omit for interactive prompt)")
	workerClientKeysAddCmd.MarkFlagRequired("tenant")

	// Flags for worker client-keys list
	workerClientKeysListCmd.Flags().Bool("show-secrets", false, "Show sensitive values (client secrets)")

	// Flags for worker client-keys delete
	workerClientKeysDeleteCmd.Flags().String("tenant", "", "Tenant name (required)")
	workerClientKeysDeleteCmd.Flags().String("project", "", "Project name (optional, for project-level credentials)")
	workerClientKeysDeleteCmd.MarkFlagRequired("tenant")
}
