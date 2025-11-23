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

// ListProjects lists all projects in a tenant
func (c *Client) ListProjects(ctx context.Context, tenant string) ([]Project, error) {
	path := apiAdminTenants + buildPath(tenant, "projects")

	var projects []Project
	req := c.http.R().SetContext(ctx).SetResult(&projects)
	c.setAdminAuth(req)
	resp, err := req.Get(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToListProjects, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return projects, nil
}

// GetProject retrieves a specific project
func (c *Client) GetProject(ctx context.Context, tenant, project string) (*Project, error) {
	path := apiAdminTenants + buildPath(tenant, "projects", project)

	var proj Project
	req := c.http.R().SetContext(ctx).SetResult(&proj)
	c.setAdminAuth(req)
	resp, err := req.Get(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToGetProject, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return &proj, nil
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
