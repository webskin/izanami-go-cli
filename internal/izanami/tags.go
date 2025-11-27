package izanami

import (
	"context"
	"fmt"
	"net/http"

	errmsg "github.com/webskin/izanami-go-cli/internal/errors"
)

// ============================================================================
// TAG OPERATIONS
// ============================================================================

// ListTags lists all tags in a tenant and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseTags for typed structs.
func ListTags[T any](c *Client, ctx context.Context, tenant string, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.listTagsRaw(ctx, tenant)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// listTagsRaw fetches tags and returns raw JSON bytes
func (c *Client) listTagsRaw(ctx context.Context, tenant string) ([]byte, error) {
	path := apiAdminTenants + buildPath(tenant, "tags")

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)
	resp, err := req.Get(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToListTags, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}

// GetTag retrieves a specific tag by name and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseTag for typed struct.
func GetTag[T any](c *Client, ctx context.Context, tenant, tagName string, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.getTagRaw(ctx, tenant, tagName)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// getTagRaw fetches a tag and returns raw JSON bytes
func (c *Client) getTagRaw(ctx context.Context, tenant, tagName string) ([]byte, error) {
	path := apiAdminTenants + buildPath(tenant, "tags", tagName)

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)
	resp, err := req.Get(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToGetTag, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}

// CreateTag creates a new tag
// The tag parameter accepts either a *Tag or any compatible struct
func (c *Client) CreateTag(ctx context.Context, tenant string, tag interface{}) error {
	path := apiAdminTenants + buildPath(tenant, "tags")

	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(tag)
	c.setAdminAuth(req)
	resp, err := req.Post(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToCreateTag, err)
	}

	if resp.StatusCode() != http.StatusCreated && resp.StatusCode() != http.StatusOK {
		return c.handleError(resp)
	}

	return nil
}

// DeleteTag deletes a tag
func (c *Client) DeleteTag(ctx context.Context, tenant, tagName string) error {
	path := apiAdminTenants + buildPath(tenant, "tags", tagName)

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)
	resp, err := req.Delete(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToDeleteTag, err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}
