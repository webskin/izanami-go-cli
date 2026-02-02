package cmd

import (
	"context"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// completionTimeout is the maximum time to wait for API responses during completion
const completionTimeout = 5 * time.Second

// Completer handles shell completion with injectable dependencies for testing.
type Completer struct {
	// LoadConfig loads the completion configuration.
	// Returns nil if config cannot be loaded.
	LoadConfig func() *izanami.ResolvedConfig

	// ListTenants fetches tenants from the API.
	ListTenants func(cfg *izanami.ResolvedConfig, ctx context.Context) ([]izanami.Tenant, error)

	// ListProjects fetches projects from the API for a given tenant.
	ListProjects func(cfg *izanami.ResolvedConfig, ctx context.Context, tenant string) ([]izanami.Project, error)

	// ListTags fetches tags from the API for a given tenant.
	ListTags func(cfg *izanami.ResolvedConfig, ctx context.Context, tenant string) ([]izanami.Tag, error)

	// ListContexts fetches contexts from the API for a given tenant and optional project.
	ListContexts func(cfg *izanami.ResolvedConfig, ctx context.Context, tenant, project string) ([]izanami.Context, error)

	// Timeout for API calls. Defaults to completionTimeout if zero.
	Timeout time.Duration
}

// defaultCompleter is the production completer with real implementations.
var defaultCompleter = &Completer{
	LoadConfig:   loadCompletionConfig,
	ListTenants:  listTenantsAPI,
	ListProjects: listProjectsAPI,
	ListTags:     listTagsAPI,
	ListContexts: listContextsAPI,
	Timeout:      completionTimeout,
}

// listTenantsAPI is the production implementation for listing tenants.
func listTenantsAPI(cfg *izanami.ResolvedConfig, ctx context.Context) ([]izanami.Tenant, error) {
	client, err := izanami.NewAdminClient(cfg)
	if err != nil {
		return nil, err
	}
	return izanami.ListTenants(client, ctx, nil, izanami.ParseTenants)
}

// listProjectsAPI is the production implementation for listing projects.
func listProjectsAPI(cfg *izanami.ResolvedConfig, ctx context.Context, tenant string) ([]izanami.Project, error) {
	client, err := izanami.NewAdminClient(cfg)
	if err != nil {
		return nil, err
	}
	return izanami.ListProjects(client, ctx, tenant, izanami.ParseProjects)
}

// listTagsAPI is the production implementation for listing tags.
func listTagsAPI(cfg *izanami.ResolvedConfig, ctx context.Context, tenant string) ([]izanami.Tag, error) {
	client, err := izanami.NewAdminClient(cfg)
	if err != nil {
		return nil, err
	}
	return izanami.ListTags(client, ctx, tenant, izanami.ParseTags)
}

// listContextsAPI is the production implementation for listing contexts.
func listContextsAPI(cfg *izanami.ResolvedConfig, ctx context.Context, tenant, project string) ([]izanami.Context, error) {
	client, err := izanami.NewAdminClient(cfg)
	if err != nil {
		return nil, err
	}
	// Use all=true to get all nested contexts for completion
	return izanami.ListContexts(client, ctx, tenant, project, true, izanami.ParseContexts)
}

// getTimeout returns the configured timeout or the default.
func (c *Completer) getTimeout() time.Duration {
	if c.Timeout == 0 {
		return completionTimeout
	}
	return c.Timeout
}

// CompleteTenantNames provides dynamic completion for tenant names.
// It fetches the list of tenants from the API and returns them as completions.
// Fails silently if auth is not configured or API is unreachable.
func (c *Completer) CompleteTenantNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Only complete the first argument
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	cfg := c.LoadConfig()
	if cfg == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Validate admin auth is configured
	if err := cfg.ValidateAdminAuth(); err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Fetch tenants with timeout
	ctx, cancel := context.WithTimeout(context.Background(), c.getTimeout())
	defer cancel()

	tenants, err := c.ListTenants(cfg, ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return buildCompletions(tenants, toComplete,
		func(t izanami.Tenant) string { return t.Name },
		func(t izanami.Tenant) string { return t.Description },
	), cobra.ShellCompDirectiveNoFileComp
}

// CompleteProjectNames provides dynamic completion for project names.
// Requires tenant to be specified (via --tenant flag or profile).
// Fails silently if tenant is not set or API is unreachable.
func (c *Completer) CompleteProjectNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Only complete the first argument
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	cfg := c.LoadConfig()
	if cfg == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Tenant is required for project listing
	if cfg.Tenant == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Validate admin auth is configured
	if err := cfg.ValidateAdminAuth(); err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Fetch projects with timeout
	ctx, cancel := context.WithTimeout(context.Background(), c.getTimeout())
	defer cancel()

	projects, err := c.ListProjects(cfg, ctx, cfg.Tenant)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return buildCompletions(projects, toComplete,
		func(p izanami.Project) string { return p.Name },
		func(p izanami.Project) string { return p.Description },
	), cobra.ShellCompDirectiveNoFileComp
}

// CompleteTagNames provides dynamic completion for tag names.
// Requires tenant to be specified (via --tenant flag or profile).
// Fails silently if tenant is not set or API is unreachable.
func (c *Completer) CompleteTagNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Only complete the first argument
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	cfg := c.LoadConfig()
	if cfg == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Tenant is required for tag listing
	if cfg.Tenant == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Validate admin auth is configured
	if err := cfg.ValidateAdminAuth(); err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Fetch tags with timeout
	ctx, cancel := context.WithTimeout(context.Background(), c.getTimeout())
	defer cancel()

	tags, err := c.ListTags(cfg, ctx, cfg.Tenant)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return buildCompletions(tags, toComplete,
		func(t izanami.Tag) string { return t.Name },
		func(t izanami.Tag) string { return t.Description },
	), cobra.ShellCompDirectiveNoFileComp
}

// CompleteContextNames provides dynamic completion for context paths.
// Requires tenant to be specified (via --tenant flag or profile).
// Optionally uses --project flag to list project-specific contexts.
// Fails silently if tenant is not set or API is unreachable.
func (c *Completer) CompleteContextNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Only complete the first argument
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	cfg := c.LoadConfig()
	if cfg == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Tenant is required for context listing
	if cfg.Tenant == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Validate admin auth is configured
	if err := cfg.ValidateAdminAuth(); err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Check for --project flag (local flag on context commands)
	project := ""
	if cmd != nil {
		if p, err := cmd.Flags().GetString("project"); err == nil {
			project = p
		}
	}

	// Fetch contexts with timeout
	ctx, cancel := context.WithTimeout(context.Background(), c.getTimeout())
	defer cancel()

	contexts, err := c.ListContexts(cfg, ctx, cfg.Tenant, project)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Flatten nested contexts and use path for completion
	return buildContextCompletions(contexts, toComplete), cobra.ShellCompDirectiveNoFileComp
}

// buildContextCompletions flattens nested contexts and builds completions from paths.
func buildContextCompletions(contexts []izanami.Context, toComplete string) []string {
	var completions []string
	var flatten func(ctxs []izanami.Context)
	flatten = func(ctxs []izanami.Context) {
		for _, c := range ctxs {
			// Use path without leading slash, or name if path is empty
			path := strings.TrimPrefix(c.Path, "/")
			if path == "" {
				path = c.Name
			}
			if toComplete == "" || strings.HasPrefix(strings.ToLower(path), strings.ToLower(toComplete)) {
				// Add description based on context type
				desc := ""
				if c.Global {
					desc = "global"
				} else if c.IsProtected {
					desc = "protected"
				}
				if desc != "" {
					completions = append(completions, path+"\t"+desc)
				} else {
					completions = append(completions, path)
				}
			}
			// Recurse into children
			if len(c.Children) > 0 {
				// Convert []*Context to []Context for recursion
				children := make([]izanami.Context, len(c.Children))
				for i, child := range c.Children {
					children[i] = *child
				}
				flatten(children)
			}
		}
	}
	flatten(contexts)
	return completions
}

// buildCompletions filters items by prefix and formats them for shell completion.
// Uses tab-separated format "name\tdescription" when description is available.
func buildCompletions[T any](items []T, toComplete string, getName func(T) string, getDesc func(T) string) []string {
	var completions []string
	for _, item := range items {
		name := getName(item)
		if toComplete == "" || strings.HasPrefix(strings.ToLower(name), strings.ToLower(toComplete)) {
			if desc := getDesc(item); desc != "" {
				completions = append(completions, name+"\t"+desc)
			} else {
				completions = append(completions, name)
			}
		}
	}
	return completions
}

// Package-level functions that use the default completer.
// These are used by command wiring.

// completeTenantNames provides dynamic completion for tenant names.
func completeTenantNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return defaultCompleter.CompleteTenantNames(cmd, args, toComplete)
}

// completeProjectNames provides dynamic completion for project names.
func completeProjectNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return defaultCompleter.CompleteProjectNames(cmd, args, toComplete)
}

// completeTagNames provides dynamic completion for tag names.
func completeTagNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return defaultCompleter.CompleteTagNames(cmd, args, toComplete)
}

// completeContextNames provides dynamic completion for context paths.
func completeContextNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return defaultCompleter.CompleteContextNames(cmd, args, toComplete)
}

// completeConfigKeys provides completion for global config keys.
// These are static keys that don't require API calls.
func completeConfigKeys(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	keys := []struct{ Name, Desc string }{
		{"timeout", "Request timeout in seconds"},
		{"verbose", "Verbose output (true/false)"},
		{"output-format", "Default output format (table/json)"},
		{"color", "Color output (auto/always/never)"},
	}

	return buildCompletions(keys, toComplete,
		func(k struct{ Name, Desc string }) string { return k.Name },
		func(k struct{ Name, Desc string }) string { return k.Desc },
	), cobra.ShellCompDirectiveNoFileComp
}

// completeProfileNames provides completion for profile names from the config file.
func completeProfileNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	profiles, _, err := izanami.ListProfiles()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var completions []string
	for name := range profiles {
		if toComplete == "" || strings.HasPrefix(name, toComplete) {
			completions = append(completions, name)
		}
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// completeProfileKeys provides completion for profile setting keys.
// These are static keys that don't require API calls.
func completeProfileKeys(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	keys := []struct{ Name, Desc string }{
		{"leader-url", "Izanami leader URL"},
		{"tenant", "Default tenant name"},
		{"project", "Default project name"},
		{"context", "Default context path"},
		{"session", "Session name to reference"},
		{"personal-access-token", "Personal access token"},
		{"personal-access-token-username", "Username for PAT auth"},
		{"client-id", "Client ID for feature/event API"},
		{"client-secret", "Client secret for feature/event API"},
		{"default-worker", "Default worker name"},
	}

	return buildCompletions(keys, toComplete,
		func(k struct{ Name, Desc string }) string { return k.Name },
		func(k struct{ Name, Desc string }) string { return k.Desc },
	), cobra.ShellCompDirectiveNoFileComp
}

// completeWorkerNames provides completion for worker names from the active profile.
func completeWorkerNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	cfg := loadCompletionConfig()
	if cfg == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Get active profile to find workers
	profileName, err := izanami.GetActiveProfileName()
	if err != nil || profileName == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	profile, err := izanami.GetProfile(profileName)
	if err != nil || profile.Workers == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var completions []string
	for name, worker := range profile.Workers {
		if toComplete == "" || strings.HasPrefix(name, toComplete) {
			desc := worker.URL
			if profile.DefaultWorker == name {
				desc += " [default]"
			}
			completions = append(completions, name+"\t"+desc)
		}
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// RegisterFlagCompletions registers dynamic completions for global flags.
// This should be called from root.go init() after flags are defined.
func RegisterFlagCompletions() {
	// Register --tenant flag completion globally
	// This enables: iz admin projects list --tenant <TAB>
	rootCmd.RegisterFlagCompletionFunc("tenant", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeTenantNames(cmd, nil, toComplete)
	})

	// Register --project flag completion globally
	// This enables: iz admin contexts list --project <TAB>
	rootCmd.RegisterFlagCompletionFunc("project", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeProjectNames(cmd, nil, toComplete)
	})
}

// loadCompletionConfig loads configuration for completion functions.
// Returns nil if config cannot be loaded (completions will be empty).
// This function silently fails to avoid breaking shell completion.
func loadCompletionConfig() *izanami.ResolvedConfig {
	var cfg *izanami.ResolvedConfig
	var err error

	// Try to load with specific profile if --profile flag was used
	if profileName != "" {
		cfg, _, err = izanami.LoadConfigWithProfile(profileName)
	} else {
		// Load with active profile (if any)
		cfg, _, err = izanami.LoadConfigWithProfile("")
	}

	if err != nil {
		return nil
	}

	// Apply command-line flag overrides
	cfg.MergeWithFlags(izanami.FlagValues{
		LeaderURL: leaderURL,
		Tenant:    tenant,
		Project:   project,
		Context:   contextPath,
		Timeout:   timeout,
		Verbose:   false, // Never verbose during completion
	})

	return cfg
}
