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

// ListTenants lists all tenants and applies the given mapper to the response.
// Use Identity mapper for raw JSON output, or ParseTenants for typed structs.
func ListTenants[T any](c *AdminClient, ctx context.Context, right *RightLevel, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.listTenantsRaw(ctx, right)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// listTenantsRaw fetches tenants and returns raw JSON bytes
func (c *AdminClient) listTenantsRaw(ctx context.Context, right *RightLevel) ([]byte, error) {
	req := c.http.R().SetContext(ctx)
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

	return resp.Body(), nil
}

// GetTenant retrieves a specific tenant and applies the given mapper to the response.
// Use Identity mapper for raw JSON output, or ParseTenant for typed struct.
func GetTenant[T any](c *AdminClient, ctx context.Context, name string, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.getTenantRaw(ctx, name)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// getTenantRaw fetches a tenant and returns raw JSON bytes
func (c *AdminClient) getTenantRaw(ctx context.Context, name string) ([]byte, error) {
	path := apiAdminTenants + buildPath(name)

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)
	resp, err := req.Get(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToGetTenant, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}

// CreateTenant creates a new tenant
// The tenant parameter accepts either a *Tenant or any compatible struct
func (c *AdminClient) CreateTenant(ctx context.Context, tenant interface{}) error {
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
func (c *AdminClient) UpdateTenant(ctx context.Context, name string, tenant interface{}) error {
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
func (c *AdminClient) DeleteTenant(ctx context.Context, name string) error {
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

// ListTenantLogs retrieves event logs for a tenant and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseAuditEvents for typed structs.
func ListTenantLogs[T any](c *AdminClient, ctx context.Context, tenant string, opts *LogsRequest, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.listTenantLogsRaw(ctx, tenant, opts)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// listTenantLogsRaw fetches tenant logs and returns raw JSON bytes
func (c *AdminClient) listTenantLogsRaw(ctx context.Context, tenant string, opts *LogsRequest) ([]byte, error) {
	path := apiAdminTenants + buildPath(tenant, "logs")

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)

	// Apply query parameters if provided
	if opts != nil {
		if opts.Order != "" {
			req.SetQueryParam("order", opts.Order)
		}
		if opts.Users != "" {
			req.SetQueryParam("users", opts.Users)
		}
		if opts.Types != "" {
			req.SetQueryParam("types", opts.Types)
		}
		if opts.Features != "" {
			req.SetQueryParam("features", opts.Features)
		}
		if opts.Projects != "" {
			req.SetQueryParam("projects", opts.Projects)
		}
		if opts.Start != "" {
			req.SetQueryParam("start", opts.Start)
		}
		if opts.End != "" {
			req.SetQueryParam("end", opts.End)
		}
		if opts.Cursor != 0 {
			req.SetQueryParam("cursor", fmt.Sprintf("%d", opts.Cursor))
		}
		if opts.Count > 0 {
			req.SetQueryParam("count", fmt.Sprintf("%d", opts.Count))
		}
		if opts.Total {
			req.SetQueryParam("total", "true")
		}
	}

	resp, err := req.Get(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToListTenantLogs, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}
