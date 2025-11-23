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
)

// eventsCmd represents the events command
var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Watch feature flag events in real-time",
	Long: `Watch for feature flag changes and other events from Izanami using Server-Sent Events (SSE).

This command opens a persistent connection to Izanami and streams events as they occur.
Useful for monitoring feature flag changes in real-time.

You can filter events by:
  - Specific features (--features)
  - Projects (--projects)
  - Tags (--one-tag-in, --all-tags-in, --no-tag-in)
  - User/context (--user, --context)

Examples:
  # Watch all events
  iz events watch --client-id xxx --client-secret yyy

  # Watch specific features
  iz events watch --features feat1-uuid,feat2-uuid

  # Watch all features in specific projects
  iz events watch --projects proj1-uuid,proj2-uuid

  # Watch with tag filtering
  iz events watch --projects proj1-uuid --one-tag-in beta,experimental

  # Watch events with pretty JSON formatting
  iz events watch --pretty

  # Watch raw SSE format (shows event IDs and types)
  iz events watch --raw`,
}

// eventsWatchCmd watches for events in real-time
var eventsWatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch for events in real-time",
	Long: `Opens a persistent Server-Sent Events (SSE) connection to Izanami
and displays events as they occur.

The connection will automatically reconnect if interrupted.
Press Ctrl+C to stop watching.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve client credentials
		if eventsClientID != "" {
			cfg.ClientID = eventsClientID
		}
		if eventsClientSecret != "" {
			cfg.ClientSecret = eventsClientSecret
		}

		if cfg.ClientID == "" && cfg.ClientSecret == "" {
			tenant := cfg.Tenant
			var projects []string
			if cfg.Project != "" {
				projects = append(projects, cfg.Project)
			}

			clientID, clientSecret := cfg.ResolveClientCredentials(tenant, projects)
			if clientID != "" && clientSecret != "" {
				cfg.ClientID = clientID
				cfg.ClientSecret = clientSecret
				if cfg.Verbose {
					if len(projects) > 0 {
						fmt.Fprintf(os.Stderr, "Using client credentials from config (tenant: %s, projects: %v)\n", tenant, projects)
					} else {
						fmt.Fprintf(os.Stderr, "Using client credentials from config (tenant: %s)\n", tenant)
					}
				}
			}
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle Ctrl+C gracefully
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigCh
			fmt.Fprintln(os.Stderr, "\n\nStopping event stream...")
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

		// Build request
		request := izanami.EventsWatchRequest{
			User:              eventsUser,
			Context:           contextPath,
			Features:          eventsFeatures,
			Projects:          eventsProjects,
			Conditions:        eventsConditions,
			Date:              eventsDate,
			OneTagIn:          eventsOneTagIn,
			AllTagsIn:         eventsAllTagsIn,
			NoTagIn:           eventsNoTagIn,
			RefreshInterval:   eventsRefreshInterval,
			KeepAliveInterval: eventsKeepAlive,
			Payload:           payload,
		}

		fmt.Fprintf(os.Stderr, "🔄 Connecting to Izanami event stream...\n")
		fmt.Fprintf(os.Stderr, "   Press Ctrl+C to stop\n\n")

		startTime := time.Now()
		eventCount := 0

		err = client.WatchEvents(ctx, request, func(event izanami.Event) error {
			eventCount++

			if eventsRaw {
				// Raw SSE format
				if event.ID != "" {
					fmt.Fprintf(os.Stdout, "id: %s\n", event.ID)
				}
				if event.Type != "" {
					fmt.Fprintf(os.Stdout, "event: %s\n", event.Type)
				}
				fmt.Fprintf(os.Stdout, "data: %s\n\n", event.Data)
			} else {
				// Parse and display as JSON
				var data interface{}
				if err := json.Unmarshal([]byte(event.Data), &data); err != nil {
					// If not JSON, just print the raw data
					fmt.Fprintln(os.Stdout, event.Data)
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

				encoder := json.NewEncoder(os.Stdout)
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
		fmt.Fprintf(os.Stderr, "\n📊 Statistics:\n")
		fmt.Fprintf(os.Stderr, "   Events received: %d\n", eventCount)
		fmt.Fprintf(os.Stderr, "   Duration: %s\n", duration.Round(time.Second))

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
	eventsWatchCmd.Flags().StringSliceVar(&eventsFeatures, "features", []string{}, "Feature IDs to watch (comma-separated)")
	eventsWatchCmd.Flags().StringSliceVar(&eventsProjects, "projects", []string{}, "Project IDs to watch (comma-separated)")
	eventsWatchCmd.Flags().BoolVar(&eventsConditions, "conditions", false, "Include activation conditions in events")
	eventsWatchCmd.Flags().StringVar(&eventsDate, "date", "", "Date for evaluation (ISO 8601 format)")
	eventsWatchCmd.Flags().StringSliceVar(&eventsOneTagIn, "one-tag-in", []string{}, "Features must have at least one of these tags (comma-separated)")
	eventsWatchCmd.Flags().StringSliceVar(&eventsAllTagsIn, "all-tags-in", []string{}, "Features must have all of these tags (comma-separated)")
	eventsWatchCmd.Flags().StringSliceVar(&eventsNoTagIn, "no-tag-in", []string{}, "Features must not have any of these tags (comma-separated)")
	eventsWatchCmd.Flags().IntVar(&eventsRefreshInterval, "refresh-interval", 0, "Periodic refresh interval in seconds (0 = no periodic refresh)")
	eventsWatchCmd.Flags().IntVar(&eventsKeepAlive, "keep-alive-interval", 0, "Keep-alive interval in seconds (default: 25)")
	eventsWatchCmd.Flags().StringVar(&eventsData, "data", "", "JSON payload for script features (from file with @file.json, stdin with -, or inline)")

	// Client credentials
	eventsWatchCmd.Flags().StringVar(&eventsClientID, "client-id", "", "Client ID for authentication (env: IZ_CLIENT_ID)")
	eventsWatchCmd.Flags().StringVar(&eventsClientSecret, "client-secret", "", "Client secret for authentication (env: IZ_CLIENT_SECRET)")
}
