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
	LoadConfig func() *izanami.Config

	// ListTenants fetches tenants from the API.
	ListTenants func(cfg *izanami.Config, ctx context.Context) ([]izanami.Tenant, error)

	// ListProjects fetches projects from the API for a given tenant.
	ListProjects func(cfg *izanami.Config, ctx context.Context, tenant string) ([]izanami.Project, error)

	// Timeout for API calls. Defaults to completionTimeout if zero.
	Timeout time.Duration
}

// defaultCompleter is the production completer with real implementations.
var defaultCompleter = &Completer{
	LoadConfig:   loadCompletionConfig,
	ListTenants:  listTenantsAPI,
	ListProjects: listProjectsAPI,
	Timeout:      completionTimeout,
}

// listTenantsAPI is the production implementation for listing tenants.
func listTenantsAPI(cfg *izanami.Config, ctx context.Context) ([]izanami.Tenant, error) {
	client, err := izanami.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return izanami.ListTenants(client, ctx, nil, izanami.ParseTenants)
}

// listProjectsAPI is the production implementation for listing projects.
func listProjectsAPI(cfg *izanami.Config, ctx context.Context, tenant string) ([]izanami.Project, error) {
	client, err := izanami.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return izanami.ListProjects(client, ctx, tenant, izanami.ParseProjects)
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

// RegisterFlagCompletions registers dynamic completions for global flags.
// This should be called from root.go init() after flags are defined.
func RegisterFlagCompletions() {
	// Register --tenant flag completion globally
	// This enables: iz admin projects list --tenant <TAB>
	rootCmd.RegisterFlagCompletionFunc("tenant", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeTenantNames(cmd, nil, toComplete)
	})
}

// loadCompletionConfig loads configuration for completion functions.
// Returns nil if config cannot be loaded (completions will be empty).
// This function silently fails to avoid breaking shell completion.
func loadCompletionConfig() *izanami.Config {
	var cfg *izanami.Config
	var err error

	// Try to load with specific profile if --profile flag was used
	if profileName != "" {
		cfg, err = izanami.LoadConfigWithProfile(profileName)
	} else {
		// Load with active profile (if any)
		cfg, err = izanami.LoadConfigWithProfile("")
	}

	if err != nil {
		return nil
	}

	// Apply command-line flag overrides
	cfg.MergeWithFlags(izanami.FlagValues{
		BaseURL: baseURL,
		Tenant:  tenant,
		Project: project,
		Context: contextPath,
		Timeout: timeout,
		Verbose: false, // Never verbose during completion
	})

	return cfg
}
