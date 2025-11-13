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
	eventsRaw    bool
	eventsPretty bool
)

// eventsCmd represents the events command
var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Watch feature flag events in real-time",
	Long: `Watch for feature flag changes and other events from Izanami using Server-Sent Events (SSE).

This command opens a persistent connection to Izanami and streams events as they occur.
Useful for monitoring feature flag changes in real-time.

Examples:
  # Watch all events
  iz events watch --client-id xxx --client-secret yyy

  # Watch events with pretty JSON formatting
  iz events watch --client-id xxx --client-secret yyy --pretty

  # Watch raw SSE format (shows event IDs and types)
  iz events watch --client-id xxx --client-secret yyy --raw`,
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

		fmt.Fprintf(os.Stderr, "🔄 Connecting to Izanami event stream...\n")
		fmt.Fprintf(os.Stderr, "   Press Ctrl+C to stop\n\n")

		startTime := time.Now()
		eventCount := 0

		err = client.WatchEvents(ctx, func(event izanami.Event) error {
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
}
