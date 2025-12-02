package izanami

import (
	"context"
	"fmt"
	"net/http"

	errmsg "github.com/webskin/izanami-go-cli/internal/errors"
)

// ============================================================================
// FEATURE ADMIN OPERATIONS
// ============================================================================

// ListFeatures lists all features in a tenant and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseFeatures for typed structs.
func ListFeatures[T any](c *Client, ctx context.Context, tenant string, tag string, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.listFeaturesRaw(ctx, tenant, tag)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// listFeaturesRaw fetches features and returns raw JSON bytes
func (c *Client) listFeaturesRaw(ctx context.Context, tenant string, tag string) ([]byte, error) {
	path := apiAdminTenants + buildPath(tenant, "features")

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)

	// Add tag filter if specified (server-side filtering)
	if tag != "" {
		req.SetQueryParam("tag", tag)
	}

	resp, err := req.Get(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToListFeatures, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}

// GetFeature retrieves a specific feature and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseFeature for typed struct.
func GetFeature[T any](c *Client, ctx context.Context, tenant, featureID string, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.getFeatureRaw(ctx, tenant, featureID)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// getFeatureRaw fetches a feature and returns raw JSON bytes
func (c *Client) getFeatureRaw(ctx context.Context, tenant, featureID string) ([]byte, error) {
	path := apiAdminTenants + buildPath(tenant, "features", featureID)

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)
	resp, err := req.Get(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToGetFeature, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}

// CreateFeature creates a new feature
// The feature parameter accepts either a *Feature or any compatible struct
func (c *Client) CreateFeature(ctx context.Context, tenant, project string, feature interface{}) (*Feature, error) {
	path := apiAdminTenants + buildPath(tenant, "projects", project, "features")

	var result Feature
	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(feature).
		SetResult(&result)
	c.setAdminAuth(req)
	resp, err := req.Post(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToCreateFeature, err)
	}

	if resp.StatusCode() != http.StatusCreated && resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return &result, nil
}

// UpdateFeature updates an existing feature
// The feature parameter accepts either a *Feature, *FeatureWithOverloads, or any compatible struct
func (c *Client) UpdateFeature(ctx context.Context, tenant, featureID string, feature interface{}, preserveProtectedContexts bool) error {
	path := apiAdminTenants + buildPath(tenant, "features", featureID)

	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(feature)
	c.setAdminAuth(req)

	if preserveProtectedContexts {
		req.SetQueryParam("preserveProtectedContexts", "true")
	}

	resp, err := req.Put(path)
	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToUpdateFeature, err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}

// DeleteFeature deletes a feature
func (c *Client) DeleteFeature(ctx context.Context, tenant, featureID string) error {
	path := apiAdminTenants + buildPath(tenant, "features", featureID)

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)
	resp, err := req.Delete(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToDeleteFeature, err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent && resp.StatusCode() != http.StatusNotFound {
		return c.handleError(resp)
	}

	return nil
}

// PatchFeatures applies batch patches to multiple features
// Supports operations: replace (enabled, project, tags), remove (delete feature)
func (c *Client) PatchFeatures(ctx context.Context, tenant string, patches interface{}) error {
	path := apiAdminTenants + buildPath(tenant, "features")

	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(patches)
	c.setAdminAuth(req)

	resp, err := req.Patch(path)
	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToPatchFeatures, err)
	}

	if resp.StatusCode() != http.StatusNoContent && resp.StatusCode() != http.StatusOK {
		return c.handleError(resp)
	}

	return nil
}

// TestFeature tests an existing feature's evaluation and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseFeatureTestResult for typed struct.
// If contextPath is provided, tests feature with context-specific overrides.
func TestFeature[T any](c *Client, ctx context.Context, tenant, featureID, contextPath, user, date, payload string, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.testFeatureRaw(ctx, tenant, featureID, contextPath, user, date, payload)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// testFeatureRaw tests an existing feature and returns raw JSON bytes
func (c *Client) testFeatureRaw(ctx context.Context, tenant, featureID, contextPath, user, date, payload string) ([]byte, error) {
	// Build path: /api/admin/tenants/{tenant}/features/{id}/test[/{context}]
	var path string
	if contextPath != "" {
		// Remove leading slash from context for path building
		ctxPath := contextPath
		if len(ctxPath) > 0 && ctxPath[0] == '/' {
			ctxPath = ctxPath[1:]
		}
		path = apiAdminTenants + buildPath(tenant, "features", featureID, "test", ctxPath)
	} else {
		path = apiAdminTenants + buildPath(tenant, "features", featureID, "test")
	}

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)

	// Set query parameters
	if user != "" {
		req.SetQueryParam("user", user)
	}
	req.SetQueryParam("date", date) // date is required

	// Server requires Content-Type: application/json even without body
	req.SetHeader("Content-Type", "application/json")
	if payload != "" {
		req.SetBody(payload)
	} else {
		req.SetBody("{}")
	}

	resp, err := req.Post(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToTestFeature, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}

// TestFeatureDefinition tests a feature definition without saving and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseFeatureTestResult for typed struct.
func TestFeatureDefinition[T any](c *Client, ctx context.Context, tenant, user, date string, definition interface{}, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.testFeatureDefinitionRaw(ctx, tenant, user, date, definition)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// testFeatureDefinitionRaw tests a feature definition and returns raw JSON bytes
func (c *Client) testFeatureDefinitionRaw(ctx context.Context, tenant, user, date string, definition interface{}) ([]byte, error) {
	path := apiAdminTenants + buildPath(tenant, "test")

	// Server expects the definition wrapped in a "feature" key
	body := map[string]interface{}{
		"feature": definition,
	}

	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(body)
	c.setAdminAuth(req)

	// Set query parameters
	if user != "" {
		req.SetQueryParam("user", user)
	}
	req.SetQueryParam("date", date) // date is required

	resp, err := req.Post(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToTestFeatureDefinition, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}

// TestFeaturesBulk tests multiple features for a context and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseFeatureTestResults for typed map.
func TestFeaturesBulk[T any](c *Client, ctx context.Context, tenant string, request TestFeaturesAdminRequest, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.testFeaturesBulkRaw(ctx, tenant, request)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// testFeaturesBulkRaw tests multiple features and returns raw JSON bytes
func (c *Client) testFeaturesBulkRaw(ctx context.Context, tenant string, request TestFeaturesAdminRequest) ([]byte, error) {
	path := apiAdminTenants + buildPath(tenant, "features", "_test")

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)

	// Set query parameters
	if request.User != "" {
		req.SetQueryParam("user", request.User)
	}
	if request.Date != "" {
		req.SetQueryParam("date", request.Date)
	}
	if len(request.Features) > 0 {
		for _, f := range request.Features {
			req.QueryParam.Add("features", f)
		}
	}
	if len(request.Projects) > 0 {
		for _, p := range request.Projects {
			req.QueryParam.Add("projects", p)
		}
	}
	if request.Context != "" {
		req.SetQueryParam("context", normalizeContextPath(request.Context))
	}
	if len(request.OneTagIn) > 0 {
		for _, t := range request.OneTagIn {
			req.QueryParam.Add("oneTagIn", t)
		}
	}
	if len(request.AllTagsIn) > 0 {
		for _, t := range request.AllTagsIn {
			req.QueryParam.Add("allTagsIn", t)
		}
	}
	if len(request.NoTagIn) > 0 {
		for _, t := range request.NoTagIn {
			req.QueryParam.Add("noTagIn", t)
		}
	}

	resp, err := req.Get(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToTestFeaturesBulk, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}
