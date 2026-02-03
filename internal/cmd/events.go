package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

var (
	eventsRaw             bool
	eventsPretty          bool
	eventsUser            string
	eventsContext         string
	eventsFeatures        []string
	eventsProjects        []string
	eventsConditions      bool
	eventsDate            string
	eventsOneTagIn        []string
	eventsAllTagsIn       []string
	eventsNoTagIn         []string
	eventsRefreshInterval int
	eventsKeepAlive       int
	eventsData            string
	// Client credentials for events
	eventsClientID     string
	eventsClientSecret string
	eventsWorker       string // Named worker selection
)

// eventsCmd represents the events command
var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Watch feature flag events in real-time",
	Long: `Watch for feature flag changes and other events from Izanami using Server-Sent Events (SSE).

This command opens a persistent connection to Izanami and streams events as they occur.
Useful for monitoring feature flag changes in real-time.

IMPORTANT: This endpoint requires client credentials (--client-id/--client-secret) or
credentials configured in your profile.

You can filter events by:
  - Specific features (--features) - feature names or UUIDs (names require --project)
  - Projects (--projects) - project names or UUIDs (names resolved via admin API)
  - Tags (--one-tag-in, --all-tags-in, --no-tag-in) - tag names or UUIDs
  - User/context (--user, --context)

Examples:
  # Watch all events (with explicit credentials)
  iz events watch --client-id xxx --client-secret yyy

  # Watch all events (using profile credentials for tenant)
  iz events watch --tenant my-tenant

  # Watch specific features (by name, requires --project)
  iz events watch --tenant my-tenant --project my-project --features my-feature

  # Watch specific features (by UUID)
  iz events watch --features 550e8400-e29b-41d4-a716-446655440000

  # Watch all features in a project (by name)
  iz events watch --tenant my-tenant --projects my-project

  # Watch all features in a project (by UUID)
  iz events watch --projects fc5eabbd-9f4d-47ff-9e29-2341275f53ad

  # Watch with context
  iz events watch --tenant my-tenant --projects my-project --context PROD

  # Watch with tag filtering (by name)
  iz events watch --tenant my-tenant --one-tag-in beta,experimental

  # Watch events with pretty JSON formatting
  iz events watch --pretty

  # Watch raw SSE format (shows event IDs and types)
  iz events watch --raw`,
}

// eventsWatchCmd watches for events in real-time
var eventsWatchCmd = &cobra.Command{
	Use:         "watch",
	Short:       "Watch for events in real-time",
	Annotations: map[string]string{"route": "GET /api/v2/events"},
	Long: `Opens a persistent Server-Sent Events (SSE) connection to Izanami
and displays events as they occur.

The connection will automatically reconnect if interrupted.
Press Ctrl+C to stop watching.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve worker and client credentials using same logic as features check
		var projects []string
		if cfg.Project != "" {
			projects = append(projects, cfg.Project)
		}

		// Re-resolve worker only if per-command --worker flag was explicitly set
		var workerClientKeys map[string]izanami.TenantClientKeysConfig
		if eventsWorker != "" {
			workers, defaultWorker := resolveWorkerFromProfile(activeProfile)
			rw, err := izanami.ResolveWorker(eventsWorker, workers, defaultWorker, func(format string, a ...interface{}) {
				fmt.Fprintf(cmd.OutOrStderr(), format, a...)
			})
			if err != nil {
				return err
			}
			cfg.WorkerURL = rw.URL
			cfg.WorkerName = rw.Name
			workerClientKeys = rw.ClientKeys
		}
		resolveClientCredentials(cmd, cfg, eventsClientID, eventsClientSecret, workerClientKeys, projects)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Check if we need admin client for name resolution
		needsAdminClient := false
		for _, f := range eventsFeatures {
			if !IsUUID(f) {
				needsAdminClient = true
				break
			}
		}
		if !needsAdminClient {
			for _, p := range eventsProjects {
				if !IsUUID(p) {
					needsAdminClient = true
					break
				}
			}
		}
		if !needsAdminClient {
			for _, t := range eventsOneTagIn {
				if !IsUUID(t) {
					needsAdminClient = true
					break
				}
			}
		}
		if !needsAdminClient {
			for _, t := range eventsAllTagsIn {
				if !IsUUID(t) {
					needsAdminClient = true
					break
				}
			}
		}
		if !needsAdminClient {
			for _, t := range eventsNoTagIn {
				if !IsUUID(t) {
					needsAdminClient = true
					break
				}
			}
		}

		var adminClient *izanami.AdminClient
		if needsAdminClient {
			var err error
			adminClient, err = izanami.NewAdminClient(cfg)
			if err != nil {
				return fmt.Errorf("failed to create admin client for name resolution: %w", err)
			}
		}

		// Handle Ctrl+C gracefully
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigCh
			fmt.Fprintln(cmd.OutOrStderr(), "\n\nStopping event stream...")
			cancel()
		}()

		// Use context from flag or config
		contextPath := eventsContext
		if contextPath == "" {
			contextPath = cfg.Context
		}

		// Parse payload if provided
		var payload string
		if eventsData != "" {
			var payloadData interface{}
			if err := parseJSONData(eventsData, &payloadData); err != nil {
				return fmt.Errorf("invalid JSON payload: %w", err)
			}
			payloadBytes, err := json.Marshal(payloadData)
			if err != nil {
				return fmt.Errorf("failed to serialize payload: %w", err)
			}
			payload = string(payloadBytes)
		}

		// Resolve project names to UUIDs if needed
		resolvedProjects, err := resolveProjectsToUUIDs(ctx, adminClient, cfg.Tenant, eventsProjects, cfg.Verbose, cmd)
		if err != nil {
			return err
		}

		// Resolve feature names to UUIDs if needed (requires project from config)
		resolvedFeatures, err := resolveFeaturesToUUIDs(ctx, adminClient, cfg.Tenant, cfg.Project, eventsFeatures, cfg.Verbose, cmd)
		if err != nil {
			return err
		}

		// Resolve tag names to UUIDs if needed
		resolvedOneTagIn, err := resolveTagsToUUIDs(ctx, adminClient, cfg.Tenant, eventsOneTagIn, cfg.Verbose, cmd)
		if err != nil {
			return err
		}
		resolvedAllTagsIn, err := resolveTagsToUUIDs(ctx, adminClient, cfg.Tenant, eventsAllTagsIn, cfg.Verbose, cmd)
		if err != nil {
			return err
		}
		resolvedNoTagIn, err := resolveTagsToUUIDs(ctx, adminClient, cfg.Tenant, eventsNoTagIn, cfg.Verbose, cmd)
		if err != nil {
			return err
		}

		// Create feature check client for event streaming (uses WorkerURL if set)
		checkClient, err := izanami.NewFeatureCheckClient(cfg)
		if err != nil {
			return err
		}

		// Build request
		request := izanami.EventsWatchRequest{
			User:              eventsUser,
			Context:           contextPath,
			Features:          resolvedFeatures,
			Projects:          resolvedProjects,
			Conditions:        eventsConditions,
			Date:              eventsDate,
			OneTagIn:          resolvedOneTagIn,
			AllTagsIn:         resolvedAllTagsIn,
			NoTagIn:           resolvedNoTagIn,
			RefreshInterval:   eventsRefreshInterval,
			KeepAliveInterval: eventsKeepAlive,
			Payload:           payload,
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Connecting to Izanami event stream...\n")
		fmt.Fprintf(cmd.OutOrStderr(), "   Press Ctrl+C to stop\n\n")

		startTime := time.Now()
		eventCount := 0

		err = checkClient.WatchEvents(ctx, request, func(event izanami.Event) error {
			eventCount++

			if eventsRaw {
				// Raw SSE format
				if event.ID != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "id: %s\n", event.ID)
				}
				if event.Type != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "event: %s\n", event.Type)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "data: %s\n\n", event.Data)
			} else {
				// Parse and display as JSON
				var data interface{}
				if err := json.Unmarshal([]byte(event.Data), &data); err != nil {
					// If not JSON, just print the raw data
					fmt.Fprintln(cmd.OutOrStdout(), event.Data)
					return nil
				}

				output := map[string]interface{}{
					"timestamp": time.Now().Format(time.RFC3339),
					"event":     event.Type,
					"data":      data,
				}

				if event.ID != "" {
					output["id"] = event.ID
				}

				encoder := json.NewEncoder(cmd.OutOrStdout())
				if eventsPretty {
					encoder.SetIndent("", "  ")
				}
				if err := encoder.Encode(output); err != nil {
					return fmt.Errorf("failed to encode event: %w", err)
				}
			}

			return nil
		})

		// Show statistics when stopping
		duration := time.Since(startTime)
		fmt.Fprintf(cmd.OutOrStderr(), "\nStatistics:\n")
		fmt.Fprintf(cmd.OutOrStderr(), "   Events received: %d\n", eventCount)
		fmt.Fprintf(cmd.OutOrStderr(), "   Duration: %s\n", duration.Round(time.Second))

		if err != nil && err != context.Canceled {
			return fmt.Errorf("event stream error: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(eventsCmd)
	eventsCmd.AddCommand(eventsWatchCmd)

	eventsWatchCmd.Flags().BoolVar(&eventsRaw, "raw", false, "Show raw SSE format (id, event, data)")
	eventsWatchCmd.Flags().BoolVar(&eventsPretty, "pretty", false, "Pretty-print JSON output (default: compact)")

	// Filtering flags
	eventsWatchCmd.Flags().StringVar(&eventsUser, "user", "", "User for feature evaluation (default: *)")
	eventsWatchCmd.Flags().StringVar(&eventsContext, "context", "", "Context path for evaluation")
	eventsWatchCmd.Flags().StringSliceVar(&eventsFeatures, "features", []string{}, "Feature names or UUIDs to watch (names require --project)")
	eventsWatchCmd.Flags().StringSliceVar(&eventsProjects, "projects", []string{}, "Project names or UUIDs to watch (comma-separated)")
	eventsWatchCmd.Flags().BoolVar(&eventsConditions, "conditions", false, "Include activation conditions in events")
	eventsWatchCmd.Flags().StringVar(&eventsDate, "date", "", "Date for evaluation (ISO 8601 format)")
	eventsWatchCmd.Flags().StringSliceVar(&eventsOneTagIn, "one-tag-in", []string{}, "Tag names or UUIDs - features must have at least one (comma-separated)")
	eventsWatchCmd.Flags().StringSliceVar(&eventsAllTagsIn, "all-tags-in", []string{}, "Tag names or UUIDs - features must have all (comma-separated)")
	eventsWatchCmd.Flags().StringSliceVar(&eventsNoTagIn, "no-tag-in", []string{}, "Tag names or UUIDs - features must not have any (comma-separated)")
	eventsWatchCmd.Flags().IntVar(&eventsRefreshInterval, "refresh-interval", 0, "Periodic refresh interval in seconds (0 = no periodic refresh)")
	eventsWatchCmd.Flags().IntVar(&eventsKeepAlive, "keep-alive-interval", 0, "Keep-alive interval in seconds (default: 25)")
	eventsWatchCmd.Flags().StringVar(&eventsData, "data", "", "JSON payload for script features (from file with @file.json, stdin with -, or inline)")

	// Client credentials
	eventsWatchCmd.Flags().StringVar(&eventsClientID, "client-id", "", "Client ID for feature/event API (env: IZ_CLIENT_ID)")
	eventsWatchCmd.Flags().StringVar(&eventsClientSecret, "client-secret", "", "Client secret for feature/event API (env: IZ_CLIENT_SECRET)")
	eventsWatchCmd.Flags().StringVar(&eventsWorker, "worker", "", "Named worker for event streaming (env: IZ_WORKER)")
	eventsWatchCmd.RegisterFlagCompletionFunc("worker", completeWorkerNames)
}
