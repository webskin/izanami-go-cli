package izanami

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	errmsg "github.com/webskin/izanami-go-cli/internal/errors"
)

// ============================================================================
// UTILITY OPERATIONS
// ============================================================================

// Health checks the health status of Izanami and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseHealthStatus for typed struct.
func Health[T any](c *Client, ctx context.Context, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.healthRaw(ctx)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// healthRaw fetches health status and returns raw JSON bytes
func (c *Client) healthRaw(ctx context.Context) ([]byte, error) {
	resp, err := c.http.R().
		SetContext(ctx).
		Get("/api/_health")

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToCheckHealth, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}

// Export exports tenant data
// By default, exports all projects, keys, webhooks, and user rights
func (c *Client) Export(ctx context.Context, tenant string) (string, error) {
	path := apiAdminTenants + buildPath(tenant, "_export")

	// Default export request: export everything
	body := map[string]interface{}{
		"allProjects": true,
		"allKeys":     true,
		"allWebhooks": true,
		"userRights":  true,
	}

	req := c.http.R().
		SetContext(ctx).
		SetHeader("Accept", "application/x-ndjson").
		SetHeader("Content-Type", "application/json").
		SetBody(body)
	c.setAdminAuth(req)
	resp, err := req.Post(path)

	if err != nil {
		return "", fmt.Errorf("%s: %w", errmsg.MsgFailedToExport, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return "", c.handleError(resp)
	}

	return string(resp.Body()), nil
}

// Import imports tenant data from a file
// All fields from ImportRequest are passed as query parameters to the API
func (c *Client) Import(ctx context.Context, tenant, filePath string, req ImportRequest) (*ImportStatus, error) {
	path := apiAdminTenants + buildPath(tenant, "_import")

	var status ImportStatus

	httpReq := c.http.R().
		SetContext(ctx).
		SetFile("export", filePath).
		SetResult(&status)
	c.setAdminAuth(httpReq)

	// Set all query parameters from ImportRequest
	if req.Version > 0 {
		httpReq.SetQueryParam("version", strconv.Itoa(req.Version))
	}
	if req.Conflict != "" {
		httpReq.SetQueryParam("conflict", req.Conflict)
	}
	if req.Timezone != "" {
		httpReq.SetQueryParam("timezone", req.Timezone)
	}
	if req.DeduceProject {
		httpReq.SetQueryParam("deduceProject", "true")
	}
	if req.CreateProjects {
		httpReq.SetQueryParam("create", "true")
	}
	if req.Project != "" {
		httpReq.SetQueryParam("project", req.Project)
	}
	if req.ProjectPartSize > 0 {
		httpReq.SetQueryParam("projectPartSize", strconv.Itoa(req.ProjectPartSize))
	}
	if req.InlineScript {
		httpReq.SetQueryParam("inlineScript", "true")
	}

	resp, err := httpReq.Post(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToImport, err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusCreated {
		return nil, c.handleError(resp)
	}

	return &status, nil
}
