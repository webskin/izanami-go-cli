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

// ListUsers lists all visible users (global admin operation)
func (c *Client) ListUsers(ctx context.Context) ([]UserListItem, error) {
	path := "/api/admin/users"

	var users []UserListItem
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

// GetUser retrieves a specific user with complete rights
func (c *Client) GetUser(ctx context.Context, username string) (*User, error) {
	path := "/api/admin/users/" + url.PathEscape(username)

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

// CreateUser creates a new user (global admin operation)
func (c *Client) CreateUser(ctx context.Context, user interface{}) (*User, error) {
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
func (c *Client) UpdateUser(ctx context.Context, username string, updateReq interface{}) error {
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
func (c *Client) DeleteUser(ctx context.Context, username string) error {
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
func (c *Client) UpdateUserRights(ctx context.Context, username string, rightsReq interface{}) error {
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
func (c *Client) SearchUsers(ctx context.Context, query string, count int) ([]string, error) {
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
func (c *Client) ListUsersForTenant(ctx context.Context, tenant string) ([]UserWithSingleLevelRight, error) {
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
func (c *Client) GetUserForTenant(ctx context.Context, tenant, username string) (*User, error) {
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
func (c *Client) UpdateUserTenantRights(ctx context.Context, tenant, username string, rightsReq interface{}) error {
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
func (c *Client) InviteUsersToTenant(ctx context.Context, tenant string, invitations []UserInvitation) error {
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
func (c *Client) ListUsersForProject(ctx context.Context, tenant, project string) ([]ProjectScopedUser, error) {
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
func (c *Client) UpdateUserProjectRights(ctx context.Context, tenant, project, username string, rightsReq interface{}) error {
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
func (c *Client) InviteUsersToProject(ctx context.Context, tenant, project string, invitations []UserInvitation) error {
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
