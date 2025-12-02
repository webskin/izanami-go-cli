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
