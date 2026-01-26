package izanami

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	errmsg "github.com/webskin/izanami-go-cli/internal/errors"
)

// ============================================================================
// USER OPERATIONS
// ============================================================================

// ListUsers lists all visible users (global admin operation) and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseUserListItems for typed structs.
func ListUsers[T any](c *AdminClient, ctx context.Context, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.listUsersRaw(ctx)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// listUsersRaw fetches users and returns raw JSON bytes
func (c *AdminClient) listUsersRaw(ctx context.Context) ([]byte, error) {
	path := "/api/admin/users"

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)
	resp, err := req.Get(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToListUsers, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}

// GetUser retrieves a specific user with complete rights and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseUser for typed struct.
func GetUser[T any](c *AdminClient, ctx context.Context, username string, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.getUserRaw(ctx, username)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// getUserRaw fetches a user and returns raw JSON bytes
func (c *AdminClient) getUserRaw(ctx context.Context, username string) ([]byte, error) {
	path := "/api/admin/users/" + url.PathEscape(username)

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)
	resp, err := req.Get(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToGetUser, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}

// CreateUser creates a new user (global admin operation)
func (c *AdminClient) CreateUser(ctx context.Context, user interface{}) (*User, error) {
	path := "/api/admin/users"

	var result User
	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(user).
		SetResult(&result)
	c.setAdminAuth(req)
	resp, err := req.Post(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToCreateUser, err)
	}

	if resp.StatusCode() != http.StatusCreated && resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return &result, nil
}

// UpdateUser updates user information (username, email, defaultTenant)
func (c *AdminClient) UpdateUser(ctx context.Context, username string, updateReq interface{}) error {
	path := "/api/admin/users/" + url.PathEscape(username)

	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(updateReq)
	c.setAdminAuth(req)
	resp, err := req.Put(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToUpdateUser, err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}

// DeleteUser deletes a user (global admin operation)
func (c *AdminClient) DeleteUser(ctx context.Context, username string) error {
	path := "/api/admin/users/" + url.PathEscape(username)

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)
	resp, err := req.Delete(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToDeleteUser, err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}

// UpdateUserRights updates user's global rights (admin status and tenant rights)
func (c *AdminClient) UpdateUserRights(ctx context.Context, username string, rightsReq interface{}) error {
	path := "/api/admin/users/" + url.PathEscape(username) + "/rights"

	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(rightsReq)
	c.setAdminAuth(req)
	resp, err := req.Put(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToUpdateUserRights, err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}

// SearchUsers searches for users by username query
func (c *AdminClient) SearchUsers(ctx context.Context, query string, count int) ([]string, error) {
	path := "/api/admin/_search/users"

	var usernames []string
	req := c.http.R().
		SetContext(ctx).
		SetQueryParam("query", query).
		SetResult(&usernames)

	if count > 0 {
		req.SetQueryParam("count", strconv.Itoa(count))
	}

	c.setAdminAuth(req)
	resp, err := req.Get(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToSearchUsers, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return usernames, nil
}

// ListUsersForTenant lists all users with rights for a specific tenant
func (c *AdminClient) ListUsersForTenant(ctx context.Context, tenant string) ([]UserWithSingleLevelRight, error) {
	path := apiAdminTenants + buildPath(tenant, "users")

	var users []UserWithSingleLevelRight
	req := c.http.R().SetContext(ctx).SetResult(&users)
	c.setAdminAuth(req)
	resp, err := req.Get(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToListUsers, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return users, nil
}

// GetUserForTenant retrieves a specific user's rights for a tenant
func (c *AdminClient) GetUserForTenant(ctx context.Context, tenant, username string) (*User, error) {
	path := apiAdminTenants + buildPath(tenant, "users", username)

	var user User
	req := c.http.R().SetContext(ctx).SetResult(&user)
	c.setAdminAuth(req)
	resp, err := req.Get(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToGetUser, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return &user, nil
}

// UpdateUserTenantRights updates a user's rights for a specific tenant
func (c *AdminClient) UpdateUserTenantRights(ctx context.Context, tenant, username string, rightsReq interface{}) error {
	path := apiAdminTenants + buildPath(tenant, "users", username)

	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(rightsReq)
	c.setAdminAuth(req)
	resp, err := req.Put(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToUpdateTenantRights, err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}

// InviteUsersToTenant invites multiple users to a tenant with specified rights
func (c *AdminClient) InviteUsersToTenant(ctx context.Context, tenant string, invitations []UserInvitation) error {
	path := apiAdminTenants + buildPath(tenant, "users")

	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(invitations)
	c.setAdminAuth(req)
	resp, err := req.Post(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToInviteUsersToTenant, err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}

// ListUsersForProject lists all users with rights for a specific project
func (c *AdminClient) ListUsersForProject(ctx context.Context, tenant, project string) ([]ProjectScopedUser, error) {
	path := apiAdminTenants + buildPath(tenant, "projects", project, "users")

	var users []ProjectScopedUser
	req := c.http.R().SetContext(ctx).SetResult(&users)
	c.setAdminAuth(req)
	resp, err := req.Get(path)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToListUsers, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return users, nil
}

// UpdateUserProjectRights updates a user's rights for a specific project
func (c *AdminClient) UpdateUserProjectRights(ctx context.Context, tenant, project, username string, rightsReq interface{}) error {
	path := apiAdminTenants + buildPath(tenant, "projects", project, "users", username)

	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(rightsReq)
	c.setAdminAuth(req)
	resp, err := req.Put(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToUpdateProjectRights, err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}

// InviteUsersToProject invites multiple users to a project with specified rights
func (c *AdminClient) InviteUsersToProject(ctx context.Context, tenant, project string, invitations []UserInvitation) error {
	path := apiAdminTenants + buildPath(tenant, "projects", project, "users")

	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(invitations)
	c.setAdminAuth(req)
	resp, err := req.Post(path)

	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToInviteUsersToProject, err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}
