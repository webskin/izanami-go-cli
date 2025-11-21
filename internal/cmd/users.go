package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/errors"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"github.com/webskin/izanami-go-cli/internal/output"
)

var (
	userName           string
	userEmail          string
	userPassword       string
	userAdmin          bool
	userType           string
	userDefaultTenant  string
	userRightsFile     string
	userTenantRight    string
	userProjectRight   string
	usersDeleteForce   bool
	usersInviteFile    string
	usersSearchCount   int
)

// usersCmd represents the users command
var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "Manage users",
	Long: `Manage users and their rights in Izanami.

Users can have global admin rights or tenant/project-specific rights.
This command supports both global operations and tenant/project-scoped operations.

Examples:
  # List all users (global)
  iz admin users list

  # List users for a specific tenant
  iz admin users list-for-tenant --tenant my-tenant

  # Get user details
  iz admin users get johndoe

  # Create a new user
  iz admin users create johndoe --email john@example.com --password secret123 --admin

  # Update user information
  iz admin users update johndoe --email newemail@example.com

  # Delete a user
  iz admin users delete johndoe`,
}

// usersListCmd lists all users (global)
var usersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all visible users (global)",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		users, err := client.ListUsers(ctx)
		if err != nil {
			return err
		}

		if len(users) == 0 {
			fmt.Fprintln(os.Stderr, "No users found")
			return nil
		}

		return output.Print(users, output.Format(outputFormat))
	},
}

// usersGetCmd gets a specific user
var usersGetCmd = &cobra.Command{
	Use:   "get <username>",
	Short: "Get details of a specific user with complete rights",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		username := args[0]

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		user, err := client.GetUser(ctx, username)
		if err != nil {
			return err
		}

		// For table output, use custom fancy display
		if output.Format(outputFormat) == output.Table {
			return printUserDetails(user)
		}

		// For JSON output, print as-is
		return output.Print(user, output.Format(outputFormat))
	},
}

// usersCreateCmd creates a new user
var usersCreateCmd = &cobra.Command{
	Use:   "create <username>",
	Short: "Create a new user",
	Args:  cobra.ExactArgs(1),
	Long: `Create a new user with specified properties.

You can provide user rights either via:
  1. Individual flags (--admin, --tenant-right, --project-right)
  2. A JSON file with --rights-file

Examples:
  # Create an admin user
  iz admin users create johndoe --email john@example.com --password secret123 --admin

  # Create a user with tenant rights
  iz admin users create janedoe --email jane@example.com --password secret123 \
    --tenant-right "tenant1:Admin" --tenant-right "tenant2:Write"

  # Create a user from JSON file
  iz admin users create bob --email bob@example.com --password secret123 --rights-file user-rights.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		username := args[0]

		if userEmail == "" {
			return fmt.Errorf("email is required (use --email flag)")
		}
		if userPassword == "" {
			return fmt.Errorf("password is required (use --password flag)")
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		userData := map[string]interface{}{
			"username": username,
			"email":    userEmail,
			"password": userPassword,
			"admin":    userAdmin,
		}

		if userType != "" {
			userData["userType"] = userType
		}

		if userDefaultTenant != "" {
			userData["defaultTenant"] = userDefaultTenant
		}

		// Handle rights from file or flags
		if userRightsFile != "" {
			rightsData, err := os.ReadFile(userRightsFile)
			if err != nil {
				return fmt.Errorf("failed to read rights file: %w", err)
			}

			var rights map[string]interface{}
			if err := json.Unmarshal(rightsData, &rights); err != nil {
				return fmt.Errorf("failed to parse rights file: %w", err)
			}

			userData["rights"] = rights
		} else if userTenantRight != "" || userProjectRight != "" {
			rights := make(map[string]interface{})
			tenants := make(map[string]interface{})

			// Parse tenant rights (format: "tenant:level")
			if userTenantRight != "" {
				parts := strings.Split(userTenantRight, ":")
				if len(parts) != 2 {
					return fmt.Errorf("invalid tenant-right format (expected 'tenant:level')")
				}
				tenants[parts[0]] = map[string]interface{}{
					"level": parts[1],
				}
			}

			// Parse project rights (format: "tenant:project:level")
			if userProjectRight != "" {
				parts := strings.Split(userProjectRight, ":")
				if len(parts) != 3 {
					return fmt.Errorf("invalid project-right format (expected 'tenant:project:level')")
				}
				tenant := parts[0]
				project := parts[1]
				level := parts[2]

				if _, exists := tenants[tenant]; !exists {
					tenants[tenant] = map[string]interface{}{
						"level":    "Read",
						"projects": make(map[string]interface{}),
					}
				}

				tenantData := tenants[tenant].(map[string]interface{})
				if _, exists := tenantData["projects"]; !exists {
					tenantData["projects"] = make(map[string]interface{})
				}
				projects := tenantData["projects"].(map[string]interface{})
				projects[project] = map[string]interface{}{
					"level": level,
				}
			}

			if len(tenants) > 0 {
				rights["tenants"] = tenants
				userData["rights"] = rights
			}
		}

		ctx := context.Background()
		result, err := client.CreateUser(ctx, userData)
		if err != nil {
			return err
		}

		if output.Format(outputFormat) == output.JSON {
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(result)
		}

		fmt.Fprintf(os.Stderr, "✅ User created successfully\n\n")
		fmt.Fprintf(os.Stderr, "Username: %s\n", result.Username)
		fmt.Fprintf(os.Stderr, "Email:    %s\n", result.Email)
		fmt.Fprintf(os.Stderr, "Admin:    %t\n", result.Admin)
		fmt.Fprintf(os.Stderr, "Type:     %s\n", result.UserType)
		if result.DefaultTenant != nil && *result.DefaultTenant != "" {
			fmt.Fprintf(os.Stderr, "Default Tenant: %s\n", *result.DefaultTenant)
		}

		return nil
	},
}

// usersUpdateCmd updates user information
var usersUpdateCmd = &cobra.Command{
	Use:   "update <username>",
	Short: "Update user information (email, default tenant)",
	Args:  cobra.ExactArgs(1),
	Long: `Update user information such as email and default tenant.

Note: This command updates basic user information only.
To update user rights, use 'update-rights' command.
Password must be provided for authentication.

Examples:
  iz admin users update johndoe --email newemail@example.com --password current123`,
	RunE: func(cmd *cobra.Command, args []string) error {
		username := args[0]

		if userPassword == "" {
			return fmt.Errorf("password is required for authentication (use --password flag)")
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		updateData := map[string]interface{}{
			"username": username,
			"password": userPassword,
		}

		if userEmail != "" {
			updateData["email"] = userEmail
		}

		if userDefaultTenant != "" {
			updateData["defaultTenant"] = userDefaultTenant
		}

		ctx := context.Background()
		if err := client.UpdateUser(ctx, username, updateData); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "✅ User updated successfully\n")
		return nil
	},
}

// usersDeleteCmd deletes a user
var usersDeleteCmd = &cobra.Command{
	Use:   "delete <username>",
	Short: "Delete a user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		username := args[0]

		// Confirm deletion unless --force is used
		if !usersDeleteForce {
			if !confirmDeletion(cmd, "user", username) {
				return nil
			}
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.DeleteUser(ctx, username); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "✅ User deleted successfully\n")
		return nil
	},
}

// usersUpdateRightsCmd updates user's global rights
var usersUpdateRightsCmd = &cobra.Command{
	Use:   "update-rights <username>",
	Short: "Update user's global rights (admin status and tenant rights)",
	Args:  cobra.ExactArgs(1),
	Long: `Update user's global rights including admin status and tenant rights.

Rights can be provided via:
  1. A JSON file with --rights-file
  2. Individual flags (--admin, --tenant-right)

Examples:
  # Make user a global admin
  iz admin users update-rights johndoe --admin

  # Update rights from JSON file
  iz admin users update-rights johndoe --rights-file rights.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		username := args[0]

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		rightsData := make(map[string]interface{})

		if userRightsFile != "" {
			fileData, err := os.ReadFile(userRightsFile)
			if err != nil {
				return fmt.Errorf("failed to read rights file: %w", err)
			}

			if err := json.Unmarshal(fileData, &rightsData); err != nil {
				return fmt.Errorf("failed to parse rights file: %w", err)
			}
		} else {
			// Handle flags
			if cmd.Flags().Changed("admin") {
				rightsData["admin"] = userAdmin
			}

			if userTenantRight != "" {
				rights := make(map[string]interface{})
				tenants := make(map[string]interface{})

				parts := strings.Split(userTenantRight, ":")
				if len(parts) != 2 {
					return fmt.Errorf("invalid tenant-right format (expected 'tenant:level')")
				}
				tenants[parts[0]] = map[string]interface{}{
					"level": parts[1],
				}

				rights["tenants"] = tenants
				rightsData["rights"] = rights
			}
		}

		if len(rightsData) == 0 {
			return fmt.Errorf("no rights specified (use --admin, --tenant-right, or --rights-file)")
		}

		ctx := context.Background()
		if err := client.UpdateUserRights(ctx, username, rightsData); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "✅ User rights updated successfully\n")
		return nil
	},
}

// usersSearchCmd searches for users
var usersSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search for users by username",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		usernames, err := client.SearchUsers(ctx, query, usersSearchCount)
		if err != nil {
			return err
		}

		if len(usernames) == 0 {
			fmt.Fprintln(os.Stderr, "No users found")
			return nil
		}

		return output.Print(usernames, output.Format(outputFormat))
	},
}

// ============================================================================
// TENANT-SCOPED OPERATIONS
// ============================================================================

// usersListForTenantCmd lists users for a specific tenant
var usersListForTenantCmd = &cobra.Command{
	Use:   "list-for-tenant",
	Short: "List all users with rights for a specific tenant",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.Tenant == "" {
			return fmt.Errorf(errors.MsgTenantRequired)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		users, err := client.ListUsersForTenant(ctx, cfg.Tenant)
		if err != nil {
			return err
		}

		if len(users) == 0 {
			fmt.Fprintln(os.Stderr, "No users found for this tenant")
			return nil
		}

		return output.Print(users, output.Format(outputFormat))
	},
}

// usersGetForTenantCmd gets a user's rights for a specific tenant
var usersGetForTenantCmd = &cobra.Command{
	Use:   "get-for-tenant <username>",
	Short: "Get user's rights for a specific tenant",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		username := args[0]

		if cfg.Tenant == "" {
			return fmt.Errorf(errors.MsgTenantRequired)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		user, err := client.GetUserForTenant(ctx, cfg.Tenant, username)
		if err != nil {
			return err
		}

		return output.Print(user, output.Format(outputFormat))
	},
}

// usersUpdateTenantRightsCmd updates user's rights for a specific tenant
var usersUpdateTenantRightsCmd = &cobra.Command{
	Use:   "update-tenant-rights <username>",
	Short: "Update user's rights for a specific tenant",
	Args:  cobra.ExactArgs(1),
	Long: `Update user's rights for a specific tenant.

You can provide rights via:
  1. A JSON file with --rights-file
  2. Individual flag --tenant-right (format: "level")

To remove all rights for the tenant, send an empty JSON object.

Examples:
  iz admin users update-tenant-rights johndoe --tenant my-tenant --tenant-right "Admin"
  iz admin users update-tenant-rights johndoe --tenant my-tenant --rights-file tenant-rights.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		username := args[0]

		if cfg.Tenant == "" {
			return fmt.Errorf(errors.MsgTenantRequired)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		var rightsData interface{}

		if userRightsFile != "" {
			fileData, err := os.ReadFile(userRightsFile)
			if err != nil {
				return fmt.Errorf("failed to read rights file: %w", err)
			}

			if err := json.Unmarshal(fileData, &rightsData); err != nil {
				return fmt.Errorf("failed to parse rights file: %w", err)
			}
		} else if userTenantRight != "" {
			rightsData = map[string]interface{}{
				"level": userTenantRight,
			}
		} else {
			return fmt.Errorf("no rights specified (use --tenant-right or --rights-file)")
		}

		ctx := context.Background()
		if err := client.UpdateUserTenantRights(ctx, cfg.Tenant, username, rightsData); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "✅ User tenant rights updated successfully\n")
		return nil
	},
}

// usersInviteToTenantCmd invites multiple users to a tenant
var usersInviteToTenantCmd = &cobra.Command{
	Use:   "invite-to-tenant",
	Short: "Invite multiple users to a tenant with specified rights",
	Long: `Invite multiple users to a tenant (bulk operation).

Provide invitations via JSON file with --invite-file.
The file should contain an array of objects with "username" and "level" fields.

Example JSON file:
[
  {"username": "user1", "level": "Read"},
  {"username": "user2", "level": "Admin"}
]

Example:
  iz admin users invite-to-tenant --tenant my-tenant --invite-file invitations.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.Tenant == "" {
			return fmt.Errorf(errors.MsgTenantRequired)
		}

		if usersInviteFile == "" {
			return fmt.Errorf("invite file is required (use --invite-file flag)")
		}

		fileData, err := os.ReadFile(usersInviteFile)
		if err != nil {
			return fmt.Errorf("failed to read invite file: %w", err)
		}

		var invitations []izanami.UserInvitation
		if err := json.Unmarshal(fileData, &invitations); err != nil {
			return fmt.Errorf("failed to parse invite file: %w", err)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.InviteUsersToTenant(ctx, cfg.Tenant, invitations); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "✅ Users invited to tenant successfully\n")
		return nil
	},
}

// ============================================================================
// PROJECT-SCOPED OPERATIONS
// ============================================================================

// usersListForProjectCmd lists users for a specific project
var usersListForProjectCmd = &cobra.Command{
	Use:   "list-for-project <project>",
	Short: "List all users with rights for a specific project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := args[0]

		if cfg.Tenant == "" {
			return fmt.Errorf(errors.MsgTenantRequired)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		users, err := client.ListUsersForProject(ctx, cfg.Tenant, project)
		if err != nil {
			return err
		}

		if len(users) == 0 {
			fmt.Fprintln(os.Stderr, "No users found for this project")
			return nil
		}

		return output.Print(users, output.Format(outputFormat))
	},
}

// usersUpdateProjectRightsCmd updates user's rights for a specific project
var usersUpdateProjectRightsCmd = &cobra.Command{
	Use:   "update-project-rights <username> <project>",
	Short: "Update user's rights for a specific project",
	Args:  cobra.ExactArgs(2),
	Long: `Update user's rights for a specific project.

You can provide rights via:
  1. A JSON file with --rights-file
  2. Individual flag --project-right (format: "level")

To remove rights, send an empty JSON object.

Examples:
  iz admin users update-project-rights johndoe myproject --tenant my-tenant --project-right "Admin"
  iz admin users update-project-rights johndoe myproject --tenant my-tenant --rights-file project-rights.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		username := args[0]
		project := args[1]

		if cfg.Tenant == "" {
			return fmt.Errorf(errors.MsgTenantRequired)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		var rightsData interface{}

		if userRightsFile != "" {
			fileData, err := os.ReadFile(userRightsFile)
			if err != nil {
				return fmt.Errorf("failed to read rights file: %w", err)
			}

			if err := json.Unmarshal(fileData, &rightsData); err != nil {
				return fmt.Errorf("failed to parse rights file: %w", err)
			}
		} else if userProjectRight != "" {
			rightsData = map[string]interface{}{
				"level": userProjectRight,
			}
		} else {
			return fmt.Errorf("no rights specified (use --project-right or --rights-file)")
		}

		ctx := context.Background()
		if err := client.UpdateUserProjectRights(ctx, cfg.Tenant, project, username, rightsData); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "✅ User project rights updated successfully\n")
		return nil
	},
}

// usersInviteToProjectCmd invites multiple users to a project
var usersInviteToProjectCmd = &cobra.Command{
	Use:   "invite-to-project <project>",
	Short: "Invite multiple users to a project with specified rights",
	Args:  cobra.ExactArgs(1),
	Long: `Invite multiple users to a project (bulk operation).

Provide invitations via JSON file with --invite-file.
The file should contain an array of objects with "username" and "level" fields.

Example JSON file:
[
  {"username": "user1", "level": "Read"},
  {"username": "user2", "level": "Admin"}
]

Example:
  iz admin users invite-to-project myproject --tenant my-tenant --invite-file invitations.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		project := args[0]

		if cfg.Tenant == "" {
			return fmt.Errorf(errors.MsgTenantRequired)
		}

		if usersInviteFile == "" {
			return fmt.Errorf("invite file is required (use --invite-file flag)")
		}

		fileData, err := os.ReadFile(usersInviteFile)
		if err != nil {
			return fmt.Errorf("failed to read invite file: %w", err)
		}

		var invitations []izanami.UserInvitation
		if err := json.Unmarshal(fileData, &invitations); err != nil {
			return fmt.Errorf("failed to parse invite file: %w", err)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.InviteUsersToProject(ctx, cfg.Tenant, project, invitations); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "✅ Users invited to project successfully\n")
		return nil
	},
}

func init() {
	adminCmd.AddCommand(usersCmd)

	// Global operations
	usersCmd.AddCommand(usersListCmd)
	usersCmd.AddCommand(usersGetCmd)
	usersCmd.AddCommand(usersCreateCmd)
	usersCmd.AddCommand(usersUpdateCmd)
	usersCmd.AddCommand(usersDeleteCmd)
	usersCmd.AddCommand(usersUpdateRightsCmd)
	usersCmd.AddCommand(usersSearchCmd)

	// Tenant-scoped operations
	usersCmd.AddCommand(usersListForTenantCmd)
	usersCmd.AddCommand(usersGetForTenantCmd)
	usersCmd.AddCommand(usersUpdateTenantRightsCmd)
	usersCmd.AddCommand(usersInviteToTenantCmd)

	// Project-scoped operations
	usersCmd.AddCommand(usersListForProjectCmd)
	usersCmd.AddCommand(usersUpdateProjectRightsCmd)
	usersCmd.AddCommand(usersInviteToProjectCmd)

	// Create command flags
	usersCreateCmd.Flags().StringVar(&userEmail, "email", "", "User email (required)")
	usersCreateCmd.Flags().StringVar(&userPassword, "password", "", "User password (required)")
	usersCreateCmd.Flags().BoolVar(&userAdmin, "admin", false, "Grant global admin privileges")
	usersCreateCmd.Flags().StringVar(&userType, "user-type", "INTERNAL", "User type (INTERNAL, OTOROSHI, OIDC)")
	usersCreateCmd.Flags().StringVar(&userDefaultTenant, "default-tenant", "", "Default tenant for the user")
	usersCreateCmd.Flags().StringVar(&userRightsFile, "rights-file", "", "Path to JSON file containing user rights")
	usersCreateCmd.Flags().StringVar(&userTenantRight, "tenant-right", "", "Tenant right (format: tenant:level)")
	usersCreateCmd.Flags().StringVar(&userProjectRight, "project-right", "", "Project right (format: tenant:project:level)")

	// Update command flags
	usersUpdateCmd.Flags().StringVar(&userEmail, "email", "", "New email address")
	usersUpdateCmd.Flags().StringVar(&userPassword, "password", "", "Current password for authentication (required)")
	usersUpdateCmd.Flags().StringVar(&userDefaultTenant, "default-tenant", "", "New default tenant")

	// Update rights command flags
	usersUpdateRightsCmd.Flags().BoolVar(&userAdmin, "admin", false, "Grant/revoke global admin privileges")
	usersUpdateRightsCmd.Flags().StringVar(&userRightsFile, "rights-file", "", "Path to JSON file containing rights")
	usersUpdateRightsCmd.Flags().StringVar(&userTenantRight, "tenant-right", "", "Tenant right (format: tenant:level)")

	// Delete command flags
	usersDeleteCmd.Flags().BoolVarP(&usersDeleteForce, "force", "f", false, "Skip confirmation prompt")

	// Search command flags
	usersSearchCmd.Flags().IntVar(&usersSearchCount, "count", 10, "Maximum number of results (max 100)")

	// Tenant-scoped command flags
	usersUpdateTenantRightsCmd.Flags().StringVar(&userRightsFile, "rights-file", "", "Path to JSON file containing tenant rights")
	usersUpdateTenantRightsCmd.Flags().StringVar(&userTenantRight, "tenant-right", "", "Tenant right level (Read, Write, Admin)")

	usersInviteToTenantCmd.Flags().StringVar(&usersInviteFile, "invite-file", "", "Path to JSON file with invitations (required)")

	// Project-scoped command flags
	usersUpdateProjectRightsCmd.Flags().StringVar(&userRightsFile, "rights-file", "", "Path to JSON file containing project rights")
	usersUpdateProjectRightsCmd.Flags().StringVar(&userProjectRight, "project-right", "", "Project right level (Read, Update, Write, Admin)")

	usersInviteToProjectCmd.Flags().StringVar(&usersInviteFile, "invite-file", "", "Path to JSON file with invitations (required)")
}

// printUserDetails displays user information in a fancy table format
func printUserDetails(user *izanami.User) error {
	// Print basic user information
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "User: %s\n", user.Username)
	fmt.Fprintf(os.Stderr, "Email: %s\n", user.Email)
	fmt.Fprintf(os.Stderr, "Type: %s\n", user.UserType)
	fmt.Fprintf(os.Stderr, "Admin: %t\n", user.Admin)
	if user.DefaultTenant != nil && *user.DefaultTenant != "" {
		fmt.Fprintf(os.Stderr, "Default Tenant: %s\n", *user.DefaultTenant)
	}

	// Print rights details
	if user.Admin {
		fmt.Fprintf(os.Stderr, "\n✓ Global Admin (full access to all resources)\n")
	}

	if len(user.Rights.Tenants) == 0 {
		fmt.Fprintf(os.Stderr, "\nNo tenant rights assigned.\n")
		return nil
	}

	fmt.Fprintf(os.Stderr, "\nTenant Rights:\n")
	fmt.Fprintf(os.Stderr, "%s\n", strings.Repeat("=", 80))

	// Sort tenant names for consistent output
	tenantNames := make([]string, 0, len(user.Rights.Tenants))
	for tenant := range user.Rights.Tenants {
		tenantNames = append(tenantNames, tenant)
	}
	sort.Strings(tenantNames)

	for _, tenant := range tenantNames {
		rights := user.Rights.Tenants[tenant]
		fmt.Fprintf(os.Stderr, "\n┌─ Tenant: %s\n", tenant)
		fmt.Fprintf(os.Stderr, "│  Level: %s\n", rights.Level)

		// Display default rights if set
		if rights.DefaultProjectRight != nil {
			fmt.Fprintf(os.Stderr, "│  Default Project Right: %s\n", *rights.DefaultProjectRight)
		}
		if rights.DefaultKeyRight != nil {
			fmt.Fprintf(os.Stderr, "│  Default Key Right: %s\n", *rights.DefaultKeyRight)
		}
		if rights.DefaultWebhookRight != nil {
			fmt.Fprintf(os.Stderr, "│  Default Webhook Right: %s\n", *rights.DefaultWebhookRight)
		}

		// Display projects
		if len(rights.Projects) > 0 {
			fmt.Fprintf(os.Stderr, "│\n│  Projects:\n")
			projectNames := make([]string, 0, len(rights.Projects))
			for proj := range rights.Projects {
				projectNames = append(projectNames, proj)
			}
			sort.Strings(projectNames)

			displayCount := min(3, len(projectNames))
			for i := 0; i < displayCount; i++ {
				proj := projectNames[i]
				fmt.Fprintf(os.Stderr, "│    • %s: %s\n", proj, rights.Projects[proj].Level)
			}
			if len(projectNames) > 3 {
				fmt.Fprintf(os.Stderr, "│    ... and %d more projects\n", len(projectNames)-3)
			}
		}

		// Display keys
		if len(rights.Keys) > 0 {
			fmt.Fprintf(os.Stderr, "│\n│  Keys:\n")
			keyNames := make([]string, 0, len(rights.Keys))
			for key := range rights.Keys {
				keyNames = append(keyNames, key)
			}
			sort.Strings(keyNames)

			displayCount := min(3, len(keyNames))
			for i := 0; i < displayCount; i++ {
				key := keyNames[i]
				fmt.Fprintf(os.Stderr, "│    • %s: %s\n", key, rights.Keys[key].Level)
			}
			if len(keyNames) > 3 {
				fmt.Fprintf(os.Stderr, "│    ... and %d more keys\n", len(keyNames)-3)
			}
		}

		// Display webhooks
		if len(rights.Webhooks) > 0 {
			fmt.Fprintf(os.Stderr, "│\n│  Webhooks:\n")
			webhookNames := make([]string, 0, len(rights.Webhooks))
			for wh := range rights.Webhooks {
				webhookNames = append(webhookNames, wh)
			}
			sort.Strings(webhookNames)

			displayCount := min(3, len(webhookNames))
			for i := 0; i < displayCount; i++ {
				wh := webhookNames[i]
				fmt.Fprintf(os.Stderr, "│    • %s: %s\n", wh, rights.Webhooks[wh].Level)
			}
			if len(webhookNames) > 3 {
				fmt.Fprintf(os.Stderr, "│    ... and %d more webhooks\n", len(webhookNames)-3)
			}
		}

		fmt.Fprintf(os.Stderr, "%s\n", "└"+strings.Repeat("─", 79))
	}

	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
