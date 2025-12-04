package cmd

import (
	"context"
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

func TestBuildCompletions(t *testing.T) {
	type item struct {
		Name string
		Desc string
	}

	tests := []struct {
		name       string
		items      []item
		toComplete string
		want       []string
	}{
		{
			name: "empty items",
			items: []item{},
			toComplete: "",
			want: nil,
		},
		{
			name: "all items when toComplete is empty",
			items: []item{
				{Name: "alpha", Desc: "First item"},
				{Name: "beta", Desc: "Second item"},
			},
			toComplete: "",
			want: []string{"alpha\tFirst item", "beta\tSecond item"},
		},
		{
			name: "filter by prefix",
			items: []item{
				{Name: "alpha", Desc: "First"},
				{Name: "beta", Desc: "Second"},
				{Name: "alphabet", Desc: "Third"},
			},
			toComplete: "alp",
			want: []string{"alpha\tFirst", "alphabet\tThird"},
		},
		{
			name: "case insensitive filter",
			items: []item{
				{Name: "Alpha", Desc: "First"},
				{Name: "ALPHABET", Desc: "Second"},
				{Name: "beta", Desc: "Third"},
			},
			toComplete: "ALP",
			want: []string{"Alpha\tFirst", "ALPHABET\tSecond"},
		},
		{
			name: "items without description",
			items: []item{
				{Name: "alpha", Desc: ""},
				{Name: "beta", Desc: "Has description"},
			},
			toComplete: "",
			want: []string{"alpha", "beta\tHas description"},
		},
		{
			name: "no matches",
			items: []item{
				{Name: "alpha", Desc: "First"},
				{Name: "beta", Desc: "Second"},
			},
			toComplete: "gamma",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCompletions(tt.items, tt.toComplete,
				func(i item) string { return i.Name },
				func(i item) string { return i.Desc },
			)

			if len(got) != len(tt.want) {
				t.Errorf("buildCompletions() returned %d items, want %d", len(got), len(tt.want))
				return
			}

			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("buildCompletions()[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

func TestCompleter_CompleteTenantNames(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		toComplete   string
		loadConfig   func() *izanami.Config
		listTenants  func(cfg *izanami.Config, ctx context.Context) ([]izanami.Tenant, error)
		wantResults  []string
		wantDirective cobra.ShellCompDirective
	}{
		{
			name: "returns tenants on success",
			args: []string{},
			toComplete: "",
			loadConfig: func() *izanami.Config {
				return &izanami.Config{
					PersonalAccessToken:         "test-token",
					PersonalAccessTokenUsername: "test-user",
					BaseURL:                     "http://localhost",
				}
			},
			listTenants: func(cfg *izanami.Config, ctx context.Context) ([]izanami.Tenant, error) {
				return []izanami.Tenant{
					{Name: "tenant-1", Description: "First tenant"},
					{Name: "tenant-2", Description: ""},
				}, nil
			},
			wantResults: []string{"tenant-1\tFirst tenant", "tenant-2"},
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name: "filters by prefix",
			args: []string{},
			toComplete: "tenant-1",
			loadConfig: func() *izanami.Config {
				return &izanami.Config{
					PersonalAccessToken:         "test-token",
					PersonalAccessTokenUsername: "test-user",
					BaseURL:                     "http://localhost",
				}
			},
			listTenants: func(cfg *izanami.Config, ctx context.Context) ([]izanami.Tenant, error) {
				return []izanami.Tenant{
					{Name: "tenant-1", Description: "First"},
					{Name: "tenant-2", Description: "Second"},
				}, nil
			},
			wantResults: []string{"tenant-1\tFirst"},
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name: "returns nil when args not empty",
			args: []string{"existing-arg"},
			toComplete: "",
			loadConfig: func() *izanami.Config {
				return &izanami.Config{PersonalAccessToken: "token"}
			},
			listTenants: func(cfg *izanami.Config, ctx context.Context) ([]izanami.Tenant, error) {
				t.Error("listTenants should not be called when args not empty")
				return nil, nil
			},
			wantResults: nil,
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name: "returns nil when config is nil",
			args: []string{},
			toComplete: "",
			loadConfig: func() *izanami.Config {
				return nil
			},
			listTenants: func(cfg *izanami.Config, ctx context.Context) ([]izanami.Tenant, error) {
				t.Error("listTenants should not be called when config is nil")
				return nil, nil
			},
			wantResults: nil,
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name: "returns nil when no admin auth",
			args: []string{},
			toComplete: "",
			loadConfig: func() *izanami.Config {
				return &izanami.Config{
					// No PersonalAccessToken or JwtToken
					BaseURL: "http://localhost",
				}
			},
			listTenants: func(cfg *izanami.Config, ctx context.Context) ([]izanami.Tenant, error) {
				t.Error("listTenants should not be called when no admin auth")
				return nil, nil
			},
			wantResults: nil,
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name: "returns nil on API error",
			args: []string{},
			toComplete: "",
			loadConfig: func() *izanami.Config {
				return &izanami.Config{
					PersonalAccessToken:         "test-token",
					PersonalAccessTokenUsername: "test-user",
					BaseURL:                     "http://localhost",
				}
			},
			listTenants: func(cfg *izanami.Config, ctx context.Context) ([]izanami.Tenant, error) {
				return nil, errors.New("API error")
			},
			wantResults: nil,
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Completer{
				LoadConfig:  tt.loadConfig,
				ListTenants: tt.listTenants,
				Timeout:     completionTimeout,
			}

			got, directive := c.CompleteTenantNames(nil, tt.args, tt.toComplete)

			if directive != tt.wantDirective {
				t.Errorf("CompleteTenantNames() directive = %v, want %v", directive, tt.wantDirective)
			}

			if len(got) != len(tt.wantResults) {
				t.Errorf("CompleteTenantNames() returned %d results, want %d", len(got), len(tt.wantResults))
				return
			}

			for i, want := range tt.wantResults {
				if got[i] != want {
					t.Errorf("CompleteTenantNames()[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

func TestCompleter_CompleteProjectNames(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		toComplete    string
		loadConfig    func() *izanami.Config
		listProjects  func(cfg *izanami.Config, ctx context.Context, tenant string) ([]izanami.Project, error)
		wantResults   []string
		wantDirective cobra.ShellCompDirective
	}{
		{
			name: "returns projects on success",
			args: []string{},
			toComplete: "",
			loadConfig: func() *izanami.Config {
				return &izanami.Config{
					PersonalAccessToken:         "test-token",
					PersonalAccessTokenUsername: "test-user",
					BaseURL:                     "http://localhost",
					Tenant:                      "my-tenant",
				}
			},
			listProjects: func(cfg *izanami.Config, ctx context.Context, tenant string) ([]izanami.Project, error) {
				if tenant != "my-tenant" {
					t.Errorf("listProjects called with tenant %q, want %q", tenant, "my-tenant")
				}
				return []izanami.Project{
					{Name: "project-1", Description: "First project"},
					{Name: "project-2", Description: ""},
				}, nil
			},
			wantResults: []string{"project-1\tFirst project", "project-2"},
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name: "returns nil when tenant not set",
			args: []string{},
			toComplete: "",
			loadConfig: func() *izanami.Config {
				return &izanami.Config{
					PersonalAccessToken:         "test-token",
					PersonalAccessTokenUsername: "test-user",
					BaseURL:                     "http://localhost",
					Tenant:                      "", // No tenant
				}
			},
			listProjects: func(cfg *izanami.Config, ctx context.Context, tenant string) ([]izanami.Project, error) {
				t.Error("listProjects should not be called when tenant is empty")
				return nil, nil
			},
			wantResults: nil,
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name: "returns nil when args not empty",
			args: []string{"existing-arg"},
			toComplete: "",
			loadConfig: func() *izanami.Config {
				return &izanami.Config{
					PersonalAccessToken: "token",
					Tenant:              "tenant",
				}
			},
			listProjects: func(cfg *izanami.Config, ctx context.Context, tenant string) ([]izanami.Project, error) {
				t.Error("listProjects should not be called when args not empty")
				return nil, nil
			},
			wantResults: nil,
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name: "returns nil when config is nil",
			args: []string{},
			toComplete: "",
			loadConfig: func() *izanami.Config {
				return nil
			},
			listProjects: func(cfg *izanami.Config, ctx context.Context, tenant string) ([]izanami.Project, error) {
				t.Error("listProjects should not be called when config is nil")
				return nil, nil
			},
			wantResults: nil,
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name: "returns nil when no admin auth",
			args: []string{},
			toComplete: "",
			loadConfig: func() *izanami.Config {
				return &izanami.Config{
					BaseURL: "http://localhost",
					Tenant:  "my-tenant",
					// No PersonalAccessToken or JwtToken
				}
			},
			listProjects: func(cfg *izanami.Config, ctx context.Context, tenant string) ([]izanami.Project, error) {
				t.Error("listProjects should not be called when no admin auth")
				return nil, nil
			},
			wantResults: nil,
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name: "returns nil on API error",
			args: []string{},
			toComplete: "",
			loadConfig: func() *izanami.Config {
				return &izanami.Config{
					PersonalAccessToken:         "test-token",
					PersonalAccessTokenUsername: "test-user",
					BaseURL:                     "http://localhost",
					Tenant:                      "my-tenant",
				}
			},
			listProjects: func(cfg *izanami.Config, ctx context.Context, tenant string) ([]izanami.Project, error) {
				return nil, errors.New("API error")
			},
			wantResults: nil,
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name: "filters by prefix",
			args: []string{},
			toComplete: "project-1",
			loadConfig: func() *izanami.Config {
				return &izanami.Config{
					PersonalAccessToken:         "test-token",
					PersonalAccessTokenUsername: "test-user",
					BaseURL:                     "http://localhost",
					Tenant:                      "my-tenant",
				}
			},
			listProjects: func(cfg *izanami.Config, ctx context.Context, tenant string) ([]izanami.Project, error) {
				return []izanami.Project{
					{Name: "project-1", Description: "First"},
					{Name: "project-2", Description: "Second"},
				}, nil
			},
			wantResults: []string{"project-1\tFirst"},
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Completer{
				LoadConfig:   tt.loadConfig,
				ListProjects: tt.listProjects,
				Timeout:      completionTimeout,
			}

			got, directive := c.CompleteProjectNames(nil, tt.args, tt.toComplete)

			if directive != tt.wantDirective {
				t.Errorf("CompleteProjectNames() directive = %v, want %v", directive, tt.wantDirective)
			}

			if len(got) != len(tt.wantResults) {
				t.Errorf("CompleteProjectNames() returned %d results, want %d", len(got), len(tt.wantResults))
				return
			}

			for i, want := range tt.wantResults {
				if got[i] != want {
					t.Errorf("CompleteProjectNames()[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}
