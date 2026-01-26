package izanami

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	errmsg "github.com/webskin/izanami-go-cli/internal/errors"
)

// ============================================================================
// OVERLOAD OPERATIONS
// ============================================================================

// SetOverload creates or updates a feature overload in a context
// PUT /api/admin/tenants/{tenant}/projects/{project}/contexts/{context}/features/{feature}
func (c *AdminClient) SetOverload(ctx context.Context, tenant, project, contextPath, featureName string, strategy interface{}, preserveProtected bool) error {
	// Build path: contexts path can contain slashes, feature name needs escaping
	path := apiAdminTenants + buildPath(tenant, "projects", project, "contexts") + "/" + contextPath + "/features/" + url.PathEscape(featureName)

	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(strategy)

	if preserveProtected {
		req.SetQueryParam("preserveProtectedContexts", "true")
	}

	c.setAdminAuth(req)
	resp, err := req.Put(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToSetOverload, err)
	}

	if resp.StatusCode() != http.StatusNoContent && resp.StatusCode() != http.StatusOK {
		return c.handleError(resp)
	}

	return nil
}

// DeleteOverload removes a feature overload from a context
// DELETE /api/admin/tenants/{tenant}/projects/{project}/contexts/{context}/features/{feature}
func (c *AdminClient) DeleteOverload(ctx context.Context, tenant, project, contextPath, featureName string, preserveProtected bool) error {
	// Build path: contexts path can contain slashes, feature name needs escaping
	path := apiAdminTenants + buildPath(tenant, "projects", project, "contexts") + "/" + contextPath + "/features/" + url.PathEscape(featureName)

	req := c.http.R().SetContext(ctx)

	if preserveProtected {
		req.SetQueryParam("preserveProtectedContexts", "true")
	}

	c.setAdminAuth(req)
	resp, err := req.Delete(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToDeleteOverload, err)
	}

	if resp.StatusCode() != http.StatusNoContent && resp.StatusCode() != http.StatusOK {
		return c.handleError(resp)
	}

	return nil
}

// GetOverload retrieves a feature's overload for a specific context and applies the given mapper.
// It fetches the context tree and extracts the overload for the specified context path.
// Use Identity mapper for raw JSON output, or a typed mapper for structured data.
// Parameters:
//   - tenant: the tenant name
//   - project: the project name (required to fetch project contexts)
//   - featureName: the feature name to look for in overloads
//   - contextPath: the context path (e.g., "PROD", "PROD/mobile")
func GetOverload[T any](c *AdminClient, ctx context.Context, tenant, project, featureName, contextPath string, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.getOverloadRaw(ctx, tenant, project, featureName, contextPath)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// getOverloadRaw fetches the context tree and extracts the overload for a specific context as raw JSON bytes
func (c *AdminClient) getOverloadRaw(ctx context.Context, tenant, project, featureName, contextPath string) ([]byte, error) {
	// Fetch the context tree for the project
	contextsRaw, err := c.listContextsRaw(ctx, tenant, project, true) // all=true to get nested contexts
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToGetOverload, err)
	}

	// Parse the context tree
	var contexts []Context
	if err := json.Unmarshal(contextsRaw, &contexts); err != nil {
		return nil, fmt.Errorf("%s: failed to parse context tree: %w", errmsg.MsgFailedToGetOverload, err)
	}

	// Find the context at the specified path and get the overload
	overload := findOverloadInContextTree(contexts, contextPath, featureName, "")
	if overload == nil {
		return nil, fmt.Errorf("%s: no overload found at context path '%s' for feature '%s'", errmsg.MsgFailedToGetOverload, contextPath, featureName)
	}

	// Convert the overload to JSON
	overloadBytes, err := json.Marshal(overload)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to serialize overload: %w", errmsg.MsgFailedToGetOverload, err)
	}

	return overloadBytes, nil
}

// findOverloadInContextTree recursively searches for an overload in the context tree
func findOverloadInContextTree(contexts []Context, targetPath, featureName, parentPath string) *FeatureOverload {
	for _, ctx := range contexts {
		// Build the full path for this context
		fullPath := ctx.Name
		if parentPath != "" {
			fullPath = parentPath + "/" + ctx.Name
		}

		// Check if this is the target context
		if fullPath == targetPath {
			// Look for the feature in this context's overloads
			for _, overload := range ctx.Overloads {
				if overload.Name == featureName {
					return &overload
				}
			}
			// Context found but no overload for this feature
			return nil
		}

		// Recursively search in children
		if ctx.Children != nil {
			if found := findOverloadInContextTree(contextsToSlice(ctx.Children), targetPath, featureName, fullPath); found != nil {
				return found
			}
		}
	}
	return nil
}

// contextsToSlice converts []*Context to []Context
func contextsToSlice(ptrs []*Context) []Context {
	result := make([]Context, len(ptrs))
	for i, ptr := range ptrs {
		if ptr != nil {
			result[i] = *ptr
		}
	}
	return result
}
