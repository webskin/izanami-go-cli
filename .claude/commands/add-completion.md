---
name: add-completion
description: Add dynamic shell tab-completion for a CLI command argument
arguments:
  - name: $resource_type
    description: The resource type to complete (e.g., projects, keys, features, tags)
    required: true
---

# Add Shell Completion for {{resource_type}}

Add dynamic tab-completion for {{resource_type}} from the API.

## Two Types of Completion

### Type 1: Positional Argument Completion (ValidArgsFunction)
For commands like `iz admin {{resource_type}} get <name> <TAB>`

### Type 2: Flag Value Completion (RegisterFlagCompletionFunc)
For flags like `iz admin features list --project <TAB>`

---

## Files to Modify

1. `internal/cmd/completions.go` - Add completion function
2. Command file - Wire `ValidArgsFunction` in `init()`
3. (Optional) `internal/cmd/completions.go` - Add to `RegisterFlagCompletions()` for flag completion

---

## Step 1: Add Completion Function

Add to `internal/cmd/completions.go`:

### Pattern A: Top-level resource (no dependencies, like tenants)

```go
// complete{{resource_type | capitalize}}Names provides dynamic completion for {{resource_type}} names.
func complete{{resource_type | capitalize}}Names(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
    if len(args) != 0 {
        return nil, cobra.ShellCompDirectiveNoFileComp
    }

    cfg := loadCompletionConfig()
    if cfg == nil {
        return nil, cobra.ShellCompDirectiveNoFileComp
    }

    if err := cfg.ValidateAdminAuth(); err != nil {
        return nil, cobra.ShellCompDirectiveNoFileComp
    }

    client, err := izanami.NewClient(cfg)
    if err != nil {
        return nil, cobra.ShellCompDirectiveNoFileComp
    }

    ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
    defer cancel()

    items, err := izanami.List{{resource_type | capitalize}}(client, ctx, izanami.Parse{{resource_type | capitalize}})
    if err != nil {
        return nil, cobra.ShellCompDirectiveNoFileComp
    }

    return buildCompletions(items, toComplete,
        func(item izanami.{{resource_type | capitalize}}) string { return item.Name },
        func(item izanami.{{resource_type | capitalize}}) string { return item.Description },
    ), cobra.ShellCompDirectiveNoFileComp
}
```

### Pattern B: Dependent resource (needs tenant, like projects/keys/tags)

```go
// complete{{resource_type | capitalize}}Names provides dynamic completion for {{resource_type}} names.
// Requires tenant to be specified (via --tenant flag or profile).
func complete{{resource_type | capitalize}}Names(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
    if len(args) != 0 {
        return nil, cobra.ShellCompDirectiveNoFileComp
    }

    cfg := loadCompletionConfig()
    if cfg == nil {
        return nil, cobra.ShellCompDirectiveNoFileComp
    }

    // Check dependency: tenant is required
    if cfg.Tenant == "" {
        return nil, cobra.ShellCompDirectiveNoFileComp
    }

    if err := cfg.ValidateAdminAuth(); err != nil {
        return nil, cobra.ShellCompDirectiveNoFileComp
    }

    client, err := izanami.NewClient(cfg)
    if err != nil {
        return nil, cobra.ShellCompDirectiveNoFileComp
    }

    ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
    defer cancel()

    items, err := izanami.List{{resource_type | capitalize}}(client, ctx, cfg.Tenant, izanami.Parse{{resource_type | capitalize}})
    if err != nil {
        return nil, cobra.ShellCompDirectiveNoFileComp
    }

    return buildCompletions(items, toComplete,
        func(item izanami.{{resource_type | capitalize}}) string { return item.Name },
        func(item izanami.{{resource_type | capitalize}}) string { return item.Description },
    ), cobra.ShellCompDirectiveNoFileComp
}
```

---

## Step 2: Wire ValidArgsFunction

In the command's `init()` function (e.g., `projects.go`):

```go
// Dynamic completion for {{resource_type}} name argument
admin{{resource_type | capitalize}}GetCmd.ValidArgsFunction = complete{{resource_type | capitalize}}Names
```

---

## Step 3: (Optional) Add Flag Value Completion

To complete flag values like `--tenant <TAB>`, add to `RegisterFlagCompletions()` in `completions.go`:

```go
func RegisterFlagCompletions() {
    // Existing: --tenant flag completion
    rootCmd.RegisterFlagCompletionFunc("tenant", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
        return completeTenantNames(cmd, nil, toComplete)
    })

    // Add new flag completion here:
    rootCmd.RegisterFlagCompletionFunc("{{resource_type}}", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
        return complete{{resource_type | capitalize}}Names(cmd, nil, toComplete)
    })
}
```

**Important:** `RegisterFlagCompletions()` is called from `root.go init()` after flags are defined (init order matters!).

---

## Step 4: Test Completion

```bash
make build

# Test positional argument completion
./build/iz __complete admin {{resource_type}} get ""

# Test with dependency (e.g., projects need tenant)
./build/iz __complete admin {{resource_type}} get --tenant my-tenant ""

# Test flag value completion
./build/iz __complete admin features list --{{resource_type}} ""

# Interactive test
source <(./build/iz completion bash)
iz admin {{resource_type}} get <TAB>
```

---

## Existing Implementations

| Function | File | Dependencies |
|----------|------|--------------|
| `completeTenantNames` | completions.go | None |
| `completeProjectNames` | completions.go | `cfg.Tenant` |

## API Functions

| Resource | API Call |
|----------|----------|
| Tenants | `ListTenants(client, ctx, nil, izanami.ParseTenants)` |
| Projects | `ListProjects(client, ctx, cfg.Tenant, izanami.ParseProjects)` |
| Features | `ListFeatures(client, ctx, cfg.Tenant, tag, izanami.ParseFeatures)` |
| Keys | `ListAPIKeys(client, ctx, cfg.Tenant, izanami.ParseAPIKeys)` |
| Tags | `ListTags(client, ctx, cfg.Tenant, izanami.ParseTags)` |
| Webhooks | `ListWebhooks(client, ctx, cfg.Tenant, izanami.ParseWebhooks)` |

## Key Points

- `loadCompletionConfig()` merges profile + flags into config
- `buildCompletions[T]()` generic helper filters by prefix and formats with descriptions
- Check dependencies (e.g., `cfg.Tenant == ""`) before API calls
- 5-second timeout via `completionTimeout` constant
- Always return `cobra.ShellCompDirectiveNoFileComp`
- Fail silently (return nil) - never break shell completion
- Description format: `"name\tdescription"` (tab-separated, handled by buildCompletions)
