package izanami

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	errmsg "github.com/webskin/izanami-go-cli/internal/errors"
)

// ============================================================================
// SEARCH OPERATIONS
// ============================================================================

// Search performs a global search and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseSearchResults for typed slice.
func Search[T any](c *Client, ctx context.Context, tenant, query string, filters []string, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.searchRaw(ctx, tenant, query, filters)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// searchRaw fetches search results and returns raw JSON bytes
func (c *Client) searchRaw(ctx context.Context, tenant, query string, filters []string) ([]byte, error) {
	var path string
	if tenant != "" {
		path = apiAdminTenants + buildPath(tenant, "search")
	} else {
		path = "/api/admin/search"
	}

	req := c.http.R().
		SetContext(ctx).
		SetQueryParam("query", query)
	c.setAdminAuth(req)

	if len(filters) > 0 {
		req.SetQueryParamsFromValues(url.Values{"filter": filters})
	}

	resp, err := req.Get(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToSearch, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}
