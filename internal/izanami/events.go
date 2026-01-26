package izanami

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	errmsg "github.com/webskin/izanami-go-cli/internal/errors"
)

// ============================================================================
// EVENT STREAMING TYPES
// ============================================================================

// Event represents a Server-Sent Event from Izanami
type Event struct {
	ID   string
	Type string
	Data string
}

// EventCallback is called for each received event
type EventCallback func(event Event) error

// ============================================================================
// SSE PARSING HELPERS
// ============================================================================

// checkContextCancellation checks if context is cancelled
func checkContextCancellation(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// readSSELine reads and normalizes a line from the SSE stream
func readSSELine(ctx context.Context, reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		// Don't wrap EOF when context is cancelled
		if err.Error() == "EOF" && ctx.Err() != nil {
			return "", ctx.Err()
		}
		if err.Error() == "EOF" {
			return "", err
		}
		return "", fmt.Errorf("%s: %w", errmsg.MsgErrorReadingEventStream, err)
	}

	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	return line, nil
}

// parseSSEField parses a field:value pair and returns field and value
func parseSSEField(line string) (field, value string, ok bool) {
	// Comment, ignore
	if strings.HasPrefix(line, ":") {
		return "", "", false
	}

	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	field = parts[0]
	value = strings.TrimPrefix(parts[1], " ")
	return field, value, true
}

// updateEventField updates the event based on field type and returns retry delay if set
func updateEventField(event *Event, field, value string) time.Duration {
	switch field {
	case "id":
		event.ID = value
	case "event":
		event.Type = value
	case "data":
		if event.Data != "" {
			event.Data += "\n"
		}
		event.Data += value
	case "retry":
		// SSE retry field specifies reconnection delay in milliseconds
		if ms, err := strconv.ParseInt(value, 10, 64); err == nil && ms > 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return 0
}
