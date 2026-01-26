package izanami

import (
	"context"
	"fmt"
	"net/http"

	errmsg "github.com/webskin/izanami-go-cli/internal/errors"
)

// ============================================================================
// UTILITY OPERATIONS
// ============================================================================

// Health checks the health status of Izanami and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseHealthStatus for typed struct.
func Health[T any](c *AdminClient, ctx context.Context, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.healthRaw(ctx)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// healthRaw fetches health status and returns raw JSON bytes
func (c *AdminClient) healthRaw(ctx context.Context) ([]byte, error) {
	resp, err := c.http.R().
		SetContext(ctx).
		Get("/api/_health")

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToCheckHealth, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}
