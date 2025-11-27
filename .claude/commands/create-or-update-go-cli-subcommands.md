---
name: create-or-update-go-cli-subcommands
description: Create or update CLI subcommands from OpenAPI path
arguments:
  - name: $api_path
    description: The OpenAPI path to implement (e.g., /api/admin/tenants/{tenant}/projects)
    required: true
---

# CLI Subcommand Generator for Izanami

You are creating or updating CLI subcommands for the Izanami Go CLI based on an OpenAPI endpoint.

## Target Endpoint
- API Path: `{{api_path}}`

## Step 1: Look Up the Endpoint

Read `docs/unsafe-izanami-openapi.yaml` and find the endpoint definition for `{{api_path}}`.

Extract:
- **HTTP methods** supported (GET, POST, PUT, DELETE)
- **Operation ID** and description
- **Path parameters** (e.g., `{tenant}`, `{project}`)
- **Query parameters** (name, type, required, description)
- **Request body schema** (if POST/PUT)
- **Response schema** (especially for GET operations)
- **Tags** (used to group related endpoints)

## Step 2: Determine Command Hierarchy

Based on the path structure, determine:

| Path Pattern | Command Structure | File |
|--------------|-------------------|------|
| `/api/admin/tenants` | `iz admin tenants [action]` | `admin.go` |
| `/api/admin/tenants/{tenant}/projects` | `iz admin projects [action]` | `admin.go` |
| `/api/admin/tenants/{tenant}/tags` | `iz admin tags [action]` | `admin.go` |
| `/api/admin/tenants/{tenant}/keys` | `iz admin keys [action]` | `keys.go` |
| `/api/admin/tenants/{tenant}/contexts` | `iz admin contexts [action]` | `contexts.go` |
| `/api/admin/users` | `iz admin users [action]` | `users.go` |
| `/api/v2/features` | `iz features [action]` | `features.go` |

**Action mapping from HTTP method:**
- `GET` (collection) ’ `list`
- `GET` (single item) ’ `get`
- `POST` ’ `create`
- `PUT` ’ `update`
- `DELETE` ’ `delete`
- Special paths like `/logs` ’ `logs`

## Step 3: Check Existing Implementations

Before writing code, search for existing implementations:

1. **In `internal/cmd/`**: Look for related commands
2. **In `internal/izanami/`**: Look for related API client methods
3. **Identify** if this is a new command or an update to existing code

## Step 4: Generate API Client Method

Add/update method in appropriate file under `internal/izanami/`.

### Pattern for GET (List)

```go
func (c *Client) ListResources(ctx context.Context, tenant string, opts *ListOptions) ([]Resource, error) {
	path := apiAdminTenants + buildPath(tenant, "resources")

	var result []Resource
	req := c.http.R().
		SetContext(ctx).
		SetResult(&result)
	c.setAdminAuth(req)

	if opts != nil {
		if opts.Filter != "" {
			req.SetQueryParam("filter", opts.Filter)
		}
	}

	resp, err := req.Get(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToListResources, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return result, nil
}
```

### Pattern for GET (Single)

```go
func (c *Client) GetResource(ctx context.Context, tenant, name string) (*Resource, error) {
	path := apiAdminTenants + buildPath(tenant, "resources", name)

	var result Resource
	req := c.http.R().
		SetContext(ctx).
		SetResult(&result)
	c.setAdminAuth(req)

	resp, err := req.Get(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToGetResource, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return &result, nil
}
```

### Pattern for POST (Create)

```go
func (c *Client) CreateResource(ctx context.Context, tenant string, data interface{}) (*Resource, error) {
	path := apiAdminTenants + buildPath(tenant, "resources")

	var result Resource
	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(data).
		SetResult(&result)
	c.setAdminAuth(req)

	resp, err := req.Post(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToCreateResource, err)
	}

	if resp.StatusCode() != http.StatusCreated && resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return &result, nil
}
```

### Pattern for PUT (Update)

```go
func (c *Client) UpdateResource(ctx context.Context, tenant, name string, data interface{}) error {
	path := apiAdminTenants + buildPath(tenant, "resources", name)

	req := c.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(data)
	c.setAdminAuth(req)

	resp, err := req.Put(path)
	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToUpdateResource, err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return c.handleError(resp)
	}

	return nil
}
```

### Pattern for DELETE

```go
func (c *Client) DeleteResource(ctx context.Context, tenant, name string) error {
	path := apiAdminTenants + buildPath(tenant, "resources", name)

	req := c.http.R().
		SetContext(ctx)
	c.setAdminAuth(req)

	resp, err := req.Delete(path)
	if err != nil {
		return fmt.Errorf("%s: %w", errmsg.MsgFailedToDeleteResource, err)
	}

	// Accept 200, 204, or 404 (already deleted) as success
	if resp.StatusCode() != http.StatusOK &&
	   resp.StatusCode() != http.StatusNoContent &&
	   resp.StatusCode() != http.StatusNotFound {
		return c.handleError(resp)
	}

	return nil
}
```

### Add Types if Needed

Define response/request types in `internal/izanami/types.go` or the appropriate domain file:

```go
type Resource struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type ResourceSummary struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}
```

### Add Error Messages

Add error message constants in `internal/izanami/errmsg/messages.go`:

```go
const (
	MsgFailedToListResources   = "failed to list resources"
	MsgFailedToGetResource     = "failed to get resource"
	MsgFailedToCreateResource  = "failed to create resource"
	MsgFailedToUpdateResource  = "failed to update resource"
	MsgFailedToDeleteResource  = "failed to delete resource"
)
```

## Step 5: Generate CLI Command

Add/update command in appropriate file under `internal/cmd/`.

### Pattern for List Command

```go
var adminResourcesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all resources",
	Long: `List all resources in the tenant.

Examples:
  iz admin resources list
  iz admin resources list --tenant my-tenant
  iz admin resources list -o json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		resources, err := client.ListResources(ctx, cfg.Tenant, nil)
		if err != nil {
			return err
		}

		// Convert to summaries for cleaner output
		summaries := make([]izanami.ResourceSummary, len(resources))
		for i, r := range resources {
			summaries[i] = izanami.ResourceSummary{
				Name:        r.Name,
				Description: r.Description,
			}
		}

		return output.Print(summaries, output.Format(outputFormat))
	},
}
```

### Pattern for Get Command

```go
var adminResourcesGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Get resource details",
	Long: `Get detailed information about a specific resource.

Examples:
  iz admin resources get my-resource
  iz admin resources get my-resource -o json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		resource, err := client.GetResource(ctx, cfg.Tenant, args[0])
		if err != nil {
			return err
		}

		return output.Print(resource, output.Format(outputFormat))
	},
}
```

### Pattern for Create Command

```go
var (
	resourceDesc string
	resourceData string
)

var adminResourcesCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new resource",
	Long: `Create a new resource in the tenant.

Examples:
  iz admin resources create my-resource
  iz admin resources create my-resource --description "My description"
  iz admin resources create my-resource --data '{"name":"my-resource","field":"value"}'
  iz admin resources create my-resource --data @resource.json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		name := args[0]
		var data interface{}

		if cmd.Flags().Changed("data") {
			if err := parseJSONData(resourceData, &data); err != nil {
				return fmt.Errorf("invalid JSON data: %w", err)
			}
		} else {
			data = map[string]interface{}{
				"name":        name,
				"description": resourceDesc,
			}
		}

		ctx := context.Background()
		result, err := client.CreateResource(ctx, cfg.Tenant, data)
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Resource created successfully: %s\n", name)
		return output.Print(result, output.Format(outputFormat))
	},
}
```

### Pattern for Update Command

```go
var adminResourcesUpdateCmd = &cobra.Command{
	Use:   "update <name>",
	Short: "Update a resource",
	Long: `Update an existing resource.

Examples:
  iz admin resources update my-resource --data '{"description":"Updated"}'
  iz admin resources update my-resource --data @resource.json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		name := args[0]
		var data interface{}
		if err := parseJSONData(resourceData, &data); err != nil {
			return fmt.Errorf("invalid JSON data: %w", err)
		}

		ctx := context.Background()
		if err := client.UpdateResource(ctx, cfg.Tenant, name, data); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Resource updated successfully: %s\n", name)
		return nil
	},
}
```

### Pattern for Delete Command

```go
var resourcesDeleteForce bool

var adminResourcesDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a resource",
	Long: `Delete a resource from the tenant.

Examples:
  iz admin resources delete my-resource
  iz admin resources delete my-resource --force`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		name := args[0]

		if !resourcesDeleteForce {
			if !confirmDeletion(cmd, "resource", name) {
				return nil
			}
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.DeleteResource(ctx, cfg.Tenant, name); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Resource deleted successfully: %s\n", name)
		return nil
	},
}
```

### Register Commands in init()

```go
func init() {
	// Add parent command if new
	adminCmd.AddCommand(adminResourcesCmd)

	// Add subcommands
	adminResourcesCmd.AddCommand(adminResourcesListCmd)
	adminResourcesCmd.AddCommand(adminResourcesGetCmd)
	adminResourcesCmd.AddCommand(adminResourcesCreateCmd)
	adminResourcesCmd.AddCommand(adminResourcesUpdateCmd)
	adminResourcesCmd.AddCommand(adminResourcesDeleteCmd)

	// Register flags
	adminResourcesCreateCmd.Flags().StringVar(&resourceDesc, "description", "", "Resource description")
	adminResourcesCreateCmd.Flags().StringVar(&resourceData, "data", "", "JSON data (inline, @file, or - for stdin)")

	adminResourcesUpdateCmd.Flags().StringVar(&resourceData, "data", "", "JSON data (inline, @file, or - for stdin)")
	adminResourcesUpdateCmd.MarkFlagRequired("data")

	adminResourcesDeleteCmd.Flags().BoolVarP(&resourcesDeleteForce, "force", "f", false, "Skip confirmation prompt")
}
```

## Step 6: Important Patterns to Follow

### Cobra Best Practices
- Use `RunE` (not `Run`) for error handling
- Use `cmd.OutOrStdout()` and `cmd.OutOrStderr()` for testability
- Use `cmd.InOrStdin()` for user input
- Return errors instead of calling `os.Exit()`

### JSON Data Input
Support flexible JSON input using `parseJSONData()`:
- Inline: `--data '{"key":"value"}'`
- File: `--data @file.json`
- Stdin: `--data -`

### Output Formatting
Support both JSON and table output:
```go
return output.Print(result, output.Format(outputFormat))
```

### Error Messages
Use the `errmsg` package for consistent error messages.

### Path Building
Use `buildPath()` for URL-safe path construction:
```go
path := apiAdminTenants + buildPath(tenant, "resources", name)
```

### Authentication
- Admin APIs (`/api/admin/`): Use `c.setAdminAuth(req)`
- Client APIs (`/api/v2/`): Use `c.setClientAuth(req)` with error check

## Step 7: Verification

After generating code:
1. Run `make build` to verify compilation
2. Run `make test` to verify tests pass
3. Test the new command manually

## Reference Files

Read these files for patterns:
- `docs/unsafe-izanami-openapi.yaml` - API specification
- `internal/cmd/admin.go` - Command hierarchy and CRUD patterns
- `internal/cmd/features.go` - Feature command patterns
- `internal/izanami/tenants.go` - Simple API client methods
- `internal/izanami/features.go` - Complex API client methods
- `internal/izanami/client.go` - Client structure and helpers

Now analyze `{{api_path}}` in the OpenAPI spec and generate the appropriate code.
