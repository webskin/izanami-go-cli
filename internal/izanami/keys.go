package izanami

import (
	"context"
	"fmt"
	"net/http"

	errmsg "github.com/webskin/izanami-go-cli/internal/errors"
)

// ============================================================================
// API KEY OPERATIONS
// ============================================================================

// ListAPIKeys lists all API keys for a tenant and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseAPIKeys for typed structs.
func ListAPIKeys[T any](c *Client, ctx context.Context, tenant string, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.listAPIKeysRaw(ctx, tenant)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// listAPIKeysRaw fetches API keys and returns raw JSON bytes
func (c *Client) listAPIKeysRaw(ctx context.Context, tenant string) ([]byte, error) {
	path := apiAdminTenants + buildPath(tenant, "keys")

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)
	resp, err := req.Get(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToListAPIKeys, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}

// GetAPIKey retrieves a specific API key by clientID.
// Note: The Izanami API doesn't have a dedicated endpoint for getting a single key,
// so this method lists all keys and filters by clientID.
// Because of client-side filtering, this method returns a parsed key rather than raw JSON.
func (c *Client) GetAPIKey(ctx context.Context, tenant, clientID string) (*APIKey, error) {
	keys, err := ListAPIKeys(c, ctx, tenant, ParseAPIKeys)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToGetAPIKey, err)
	}

	// Find the key with matching clientID
	for i := range keys {
		if keys[i].ClientID == clientID {
			return &keys[i], nil
		}
	}

	return nil, &APIError{
		StatusCode: http.StatusNotFound,
		Message:    fmt.Sprintf("API key with clientID '%s' not found", clientID),
		RawBody:    "",
	}
}

// CreateAPIKey creates a new API key
// The key parameter accepts either an *APIKey or any compatible struct
func (c *Client) CreateAPIKey(ctx context.Context, tenant string, key interface{}) (*APIKey, error) {
	path := apiAdminTenants + buildPath(tenant, "keys")

	var result APIKey
	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(key).
		SetResult(&result)
	c.setAdminAuth(req)
	resp, err := req.Post(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToCreateAPIKey, err)
	}

	if resp.StatusCode() != http.StatusCreated && resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return &result, nil
}

// UpdateAPIKey updates an existing API key
// The key parameter accepts either an *APIKey or any compatible struct
func (c *Client) UpdateAPIKey(ctx context.Context, tenant, clientID string, key interface{}) error {
	path := apiAdminTenants + buildPath(tenant, "keys", clientID)

	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(key)
	c.setAdminAuth(req)
	resp, err := req.Put(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToUpdateAPIKey, err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}

// DeleteAPIKey deletes an API key
func (c *Client) DeleteAPIKey(ctx context.Context, tenant, clientID string) error {
	path := apiAdminTenants + buildPath(tenant, "keys", clientID)

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)
	resp, err := req.Delete(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToDeleteAPIKey, err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}

// ListAPIKeyUsers lists users with rights on an API key and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseKeyScopedUsers for typed structs.
func ListAPIKeyUsers[T any](c *Client, ctx context.Context, tenant, clientID string, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.listAPIKeyUsersRaw(ctx, tenant, clientID)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// listAPIKeyUsersRaw fetches users with rights on an API key and returns raw JSON bytes
func (c *Client) listAPIKeyUsersRaw(ctx context.Context, tenant, clientID string) ([]byte, error) {
	path := apiAdminTenants + buildPath(tenant, "keys", clientID, "users")

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)
	resp, err := req.Get(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToListAPIKeyUsers, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}
