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

> **Note**: This OpenAPI spec is AI-generated and may contain inaccuracies. Use it as a starting point, but always verify against actual server responses in Step 7.

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
- `GET` (collection) → `list`
- `GET` (single item) → `get`
- `POST` → `create`
- `PUT` → `update`
- `DELETE` → `delete`
- Special paths like `/logs` → `logs`

## Step 3: Check Existing Implementations

Before writing code, search for existing implementations:

1. **In `internal/cmd/`**: Look for related commands
2. **In `internal/izanami/`**: Look for related API client methods
3. **Identify** if this is a new command or an update to existing code

## Step 4: Generate API Client Method

Add/update method in appropriate file under `internal/izanami/`.

**IMPORTANT**: Use the functional mapper pattern for all GET operations that return data. This allows:
- Raw JSON passthrough with `Identity` mapper for `-o json` output
- Typed parsing with `Parse*` mapper for table output

### Pattern for GET (List) - With Mapper

```go
// ListResources lists all resources and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseResources for typed structs.
func ListResources[T any](c *Client, ctx context.Context, tenant string, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.listResourcesRaw(ctx, tenant)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// listResourcesRaw fetches resources and returns raw JSON bytes
func (c *Client) listResourcesRaw(ctx context.Context, tenant string) ([]byte, error) {
	path := apiAdminTenants + buildPath(tenant, "resources")

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)

	resp, err := req.Get(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToListResources, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}
```

### Pattern for GET (Single) - With Mapper

```go
// GetResource retrieves a specific resource and applies the given mapper.
// Use Identity mapper for raw JSON output, or ParseResource for typed struct.
func GetResource[T any](c *Client, ctx context.Context, tenant, name string, mapper Mapper[T]) (T, error) {
	var zero T
	raw, err := c.getResourceRaw(ctx, tenant, name)
	if err != nil {
		return zero, err
	}
	return mapper(raw)
}

// getResourceRaw fetches a resource and returns raw JSON bytes
func (c *Client) getResourceRaw(ctx context.Context, tenant, name string) ([]byte, error) {
	path := apiAdminTenants + buildPath(tenant, "resources", name)

	req := c.http.R().SetContext(ctx)
	c.setAdminAuth(req)

	resp, err := req.Get(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errmsg.MsgFailedToGetResource, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}
```

### Pattern for POST (Create)

Note: POST/PUT/DELETE don't typically need the mapper pattern as they don't return raw JSON for display.

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

	req := c.http.R().SetContext(ctx)
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

### Add Mappers in mappers.go

Add mapper definitions using the generic factories:

```go
var (
	// In the var block in internal/izanami/mappers.go
	ParseResources = Unmarshal[[]Resource]()
	ParseResource  = UnmarshalPtr[Resource]()
)
```

The mapper factories are:
- `Unmarshal[T]()` - for values and slices (e.g., `[]Resource`)
- `UnmarshalPtr[T]()` - for single objects as pointers (e.g., `*Resource`)

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

Add error message constants in `internal/errors/messages.go`:

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

### Pattern for List Command - With Mapper

```go
var adminResourcesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all resources",
	Long: `List all resources in the tenant.

Examples:
  iz admin resources list
  iz admin resources list --tenant my-tenant
  iz admin resources list -o json
  iz admin resources list -o json --compact`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()

		// For JSON output, use Identity mapper for raw JSON passthrough
		if outputFormat == "json" {
			raw, err := izanami.ListResources(client, ctx, cfg.Tenant, izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseResources mapper
		resources, err := izanami.ListResources(client, ctx, cfg.Tenant, izanami.ParseResources)
		if err != nil {
			return err
		}

		if len(resources) == 0 {
			fmt.Fprintln(cmd.OutOrStderr(), "No resources found")
			return nil
		}

		return output.Print(resources, output.Format(outputFormat))
	},
}
```

### Pattern for Get Command - With Mapper

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

		// For JSON output, use Identity mapper for raw JSON passthrough
		if outputFormat == "json" {
			raw, err := izanami.GetResource(client, ctx, cfg.Tenant, args[0], izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseResource mapper
		resource, err := izanami.GetResource(client, ctx, cfg.Tenant, args[0], izanami.ParseResource)
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

		fmt.Fprintf(cmd.OutOrStderr(), "✅ Resource created successfully: %s\n", name)
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

		fmt.Fprintf(cmd.OutOrStderr(), "✅ Resource updated successfully: %s\n", name)
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

		fmt.Fprintf(cmd.OutOrStderr(), "✅ Resource deleted successfully: %s\n", name)
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

### Mapper Pattern (Critical)
All GET operations that return data MUST use the mapper pattern:
- Generic function `ListResources[T any](..., mapper Mapper[T])` as public API
- Private `*Raw` method returns `[]byte`
- CLI checks `outputFormat == "json"` to choose between `Identity` and `Parse*` mappers
- Use `output.PrintRawJSON()` for raw JSON output (supports `--compact` flag)

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
Support both JSON and table output with the mapper pattern:
```go
// JSON output - raw passthrough
if outputFormat == "json" {
    raw, err := izanami.ListResources(client, ctx, tenant, izanami.Identity)
    return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
}

// Table output - typed parsing
resources, err := izanami.ListResources(client, ctx, tenant, izanami.ParseResources)
return output.Print(resources, output.Format(outputFormat))
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

## Step 7: Verify Against Live Server

**IMPORTANT**: The OpenAPI spec (`docs/unsafe-izanami-openapi.yaml`) is AI-generated and may have inaccuracies. Always verify against the actual server response.

### 7.1 Test with Verbose Mode

After implementing the command, test it against a live Izanami server with `--verbose` to see the raw HTTP response:

```bash
# For GET operations - use verbose and JSON output to see raw response
iz admin resources list --verbose -o json 2>&1 | head -50

# Or capture just the response body
iz admin resources get my-resource -o json --compact
```

The verbose output shows:
- Request URL and method
- Request headers
- Response status code
- Response headers
- Response body (raw JSON)

### 7.2 Compare Response with OpenAPI Spec

Compare the actual JSON response structure with the schema defined in `docs/unsafe-izanami-openapi.yaml`.

Check for discrepancies:
- **Missing fields**: Fields in the response not in the spec
- **Extra fields**: Fields in the spec not in the response
- **Type mismatches**: Different data types (string vs number, array vs object)
- **Naming differences**: camelCase vs snake_case, different field names
- **Nested structure**: Different nesting or wrapper objects

### 7.3 Update OpenAPI Spec if Needed

If the actual response differs from the spec, **update `docs/unsafe-izanami-openapi.yaml`**:

1. Locate the endpoint's response schema in the spec
2. Update the schema to match the actual server response
3. Add any missing fields with correct types
4. Remove fields that don't exist in actual responses
5. Fix any type mismatches

Example fix:
```yaml
# Before (incorrect)
ResponseSchema:
  type: object
  properties:
    id:
      type: string
    name:
      type: string

# After (matches actual response)
ResponseSchema:
  type: object
  properties:
    id:
      type: string
      format: uuid
    name:
      type: string
    description:
      type: string
    createdAt:
      type: string
      format: date-time
```

### 7.4 Update Go Types

After updating the OpenAPI spec, ensure Go types in `internal/izanami/types.go` match:

```go
// Update struct to match actual response
type Resource struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description,omitempty"`
    CreatedAt   time.Time `json:"createdAt,omitempty"`  // Added field
}
```

### 7.5 Re-run Tests

After any schema updates:
```bash
make build  # Verify compilation
make test   # Verify tests pass
```

## Step 8: Final Verification

After generating code and verifying against live server:
1. Run `make build` to verify compilation
2. Run `make test` to verify tests pass
3. Test the new command manually with various inputs
4. Verify JSON output matches server response exactly
5. Verify table output displays correctly

## Reference Files

Read these files for patterns:
- `docs/unsafe-izanami-openapi.yaml` - API specification (AI-generated, verify against live server)
- `internal/izanami/mappers.go` - Mapper definitions and factories
- `internal/cmd/admin.go` - Command hierarchy and CRUD patterns with mappers
- `internal/cmd/features.go` - Feature command patterns
- `internal/izanami/tenants.go` - API client methods with mapper pattern
- `internal/izanami/features.go` - Complex API client methods
- `internal/izanami/client.go` - Client structure and helpers

## Workflow Summary

1. **Look up** endpoint in OpenAPI spec (Step 1)
2. **Determine** command hierarchy (Step 2)
3. **Check** existing implementations (Step 3)
4. **Generate** API client method with mapper pattern (Step 4)
5. **Generate** CLI command (Step 5)
6. **Follow** important patterns (Step 6)
7. **Verify** against live server and update OpenAPI spec if needed (Step 7)
8. **Final** verification with build and tests (Step 8)

Now analyze `{{api_path}}` in the OpenAPI spec and generate the appropriate code.
