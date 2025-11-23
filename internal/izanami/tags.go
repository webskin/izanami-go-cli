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

// ListTags lists all tags in a tenant
func (c *Client) ListTags(ctx context.Context, tenant string) ([]Tag, error) {
	path := apiAdminTenants + buildPath(tenant, "tags")

	var tags []Tag
	req := c.http.R().SetContext(ctx).SetResult(&tags)
	c.setAdminAuth(req)
	resp, err := req.Get(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToListTags, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return tags, nil
}

// GetTag retrieves a specific tag by name using the dedicated endpoint
// GET /api/admin/tenants/:tenant/tags/:name
func (c *Client) GetTag(ctx context.Context, tenant, tagName string) (*Tag, error) {
	path := apiAdminTenants + buildPath(tenant, "tags", tagName)

	var tag Tag
	req := c.http.R().SetContext(ctx).SetResult(&tag)
	c.setAdminAuth(req)
	resp, err := req.Get(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToGetTag, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return &tag, nil
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
