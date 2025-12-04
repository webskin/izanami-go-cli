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

// completeTenantNames provides dynamic completion for tenant names.
// It fetches the list of tenants from the API and returns them as completions.
// Fails silently if auth is not configured or API is unreachable.
func completeTenantNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Only complete the first argument
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	cfg := loadCompletionConfig()
	if cfg == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Validate admin auth is configured
	if err := cfg.ValidateAdminAuth(); err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Create client
	client, err := izanami.NewClient(cfg)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Fetch tenants with timeout
	ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
	defer cancel()

	tenants, err := izanami.ListTenants(client, ctx, nil, izanami.ParseTenants)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Build completion list with descriptions (format: "name\tdescription")
	var completions []string
	for _, t := range tenants {
		// Filter by prefix (case-insensitive)
		if toComplete == "" || strings.HasPrefix(strings.ToLower(t.Name), strings.ToLower(toComplete)) {
			if t.Description != "" {
				completions = append(completions, t.Name+"\t"+t.Description)
			} else {
				completions = append(completions, t.Name)
			}
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}
