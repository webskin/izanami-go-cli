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
