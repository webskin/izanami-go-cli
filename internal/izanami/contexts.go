package izanami

import (
	"context"
	"fmt"
	"net/http"

	errmsg "github.com/webskin/izanami-go-cli/internal/errors"
)

// ============================================================================
// CONTEXT OPERATIONS
// ============================================================================

// ListContexts lists all contexts for a tenant or project
func (c *Client) ListContexts(ctx context.Context, tenant, project string, all bool) ([]Context, error) {
	var path string
	if project != "" {
		path = apiAdminTenants + buildPath(tenant, "projects", project, "contexts")
	} else {
		path = apiAdminTenants + buildPath(tenant, "contexts")
	}

	req := c.http.R().SetContext(ctx).SetResult(&[]Context{})
	c.setAdminAuth(req)

	if all {
		req.SetQueryParam("all", "true")
	}

	resp, err := req.Get(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToListContexts, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	contexts := resp.Result().(*[]Context)
	return *contexts, nil
}

// CreateContext creates a new context
// The contextData parameter accepts a map or any compatible struct with context fields
func (c *Client) CreateContext(ctx context.Context, tenant, project, name, parentPath string, contextData interface{}) error {
	var path string
	if project != "" {
		if parentPath != "" {
			path = apiAdminTenants + buildPath(tenant, "projects", project, "contexts", parentPath)
		} else {
			path = apiAdminTenants + buildPath(tenant, "projects", project, "contexts")
		}
	} else {
		if parentPath != "" {
			path = apiAdminTenants + buildPath(tenant, "contexts", parentPath)
		} else {
			path = apiAdminTenants + buildPath(tenant, "contexts")
		}
	}

	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(contextData)
	c.setAdminAuth(req)
	resp, err := req.Post(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToCreateContext, err)
	}

	if resp.StatusCode() != http.StatusCreated && resp.StatusCode() != http.StatusOK {
		return c.handleError(resp)
	}

	return nil
}

// DeleteContext deletes a context
func (c *Client) DeleteContext(ctx context.Context, tenant, project, contextPath string) error {
	var path string
	if project != "" {
		path = apiAdminTenants + buildPath(tenant, "projects", project, "contexts", contextPath)
	} else {
		path = apiAdminTenants + buildPath(tenant, "contexts", contextPath)
	}

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)
	resp, err := req.Delete(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToDeleteContext, err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}
