package izanami

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-resty/resty/v2"
	errmsg "github.com/webskin/izanami-go-cli/internal/errors"
)

// ============================================================================
// FEATURE CHECK OPERATIONS (Client API)
// ============================================================================

// CheckFeature checks if a feature is active (client API) and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseFeatureCheckResult for typed struct.
// Note: Feature check always uses CLIENT_ID/CLIENT_SECRET, not PAT token.
// If payload is provided, uses POST method for script features, otherwise uses GET.
func CheckFeature[T any](c *Client, ctx context.Context, featureID, user, contextPath, payload string, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.checkFeatureRaw(ctx, featureID, user, contextPath, payload)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// checkFeatureRaw fetches feature check result and returns raw JSON bytes
func (c *Client) checkFeatureRaw(ctx context.Context, featureID, user, contextPath, payload string) ([]byte, error) {
	path := "/api/v2/features/" + buildPath(featureID)

	req := c.http.R().SetContext(ctx)

	// Set client authentication (client-id/secret headers only)
	if err := c.setClientAuth(req); err != nil {
		return nil, err
	}

	if user != "" {
		req.SetQueryParam("user", user)
	}
	if contextPath != "" {
		req.SetQueryParam("context", normalizeContextPath(contextPath))
	}

	var resp *resty.Response
	var err error

	// Use POST if payload is provided, GET otherwise
	if payload != "" {
		req.SetHeader("Content-Type", "application/json").SetBody(payload)
		resp, err = req.Post(path)
	} else {
		resp, err = req.Get(path)
	}

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToCheckFeature, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}

// CheckFeatures checks activation for multiple features (bulk operation) and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseActivationsWithConditions for typed map.
// Supports filtering by feature IDs, projects, tags, and returns conditions if requested.
func CheckFeatures[T any](c *Client, ctx context.Context, request CheckFeaturesRequest, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.checkFeaturesRaw(ctx, request)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// checkFeaturesRaw fetches feature activations and returns raw JSON bytes
func (c *Client) checkFeaturesRaw(ctx context.Context, request CheckFeaturesRequest) ([]byte, error) {
	path := "/api/v2/features"

	req := c.http.R().SetContext(ctx)

	// Set client authentication
	if err := c.setClientAuth(req); err != nil {
		return nil, err
	}

	// Set query parameters
	if request.User != "" {
		req.SetQueryParam("user", request.User)
	}
	if request.Context != "" {
		req.SetQueryParam("context", normalizeContextPath(request.Context))
	}
	if len(request.Features) > 0 {
		req.SetQueryParam("features", strings.Join(request.Features, ","))
	}
	if len(request.Projects) > 0 {
		req.SetQueryParam("projects", strings.Join(request.Projects, ","))
	}
	if request.Conditions {
		req.SetQueryParam("conditions", "true")
	}
	if request.Date != "" {
		req.SetQueryParam("date", request.Date)
	}
	if len(request.OneTagIn) > 0 {
		req.SetQueryParam("oneTagIn", strings.Join(request.OneTagIn, ","))
	}
	if len(request.AllTagsIn) > 0 {
		req.SetQueryParam("allTagsIn", strings.Join(request.AllTagsIn, ","))
	}
	if len(request.NoTagIn) > 0 {
		req.SetQueryParam("noTagIn", strings.Join(request.NoTagIn, ","))
	}

	var resp *resty.Response
	var err error

	// Use POST if payload is provided, GET otherwise
	if request.Payload != "" {
		req.SetHeader("Content-Type", "application/json").SetBody(request.Payload)
		resp, err = req.Post(path)
	} else {
		resp, err = req.Get(path)
	}

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToCheckFeatures, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}
