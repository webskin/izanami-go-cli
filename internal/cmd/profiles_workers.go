package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

var (
	workerDeleteForce bool
	workerAddForce    bool
)

// profileWorkersCmd is the parent command for worker management
var profileWorkersCmd = &cobra.Command{
	Use:   "workers",
	Short: "Manage worker instances in active profile",
	Long: `Manage named worker instances for split deployments.

Workers are separate Izanami instances that handle client operations
(feature checks, event streaming) while the leader handles admin operations.

In standalone deployments (single server), workers are not needed.
Feature checks will use the leader URL directly.

Workers are stored in the active profile. Each worker has a URL and
optionally its own client-id/client-secret credentials.

Use subcommands to add, list, delete, and configure workers.`,
}

// profileWorkersAddCmd adds a new worker
var profileWorkersAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a worker to active profile",
	Long: `Add a named worker instance to the active profile.

The first worker added to a profile automatically becomes the default worker.

To add client credentials for a worker, use:
  iz profiles workers client-keys add --tenant <tenant> --worker <name>

Examples:
  iz profiles workers add eu-west --url https://worker-eu.example.com
  iz profiles workers add local --url http://localhost:9001 --force

  # Add and set as default worker
  iz profiles workers add eu-west --url https://worker-eu.example.com --default`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		url, _ := cmd.Flags().GetString("url")
		setDefault, _ := cmd.Flags().GetBool("default")

		if url == "" {
			return fmt.Errorf("--url is required")
		}

		worker := &izanami.WorkerConfig{
			URL: url,
		}

		if err := izanami.AddWorker(name, worker, workerAddForce); err != nil {
			return err
		}

		// Set as default if requested and not already default
		// (AddWorker auto-defaults the first worker; check actual state)
		profileName, _ := izanami.GetActiveProfileName()
		if setDefault {
			profile, err := izanami.GetProfile(profileName)
			if err == nil && profile.DefaultWorker != name {
				if err := izanami.SetDefaultWorker(name); err != nil {
					return fmt.Errorf("worker added but failed to set as default: %w", err)
				}
			}
		}

		// Re-read to get final state for output
		profile, err := izanami.GetProfile(profileName)
		isDefault := err == nil && profile.DefaultWorker == name

		fmt.Fprintf(cmd.OutOrStdout(), "Added worker '%s' to profile '%s'\n", name, profileName)
		fmt.Fprintf(cmd.OutOrStdout(), "  URL: %s\n", url)

		if isDefault {
			fmt.Fprintf(cmd.OutOrStdout(), "  Set as default worker\n")
		}

		fmt.Fprintf(cmd.OutOrStdout(), "\nTo add client credentials for this worker:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  iz profiles workers client-keys add --tenant <tenant> --worker %s\n", name)

		return nil
	},
}

// profileWorkersDeleteCmd deletes a worker
var profileWorkersDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a worker from active profile",
	Long: `Delete a named worker from the active profile.

If the deleted worker is the default, the default-worker setting is cleared
with a warning.

Examples:
  iz profiles workers delete eu-west
  iz profiles workers delete us-east --force`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Check if this is the default worker
		profileName, _ := izanami.GetActiveProfileName()
		if profileName != "" {
			profile, err := izanami.GetProfile(profileName)
			if err == nil && profile.DefaultWorker == name {
				if !workerDeleteForce {
					if !confirmDeletion(cmd, "default worker", name) {
						return nil
					}
				}
				fmt.Fprintf(cmd.OutOrStderr(), "Warning: '%s' was the default worker. Default worker cleared.\n", name)
			}
		}

		if err := izanami.DeleteWorker(name); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Deleted worker '%s'\n", name)
		return nil
	},
}

// profileWorkersListCmd lists all workers
var profileWorkersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workers in active profile",
	Long: `List all named workers configured in the active profile.

The default worker is marked with [default].

Example:
  iz profiles workers list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName, err := izanami.GetActiveProfileName()
		if err != nil || profileName == "" {
			return fmt.Errorf("no active profile. Use 'iz profiles use <name>' first")
		}

		profile, err := izanami.GetProfile(profileName)
		if err != nil {
			return err
		}

		if len(profile.Workers) == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "No workers configured in profile '%s'\n", profileName)
			fmt.Fprintln(cmd.OutOrStdout(), "\nTo add a worker:")
			fmt.Fprintln(cmd.OutOrStdout(), "  iz profiles workers add <name> --url <url>")
			return nil
		}

		table := tablewriter.NewWriter(cmd.OutOrStdout())
		table.SetHeader([]string{"", "NAME", "URL", "CLIENT-KEYS"})
		table.SetBorder(false)
		table.SetColumnSeparator("")
		table.SetHeaderLine(false)
		table.SetAutoWrapText(false)

		// Sort worker names
		names := make([]string, 0, len(profile.Workers))
		for name := range profile.Workers {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			worker := profile.Workers[name]
			marker := " "
			if name == profile.DefaultWorker {
				marker = "*"
			}

			clientKeysDisplay := formatClientKeysCount(worker.ClientKeys)
			if clientKeysDisplay == "" {
				clientKeysDisplay = "-"
			}

			table.Append([]string{marker, name, worker.URL, clientKeysDisplay})
		}

		table.Render()

		if profile.DefaultWorker != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "\nDefault worker: %s\n", profile.DefaultWorker)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "\nNo default worker set. Use: iz profiles workers use <name>")
		}

		return nil
	},
}

// profileWorkersUseCmd sets the default worker
var profileWorkersUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set default worker for active profile",
	Long: `Set the default worker for the active profile.

The default worker is used for feature checks and event streaming when
no --worker flag or IZ_WORKER env var is specified.

Example:
  iz profiles workers use eu-west`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if err := izanami.SetDefaultWorker(name); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Default worker set to '%s'\n", name)
		return nil
	},
}

// profileWorkersCurrentCmd shows the resolved worker
var profileWorkersCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show resolved worker with source info",
	Long: `Show which worker would be used for feature checks.

Runs the full worker resolution (flag/env/default/standalone) and displays
the result. Useful as a diagnostic tool.

Example:
  iz profiles workers current`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load config with profile
		resolved, resolvedProfile, err := izanami.LoadConfigWithProfile(profileName)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Apply flag/env overrides
		resolved.MergeWithFlags(izanami.FlagValues{
			LeaderURL: getValueWithEnvFallback(leaderURL, "IZ_LEADER_URL"),
		})

		// Resolve worker
		workers, defaultWorker := resolveWorkerFromProfile(resolvedProfile)
		rw, err := izanami.ResolveWorker("", workers, defaultWorker, func(format string, a ...interface{}) {
			fmt.Fprintf(cmd.OutOrStderr(), format, a...)
		})
		if err != nil {
			return err
		}
		resolved.WorkerURL = rw.URL
		resolved.WorkerName = rw.Name

		workerURL := resolved.GetWorkerURL()
		if workerURL == "" {
			fmt.Fprintln(cmd.OutOrStdout(), "No URL resolved (leader URL not configured)")
			return nil
		}

		switch rw.Source {
		case "standalone":
			fmt.Fprintf(cmd.OutOrStdout(), "Worker: standalone (using leader URL: %s)\n", workerURL)
		case "env-url":
			fmt.Fprintf(cmd.OutOrStdout(), "Worker: <direct> (%s) [source: IZ_WORKER_URL]\n", workerURL)
		default:
			fmt.Fprintf(cmd.OutOrStdout(), "Worker: %s (%s) [source: %s]\n", rw.Name, workerURL, rw.Source)
		}

		// Show credential source
		if len(rw.ClientKeys) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Credentials: per-worker (%s)\n", formatClientKeysCount(rw.ClientKeys))
		} else if len(resolved.ClientKeys) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Credentials: profile-level (%s)\n", formatClientKeysCount(resolved.ClientKeys))
		} else if resolved.ClientID != "" {
			fmt.Fprintln(cmd.OutOrStdout(), "Credentials: env/flags")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "Credentials: not configured")
		}

		return nil
	},
}

// profileWorkersShowCmd shows details for a specific worker
var profileWorkersShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show worker details",
	Long: `Show detailed configuration for a specific named worker.

Example:
  iz profiles workers show eu-west`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		showSecrets, _ := cmd.Flags().GetBool("show-secrets")

		profileName, err := izanami.GetActiveProfileName()
		if err != nil || profileName == "" {
			return fmt.Errorf("no active profile. Use 'iz profiles use <name>' first")
		}

		profile, err := izanami.GetProfile(profileName)
		if err != nil {
			return err
		}

		if profile.Workers == nil {
			return fmt.Errorf("no workers configured in profile '%s'", profileName)
		}

		worker, exists := profile.Workers[name]
		if !exists {
			available := make([]string, 0, len(profile.Workers))
			for n := range profile.Workers {
				available = append(available, n)
			}
			sort.Strings(available)
			return fmt.Errorf("worker '%s' not found. Available: %s", name, strings.Join(available, ", "))
		}

		isDefault := profile.DefaultWorker == name

		fmt.Fprintf(cmd.OutOrStdout(), "Worker: %s", name)
		if isDefault {
			fmt.Fprint(cmd.OutOrStdout(), " [default]")
		}
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintf(cmd.OutOrStdout(), "  URL:           %s\n", worker.URL)
		if len(worker.ClientKeys) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  Client Keys:   %s\n", formatClientKeysCount(worker.ClientKeys))
			printClientKeysSummary(cmd.OutOrStdout(), worker.ClientKeys, showSecrets)
		}

		return nil
	},
}

func init() {
	// Register workers under profiles
	profileCmd.AddCommand(profileWorkersCmd)

	profileWorkersCmd.AddCommand(profileWorkersAddCmd)
	profileWorkersCmd.AddCommand(profileWorkersDeleteCmd)
	profileWorkersCmd.AddCommand(profileWorkersListCmd)
	profileWorkersCmd.AddCommand(profileWorkersUseCmd)
	profileWorkersCmd.AddCommand(profileWorkersCurrentCmd)
	profileWorkersCmd.AddCommand(profileWorkersShowCmd)

	// Flags for workers add
	profileWorkersAddCmd.Flags().String("url", "", "Worker URL (required)")
	profileWorkersAddCmd.Flags().BoolVar(&workerAddForce, "force", false, "Overwrite existing worker")
	profileWorkersAddCmd.Flags().Bool("default", false, "Set as the default worker after creation")
	profileWorkersAddCmd.MarkFlagRequired("url")

	// Flags for workers delete
	profileWorkersDeleteCmd.Flags().BoolVarP(&workerDeleteForce, "force", "f", false, "Skip confirmation prompt")

	// Flags for workers show
	profileWorkersShowCmd.Flags().Bool("show-secrets", false, "Show sensitive values (client secrets)")

	// Dynamic completions
	profileWorkersUseCmd.ValidArgsFunction = completeWorkerNames
	profileWorkersDeleteCmd.ValidArgsFunction = completeWorkerNames
	profileWorkersShowCmd.ValidArgsFunction = completeWorkerNames
}
