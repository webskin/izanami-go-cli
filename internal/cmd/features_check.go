package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"github.com/webskin/izanami-go-cli/internal/output"
)

var (
	// Client credentials (only used by check command)
	checkClientID      string
	checkClientSecret  string
	checkClientBaseURL string
	// Bulk check parameters
	checkFeatures   []string
	checkProjects   []string
	checkConditions bool
	checkDate       string
	checkOneTagIn   []string
	checkAllTagsIn  []string
	checkNoTagIn    []string
)

// Root-level features command for client operations
var rootFeaturesCmd = &cobra.Command{
	Use:   "features",
	Short: "Client feature operations",
	Long: `Client-facing feature operations using the /api/v2/features endpoint.

This command provides client operations that don't require admin privileges,
such as checking if a feature is active for a specific user or context.

For administrative operations (create, update, delete, list), use:
  iz admin features <command>`,
}

// featuresCheckCmd checks if a feature is active
var featuresCheckCmd = &cobra.Command{
	Use:   "check <uuid-or-name>",
	Short: "Check if a feature is active",
	Long: `Check if a feature is active for a specific user and context.

This command accepts either a feature UUID or feature name:

  UUID mode:
    - Provide a UUID (e.g., e878a149-df86-4f28-b1db-059580304e1e)
    - --tenant and --project flags are optional (UUID is globally unique)

  Name mode:
    - Provide a feature name (e.g., my-feature)
    - --tenant flag is REQUIRED
    - --project flag is optional (helps disambiguate if multiple features have same name, use global --project flag)
    - If multiple features match, an error is returned

This uses the client API (v2) to evaluate the feature, taking into account:
- Feature enabled status
- Activation conditions (user targeting, percentages)
- Time-based activation
- Context-specific overrides

Script Features:
  For script-based features, you can provide a JSON payload via --data:
    iz features check <uuid> --user user123 --data '{"customField": "value"}'
    iz features check <uuid> --user user123 --data @payload.json
  This will use POST /api/v2/features/{id} instead of GET.

Examples:
  # Check feature by UUID
  iz features check e878a149-df86-4f28-b1db-059580304e1e --user user123

  # Check feature by name (requires --tenant)
  iz features check my-feature --tenant my-tenant --user user123

  # Check feature by name with project disambiguation
  iz features check my-feature --tenant my-tenant --project my-project --user user123

  # Check script feature with payload
  iz features check e878a149-df86-4f28-b1db-059580304e1e --data '{"age": 25}'`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Build projects list for credential resolution (uses global --project flag)
		var projects []string
		if cfg.Project != "" {
			projects = append(projects, cfg.Project)
		}

		resolveClientCredentials(cmd, cfg, checkClientID, checkClientSecret, checkClientBaseURL, projects)

		ctx := context.Background()
		featureIDOrName := args[0]
		var featureID string

		// Determine if input is a UUID or name
		if IsUUID(featureIDOrName) {
			// UUID mode: use directly
			featureID = featureIDOrName
			if cfg.Verbose {
				fmt.Fprintf(cmd.OutOrStderr(), "Using feature UUID: %s\n", featureID)
			}
		} else {
			// Name mode: need admin client to resolve feature name
			if err := cfg.ValidateTenant(); err != nil {
				return fmt.Errorf("feature name requires --tenant flag: %w", err)
			}

			if cfg.Verbose {
				fmt.Fprintf(cmd.OutOrStderr(), "Resolving feature name '%s' in tenant '%s'...\n", featureIDOrName, cfg.Tenant)
			}

			// Create admin client for name resolution (uses BaseURL)
			adminClient, err := izanami.NewAdminClient(cfg)
			if err != nil {
				return fmt.Errorf("failed to create admin client for name resolution: %w", err)
			}

			// List all features for the tenant
			features, err := izanami.ListFeatures(adminClient, ctx, cfg.Tenant, "", izanami.ParseFeatures)
			if err != nil {
				return fmt.Errorf("failed to list features: %w", err)
			}

			// Filter by name
			var matches []izanami.Feature
			for _, f := range features {
				if f.Name == featureIDOrName {
					matches = append(matches, f)
				}
			}

			// Further filter by project if specified (uses global --project flag)
			if cfg.Project != "" {
				var projectMatches []izanami.Feature
				for _, f := range matches {
					if f.Project == cfg.Project {
						projectMatches = append(projectMatches, f)
					}
				}
				matches = projectMatches
			}

			// Validate matches
			if len(matches) == 0 {
				if cfg.Project != "" {
					return fmt.Errorf("no feature named '%s' found in tenant '%s' and project '%s'", featureIDOrName, cfg.Tenant, cfg.Project)
				}
				return fmt.Errorf("no feature named '%s' found in tenant '%s'", featureIDOrName, cfg.Tenant)
			}
			if len(matches) > 1 {
				return fmt.Errorf("multiple features named '%s' found (use --project to disambiguate or provide UUID instead)", featureIDOrName)
			}

			// Use the resolved UUID
			featureID = matches[0].ID
			if cfg.Verbose {
				fmt.Fprintf(cmd.OutOrStderr(), "Found feature ID: %s\n", featureID)
			}
		}

		// Create feature check client (uses ClientBaseURL if set, otherwise BaseURL)
		checkClient, err := izanami.NewFeatureCheckClient(cfg)
		if err != nil {
			return err
		}

		// Use context from flag or config
		contextPath := featureContextStr
		if contextPath == "" {
			contextPath = cfg.Context
		}
		// Ensure context has leading slash if specified
		contextPath = ensureLeadingSlash(contextPath)

		// Parse payload if provided
		var payload string
		if featureData != "" {
			var payloadData interface{}
			if err := parseJSONData(featureData, &payloadData); err != nil {
				return fmt.Errorf("invalid JSON payload: %w", err)
			}
			// Convert back to JSON string for the API call
			payloadBytes, err := json.Marshal(payloadData)
			if err != nil {
				return fmt.Errorf("failed to serialize payload: %w", err)
			}
			payload = string(payloadBytes)
		}

		// For JSON output, use Identity mapper for raw JSON
		if outputFormat == "json" {
			raw, err := izanami.CheckFeature(checkClient, ctx, featureID, featureUser, contextPath, payload, izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseFeatureCheckResult mapper
		result, err := izanami.CheckFeature(checkClient, ctx, featureID, featureUser, contextPath, payload, izanami.ParseFeatureCheckResult)
		if err != nil {
			return err
		}

		// Populate tenant and id fields (not returned by the API)
		result.Tenant = cfg.Tenant
		result.ID = featureID

		return output.PrintTo(cmd.OutOrStdout(), result, output.Format(outputFormat))
	},
}

// featuresCheckBulkCmd checks multiple features in a single request
var featuresCheckBulkCmd = &cobra.Command{
	Use:   "check-bulk",
	Short: "Check multiple features at once",
	Long: `Check activation status for multiple features in a single request.

This command uses the GET/POST /api/v2/features endpoint to check multiple
features simultaneously. You can filter features by:
  - Specific feature IDs or names (--features)
  - Project IDs or names (--projects) - checks all features in those projects
  - Tags (--one-tag-in, --all-tags-in, --no-tag-in)

The --features and --projects flags accept both UUIDs and names:
  - UUIDs are used directly (no tenant required)
  - Names are resolved to UUIDs (requires --tenant flag)
  - If a feature name matches multiple features, use --projects to disambiguate

Optionally, you can request activation conditions (--conditions) which allows
offline re-evaluation of features without another API call.

Script Features:
  For script-based features, provide a JSON payload via --data to use POST method.

Examples:
  # Check specific features by UUID
  iz features check-bulk --features feat1-uuid,feat2-uuid --user user123

  # Check features by name (requires --tenant)
  iz features check-bulk --features my-feature,other-feature --tenant my-tenant --user user123

  # Mix UUIDs and names
  iz features check-bulk --features feat1-uuid,my-feature --tenant my-tenant --user user123

  # Check all features in specific projects by UUID
  iz features check-bulk --projects proj1-uuid,proj2-uuid --user user123

  # Check all features in specific projects by name (requires --tenant)
  iz features check-bulk --projects test-project,prod-project --tenant my-tenant --user user123

  # Check features with tag filtering
  iz features check-bulk --projects test-project --tenant my-tenant --one-tag-in beta,experimental

  # Get conditions for offline evaluation
  iz features check-bulk --features feat1-uuid --conditions --user user123

  # Check script features with payload
  iz features check-bulk --features feat1-uuid --data '{"age": 25}'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Build projects list for credential resolution
		var projects []string
		if len(checkProjects) > 0 {
			projects = append(projects, checkProjects...)
		}
		if cfg.Project != "" {
			projects = append(projects, cfg.Project)
		}

		resolveClientCredentials(cmd, cfg, checkClientID, checkClientSecret, checkClientBaseURL, projects)

		// Validate that at least one filter is provided
		if len(checkFeatures) == 0 && len(checkProjects) == 0 {
			return fmt.Errorf("at least one filter is required: --features or --projects")
		}

		ctx := context.Background()

		// Check if we need admin client for name resolution
		needsAdminClient := false
		for _, f := range checkFeatures {
			if !IsUUID(f) {
				needsAdminClient = true
				break
			}
		}
		if !needsAdminClient {
			for _, p := range checkProjects {
				if !IsUUID(p) {
					needsAdminClient = true
					break
				}
			}
		}
		if !needsAdminClient {
			for _, t := range checkOneTagIn {
				if !IsUUID(t) {
					needsAdminClient = true
					break
				}
			}
		}
		if !needsAdminClient {
			for _, t := range checkAllTagsIn {
				if !IsUUID(t) {
					needsAdminClient = true
					break
				}
			}
		}
		if !needsAdminClient {
			for _, t := range checkNoTagIn {
				if !IsUUID(t) {
					needsAdminClient = true
					break
				}
			}
		}

		var adminClient *izanami.AdminClient
		if needsAdminClient {
			var err error
			adminClient, err = izanami.NewAdminClient(cfg)
			if err != nil {
				return fmt.Errorf("failed to create admin client for name resolution: %w", err)
			}
		}

		// Resolve tag names to UUIDs if needed
		resolvedOneTagIn, err := resolveTagNames(ctx, adminClient, cfg.Tenant, checkOneTagIn)
		if err != nil {
			return err
		}
		resolvedAllTagsIn, err := resolveTagNames(ctx, adminClient, cfg.Tenant, checkAllTagsIn)
		if err != nil {
			return err
		}
		resolvedNoTagIn, err := resolveTagNames(ctx, adminClient, cfg.Tenant, checkNoTagIn)
		if err != nil {
			return err
		}

		// Resolve project names to UUIDs if needed
		resolvedProjects := make([]string, 0, len(checkProjects))
		var projectsToResolve []string

		for _, projectIDOrName := range checkProjects {
			if IsUUID(projectIDOrName) {
				// Already a UUID, use as-is
				resolvedProjects = append(resolvedProjects, projectIDOrName)
			} else {
				// Not a UUID, need to resolve
				projectsToResolve = append(projectsToResolve, projectIDOrName)
			}
		}

		// If we have project names to resolve, fetch all projects and map them
		if len(projectsToResolve) > 0 {
			if err := cfg.ValidateTenant(); err != nil {
				return fmt.Errorf("project names require --tenant flag: %w", err)
			}

			if cfg.Verbose {
				fmt.Fprintf(cmd.OutOrStderr(), "Resolving project names %v in tenant '%s'...\n", projectsToResolve, cfg.Tenant)
			}

			// List all projects for the tenant
			projectsList, err := izanami.ListProjects(adminClient, ctx, cfg.Tenant, izanami.ParseProjects)
			if err != nil {
				return fmt.Errorf("failed to list projects: %w", err)
			}

			// Build name->ID map
			nameToID := make(map[string]string)
			for _, p := range projectsList {
				nameToID[p.Name] = p.ID
			}

			// Resolve each name
			for _, name := range projectsToResolve {
				if id, found := nameToID[name]; found {
					resolvedProjects = append(resolvedProjects, id)
					if cfg.Verbose {
						fmt.Fprintf(cmd.OutOrStderr(), "Resolved project '%s' to ID: %s\n", name, id)
					}
				} else {
					return fmt.Errorf("no project named '%s' found in tenant '%s'", name, cfg.Tenant)
				}
			}
		}

		// Resolve feature names to UUIDs if needed
		// If any feature in checkFeatures is not a UUID, we need tenant to resolve it
		resolvedFeatures := make([]string, 0, len(checkFeatures))
		var featuresToResolve []string

		for _, featureIDOrName := range checkFeatures {
			if IsUUID(featureIDOrName) {
				// Already a UUID, use as-is
				resolvedFeatures = append(resolvedFeatures, featureIDOrName)
			} else {
				// Not a UUID, need to resolve
				featuresToResolve = append(featuresToResolve, featureIDOrName)
			}
		}

		// If we have names to resolve, fetch all features and map them
		if len(featuresToResolve) > 0 {
			if err := cfg.ValidateTenant(); err != nil {
				return fmt.Errorf("feature names require --tenant flag: %w", err)
			}

			if cfg.Verbose {
				fmt.Fprintf(cmd.OutOrStderr(), "Resolving feature names %v in tenant '%s'...\n", featuresToResolve, cfg.Tenant)
			}

			// List all features for the tenant
			allFeatures, err := izanami.ListFeatures(adminClient, ctx, cfg.Tenant, "", izanami.ParseFeatures)
			if err != nil {
				return fmt.Errorf("failed to list features: %w", err)
			}

			// Build name->ID map
			nameToID := make(map[string][]izanami.Feature)
			for _, f := range allFeatures {
				nameToID[f.Name] = append(nameToID[f.Name], f)
			}

			// Resolve each name
			for _, name := range featuresToResolve {
				matches := nameToID[name]

				// Further filter by project if projects are specified (use resolved project UUIDs)
				if len(resolvedProjects) > 0 {
					var projectMatches []izanami.Feature
					projectSet := make(map[string]bool)
					for _, p := range resolvedProjects {
						projectSet[p] = true
					}
					for _, f := range matches {
						if projectSet[f.Project] {
							projectMatches = append(projectMatches, f)
						}
					}
					matches = projectMatches
				}

				// Validate matches
				if len(matches) == 0 {
					if len(resolvedProjects) > 0 {
						return fmt.Errorf("no feature named '%s' found in tenant '%s' within specified projects %v", name, cfg.Tenant, resolvedProjects)
					}
					return fmt.Errorf("no feature named '%s' found in tenant '%s'", name, cfg.Tenant)
				}
				if len(matches) > 1 {
					return fmt.Errorf("multiple features named '%s' found (use --projects to disambiguate or provide UUID instead)", name)
				}

				// Use the resolved UUID
				resolvedFeatures = append(resolvedFeatures, matches[0].ID)
				if cfg.Verbose {
					fmt.Fprintf(cmd.OutOrStderr(), "Resolved feature '%s' to ID: %s\n", name, matches[0].ID)
				}
			}
		}

		// Create feature check client (uses ClientBaseURL if set, otherwise BaseURL)
		checkClient, err := izanami.NewFeatureCheckClient(cfg)
		if err != nil {
			return err
		}

		// Use context from flag or config
		contextPath := featureContextStr
		if contextPath == "" {
			contextPath = cfg.Context
		}
		// Ensure context has leading slash if specified
		contextPath = ensureLeadingSlash(contextPath)

		// Parse payload if provided
		var payload string
		if featureData != "" {
			var payloadData interface{}
			if err := parseJSONData(featureData, &payloadData); err != nil {
				return fmt.Errorf("invalid JSON payload: %w", err)
			}
			payloadBytes, err := json.Marshal(payloadData)
			if err != nil {
				return fmt.Errorf("failed to serialize payload: %w", err)
			}
			payload = string(payloadBytes)
		}

		// Build request with resolved UUIDs (features, projects, and tags)
		request := izanami.CheckFeaturesRequest{
			User:       featureUser,
			Context:    contextPath,
			Features:   resolvedFeatures,
			Projects:   resolvedProjects,
			Conditions: checkConditions,
			Date:       checkDate,
			OneTagIn:   resolvedOneTagIn,
			AllTagsIn:  resolvedAllTagsIn,
			NoTagIn:    resolvedNoTagIn,
			Payload:    payload,
		}

		// For JSON output, use Identity mapper for raw JSON
		if outputFormat == "json" {
			raw, err := izanami.CheckFeatures(checkClient, ctx, request, izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseActivationsWithConditions mapper
		results, err := izanami.CheckFeatures(checkClient, ctx, request, izanami.ParseActivationsWithConditions)
		if err != nil {
			return err
		}

		// Convert to table view for table format
		tableView := results.ToTableView()
		return output.PrintTo(cmd.OutOrStdout(), tableView, output.Table)
	},
}

// resolveTagNames resolves tag names to UUIDs
// Supports mixing UUIDs and names; requires tenant for name resolution
// Uses the dedicated GET /api/admin/tenants/:tenant/tags/:name endpoint for individual lookups
func resolveTagNames(ctx context.Context, client *izanami.AdminClient, tenant string, tags []string) ([]string, error) {
	if len(tags) == 0 {
		return nil, nil
	}

	resolved := make([]string, 0, len(tags))

	for _, tag := range tags {
		if IsUUID(tag) {
			// Already a UUID, use as-is
			resolved = append(resolved, tag)
		} else {
			// Not a UUID, need to resolve using dedicated endpoint
			if tenant == "" {
				return nil, fmt.Errorf("tag names require --tenant flag")
			}
			if client == nil {
				return nil, fmt.Errorf("admin client required for tag name resolution")
			}

			tagObj, err := izanami.GetTag(client, ctx, tenant, tag, izanami.ParseTag)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve tag '%s': %w", tag, err)
			}

			resolved = append(resolved, tagObj.ID)
		}
	}

	return resolved, nil
}

// resolveClientCredentials resolves client credentials with 3-tier precedence:
// 1. Command-specific flags (--client-id/--client-secret/--client-base-url)
// 2. Environment variables (IZ_CLIENT_ID/IZ_CLIENT_SECRET/IZ_CLIENT_BASE_URL) - already in cfg via viper
// 3. Config file (client-keys section) - fallback if both are empty
// Also resolves ClientBaseURL from client-keys if not already set.
func resolveClientCredentials(cmd *cobra.Command, cfg *izanami.Config, flagClientID, flagClientSecret, flagClientBaseURL string, projects []string) {
	// First, apply command-specific flags if provided
	if flagClientID != "" {
		cfg.ClientID = flagClientID
	}
	if flagClientSecret != "" {
		cfg.ClientSecret = flagClientSecret
	}
	if flagClientBaseURL != "" {
		cfg.ClientBaseURL = flagClientBaseURL
	}

	// If still empty, try to resolve from client-keys in config
	if cfg.ClientID == "" && cfg.ClientSecret == "" {
		tenant := cfg.Tenant

		clientID, clientSecret := cfg.ResolveClientCredentials(tenant, projects)
		if clientID != "" && clientSecret != "" {
			cfg.ClientID = clientID
			cfg.ClientSecret = clientSecret
			if cfg.Verbose {
				if len(projects) > 0 {
					fmt.Fprintf(cmd.OutOrStderr(), "Using client credentials from config (tenant: %s, projects: %v)\n", tenant, projects)
				} else {
					fmt.Fprintf(cmd.OutOrStderr(), "Using client credentials from config (tenant: %s)\n", tenant)
				}
			}
		}
	}
}

func init() {
	// Register root-level features command for client operations
	rootCmd.AddCommand(rootFeaturesCmd)
	rootFeaturesCmd.AddCommand(featuresCheckCmd)
	rootFeaturesCmd.AddCommand(featuresCheckBulkCmd)

	// Check flags
	featuresCheckCmd.Flags().StringVar(&featureUser, "user", "", "User ID for evaluation")
	featuresCheckCmd.Flags().StringVar(&featureContextStr, "context", "", "Context path for evaluation")
	// Project disambiguation uses global --project flag
	featuresCheckCmd.Flags().StringVar(&checkClientID, "client-id", "", "Client ID for authentication (env: IZ_CLIENT_ID)")
	featuresCheckCmd.Flags().StringVar(&checkClientSecret, "client-secret", "", "Client secret for authentication (env: IZ_CLIENT_SECRET)")
	featuresCheckCmd.Flags().StringVar(&checkClientBaseURL, "client-base-url", "", "Base URL for client operations (env: IZ_CLIENT_BASE_URL)")
	featuresCheckCmd.Flags().StringVar(&featureData, "data", "", "JSON payload for script features (from file with @file.json, stdin with -, or inline)")

	// Bulk check flags
	featuresCheckBulkCmd.Flags().StringVar(&featureUser, "user", "", "User ID for evaluation")
	featuresCheckBulkCmd.Flags().StringVar(&featureContextStr, "context", "", "Context path for evaluation")
	featuresCheckBulkCmd.Flags().StringSliceVar(&checkFeatures, "features", []string{}, "Feature IDs or names to check (comma-separated, names require --tenant)")
	featuresCheckBulkCmd.Flags().StringSliceVar(&checkProjects, "projects", []string{}, "Project IDs or names to check all features from (comma-separated, names require --tenant)")
	featuresCheckBulkCmd.Flags().BoolVar(&checkConditions, "conditions", false, "Return activation conditions alongside results")
	featuresCheckBulkCmd.Flags().StringVar(&checkDate, "date", "", "Date for evaluation (ISO 8601 format)")
	featuresCheckBulkCmd.Flags().StringSliceVar(&checkOneTagIn, "one-tag-in", []string{}, "Tag IDs or names - features must have at least one of these tags (comma-separated, names require --tenant)")
	featuresCheckBulkCmd.Flags().StringSliceVar(&checkAllTagsIn, "all-tags-in", []string{}, "Tag IDs or names - features must have all of these tags (comma-separated, names require --tenant)")
	featuresCheckBulkCmd.Flags().StringSliceVar(&checkNoTagIn, "no-tag-in", []string{}, "Tag IDs or names - features must not have any of these tags (comma-separated, names require --tenant)")
	featuresCheckBulkCmd.Flags().StringVar(&checkClientID, "client-id", "", "Client ID for authentication (env: IZ_CLIENT_ID)")
	featuresCheckBulkCmd.Flags().StringVar(&checkClientSecret, "client-secret", "", "Client secret for authentication (env: IZ_CLIENT_SECRET)")
	featuresCheckBulkCmd.Flags().StringVar(&checkClientBaseURL, "client-base-url", "", "Base URL for client operations (env: IZ_CLIENT_BASE_URL)")
	featuresCheckBulkCmd.Flags().StringVar(&featureData, "data", "", "JSON payload for script features (from file with @file.json, stdin with -, or inline)")
}
