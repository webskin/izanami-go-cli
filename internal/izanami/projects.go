package izanami

import (
	"context"
	"fmt"
	"net/http"

	errmsg "github.com/webskin/izanami-go-cli/internal/errors"
)

// ============================================================================
// PROJECT OPERATIONS
// ============================================================================

// ListProjects lists all projects in a tenant and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseProjects for typed structs.
func ListProjects[T any](c *Client, ctx context.Context, tenant string, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.listProjectsRaw(ctx, tenant)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// listProjectsRaw fetches projects and returns raw JSON bytes
func (c *Client) listProjectsRaw(ctx context.Context, tenant string) ([]byte, error) {
	path := apiAdminTenants + buildPath(tenant, "projects")

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)
	resp, err := req.Get(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToListProjects, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}

// GetProject retrieves a specific project and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseProject for typed struct.
func GetProject[T any](c *Client, ctx context.Context, tenant, project string, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.getProjectRaw(ctx, tenant, project)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// getProjectRaw fetches a project and returns raw JSON bytes
func (c *Client) getProjectRaw(ctx context.Context, tenant, project string) ([]byte, error) {
	path := apiAdminTenants + buildPath(tenant, "projects", project)

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)
	resp, err := req.Get(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToGetProject, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}

// CreateProject creates a new project
// The project parameter accepts either a *Project or any compatible struct
func (c *Client) CreateProject(ctx context.Context, tenant string, project interface{}) error {
	path := apiAdminTenants + buildPath(tenant, "projects")

	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(project)
	c.setAdminAuth(req)
	resp, err := req.Post(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToCreateProject, err)
	}

	if resp.StatusCode() != http.StatusCreated && resp.StatusCode() != http.StatusOK {
		return c.handleError(resp)
	}

	return nil
}

// UpdateProject updates an existing project
// The project parameter accepts either a *Project or any compatible struct
func (c *Client) UpdateProject(ctx context.Context, tenant, name string, project interface{}) error {
	path := apiAdminTenants + buildPath(tenant, "projects", name)

	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(project)
	c.setAdminAuth(req)
	resp, err := req.Put(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToUpdateProject, err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}

// DeleteProject deletes a project
func (c *Client) DeleteProject(ctx context.Context, tenant, project string) error {
	path := apiAdminTenants + buildPath(tenant, "projects", project)

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)
	resp, err := req.Delete(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToDeleteProject, err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}

// ListProjectLogs retrieves event logs for a project and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseLogsResponse for typed structs.
func ListProjectLogs[T any](c *Client, ctx context.Context, tenant, project string, opts *LogsRequest, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.listProjectLogsRaw(ctx, tenant, project, opts)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// listProjectLogsRaw fetches project logs and returns raw JSON bytes
func (c *Client) listProjectLogsRaw(ctx context.Context, tenant, project string, opts *LogsRequest) ([]byte, error) {
	path := apiAdminTenants + buildPath(tenant, "projects", project, "logs")

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
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToListProjectLogs, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}
