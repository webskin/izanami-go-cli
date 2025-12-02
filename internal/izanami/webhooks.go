package izanami

import (
	"context"
	"fmt"
	"net/http"

	errmsg "github.com/webskin/izanami-go-cli/internal/errors"
)

// ============================================================================
// WEBHOOK OPERATIONS
// ============================================================================

// ListWebhooks lists all webhooks in a tenant and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseWebhooks for typed structs.
func ListWebhooks[T any](c *Client, ctx context.Context, tenant string, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.listWebhooksRaw(ctx, tenant)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// listWebhooksRaw fetches webhooks and returns raw JSON bytes
func (c *Client) listWebhooksRaw(ctx context.Context, tenant string) ([]byte, error) {
	path := apiAdminTenants + buildPath(tenant, "webhooks")

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)
	resp, err := req.Get(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToListWebhooks, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}

// Note: Izanami API doesn't have a GET endpoint for single webhooks.
// Use ListWebhooks and filter by ID instead.

// CreateWebhook creates a new webhook
// The webhook parameter accepts either a *Webhook or any compatible struct
func (c *Client) CreateWebhook(ctx context.Context, tenant string, webhook interface{}) (*WebhookFull, error) {
	path := apiAdminTenants + buildPath(tenant, "webhooks")

	var result WebhookFull
	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(webhook).
		SetResult(&result)
	c.setAdminAuth(req)
	resp, err := req.Post(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToCreateWebhook, err)
	}

	if resp.StatusCode() != http.StatusCreated && resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return &result, nil
}

// UpdateWebhook updates an existing webhook
// The webhook parameter accepts either a *Webhook or any compatible struct
func (c *Client) UpdateWebhook(ctx context.Context, tenant, webhookID string, webhook interface{}) error {
	path := apiAdminTenants + buildPath(tenant, "webhooks", webhookID)

	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(webhook)
	c.setAdminAuth(req)
	resp, err := req.Put(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToUpdateWebhook, err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}

// DeleteWebhook deletes a webhook
func (c *Client) DeleteWebhook(ctx context.Context, tenant, webhookID string) error {
	path := apiAdminTenants + buildPath(tenant, "webhooks", webhookID)

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)
	resp, err := req.Delete(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToDeleteWebhook, err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}

// ListWebhookUsers lists users with rights on a webhook and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseWebhookUsers for typed structs.
func ListWebhookUsers[T any](c *Client, ctx context.Context, tenant, webhookID string, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.listWebhookUsersRaw(ctx, tenant, webhookID)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// listWebhookUsersRaw fetches users with rights on a webhook and returns raw JSON bytes
func (c *Client) listWebhookUsersRaw(ctx context.Context, tenant, webhookID string) ([]byte, error) {
	path := apiAdminTenants + buildPath(tenant, "webhooks", webhookID, "users")

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)
	resp, err := req.Get(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToListWebhookUsers, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}
