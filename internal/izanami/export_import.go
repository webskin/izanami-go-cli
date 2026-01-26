package izanami

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	errmsg "github.com/webskin/izanami-go-cli/internal/errors"
)

// ============================================================================
// EXPORT/IMPORT OPERATIONS
// ============================================================================

// Export exports tenant data
// By default, exports all projects, keys, webhooks, and user rights
func (c *AdminClient) Export(ctx context.Context, tenant string) (string, error) {
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

// ImportV2 imports tenant data from a V2 export file (synchronous)
// Returns messages on success, or messages+conflicts on conflict (HTTP 409)
func (c *AdminClient) ImportV2(ctx context.Context, tenant, filePath string, req ImportRequest) (*ImportV2Response, error) {
	path := apiAdminTenants + buildPath(tenant, "_import")

	httpReq := c.http.R().
		SetContext(ctx).
		SetFile("export", filePath).
		SetQueryParam("version", "2")
	c.setAdminAuth(httpReq)

	if req.Conflict != "" {
		httpReq.SetQueryParam("conflict", req.Conflict)
	}

	resp, err := httpReq.Post(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToImport, err)
	}

	var result ImportV2Response
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse import response: %w", err)
	}

	// HTTP 409 Conflict: return the conflicts along with messages
	if resp.StatusCode() == http.StatusConflict {
		return &result, &APIError{
			StatusCode: resp.StatusCode(),
			Message:    "import completed with conflicts",
			RawBody:    string(resp.Body()),
		}
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return &result, nil
}

// ImportV1 imports Izanami v1 data into v2 server (async migration)
// Returns an import job ID that can be polled with GetImportStatus
func (c *AdminClient) ImportV1(ctx context.Context, tenant, filePath string, req ImportRequest) (*ImportV1Response, error) {
	path := apiAdminTenants + buildPath(tenant, "_import")

	httpReq := c.http.R().
		SetContext(ctx).
		SetFile("export", filePath).
		SetQueryParam("version", "1")
	c.setAdminAuth(httpReq)

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

	if resp.StatusCode() != http.StatusAccepted {
		return nil, c.handleError(resp)
	}

	var result ImportV1Response
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse import response: %w", err)
	}

	return &result, nil
}

// GetImportStatus retrieves the status of an async V1 import operation
func (c *AdminClient) GetImportStatus(ctx context.Context, tenant, importID string) (*ImportV1Status, error) {
	path := apiAdminTenants + buildPath(tenant, "_import", importID)

	var status ImportV1Status

	req := c.http.R().
		SetContext(ctx).
		SetResult(&status)
	c.setAdminAuth(req)

	resp, err := req.Get(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get import status: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return &status, nil
}
