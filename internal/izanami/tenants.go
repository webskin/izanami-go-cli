package izanami

import (
	"context"
	"fmt"
	"net/http"

	errmsg "github.com/webskin/izanami-go-cli/internal/errors"
)

// ============================================================================
// TENANT OPERATIONS
// ============================================================================

// ListTenants lists all tenants
// The right parameter filters tenants by minimum required permission level (Read, Write, or Admin)
func (c *Client) ListTenants(ctx context.Context, right *RightLevel) ([]Tenant, error) {
	req := c.http.R().
		SetContext(ctx).
		SetResult(&[]Tenant{})
	c.setAdminAuth(req)

	// Add right filter if specified
	if right != nil {
		req.SetQueryParam("right", right.String())
	}

	resp, err := req.Get("/api/admin/tenants")

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToListTenants, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	tenants := resp.Result().(*[]Tenant)
	return *tenants, nil
}

// GetTenant retrieves a specific tenant
func (c *Client) GetTenant(ctx context.Context, name string) (*Tenant, error) {
	path := apiAdminTenants + buildPath(name)

	var tenant Tenant
	req := c.http.R().SetContext(ctx).SetResult(&tenant)
	c.setAdminAuth(req)
	resp, err := req.Get(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToGetTenant, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return &tenant, nil
}

// CreateTenant creates a new tenant
// The tenant parameter accepts either a *Tenant or any compatible struct
func (c *Client) CreateTenant(ctx context.Context, tenant interface{}) error {
	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(tenant)
	c.setAdminAuth(req)
	resp, err := req.Post("/api/admin/tenants")

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToCreateTenant, err)
	}

	if resp.StatusCode() != http.StatusCreated && resp.StatusCode() != http.StatusOK {
		return c.handleError(resp)
	}

	return nil
}

// UpdateTenant updates an existing tenant
// The tenant parameter accepts either a *Tenant or any compatible struct
func (c *Client) UpdateTenant(ctx context.Context, name string, tenant interface{}) error {
	path := apiAdminTenants + buildPath(name)

	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(tenant)
	c.setAdminAuth(req)
	resp, err := req.Put(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToUpdateTenant, err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}

// DeleteTenant deletes a tenant
func (c *Client) DeleteTenant(ctx context.Context, name string) error {
	path := apiAdminTenants + buildPath(name)

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)
	resp, err := req.Delete(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToDeleteTenant, err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}
