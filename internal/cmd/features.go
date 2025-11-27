package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"github.com/webskin/izanami-go-cli/internal/output"
)

var (
	featureProject    string
	featureTag        string   // For filtering features list by tag
	featureTags       []string // For assigning tags to a feature when creating
	featureUser       string
	featureContextStr string
	featureName       string
	featureDesc       string
	featureEnabled    bool
	featureData       string
	// Client credentials (only used by check command)
	checkClientID     string
	checkClientSecret string
	// Bulk check parameters
	checkFeatures       []string
	checkProjects       []string
	checkConditions     bool
	checkDate           string
	checkOneTagIn       []string
	checkAllTagsIn      []string
	checkNoTagIn        []string
	featuresDeleteForce bool
)

// featuresCmd represents the admin features command
var featuresCmd = &cobra.Command{
	Use:   "features",
	Short: "Manage feature flags (admin)",
	Long: `Administrative operations for feature flags in Izanami.

These are admin operations that use the /api/admin/... endpoints and require
admin authentication (username and personal access token).

Features are the core of Izanami - they represent toggleable functionality
or configuration values that can be controlled dynamically.

Features can have:
- Different activation strategies (all users, percentage, specific users)
- Time-based activation (specific periods, days, hours)
- Context-specific overrides (different behavior per environment)
- Tags for organization

For client operations (checking if a feature is active), use:
  iz features check <feature-id>`,
}

// featuresListCmd lists features
var featuresListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all features",
	Long: `List all features in a tenant.

The list endpoint supports filtering by:
  --tag: Filter by tag (server-side filtering by Izanami API)
  --project: Filter by project (client-side filtering - Izanami API does not support this)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.Validate(); err != nil {
			return err
		}
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()

		// For JSON output, use Identity mapper to get raw JSON
		if outputFormat == "json" {
			raw, err := izanami.ListFeatures(client, ctx, cfg.Tenant, featureTag, izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseFeatures mapper
		features, err := izanami.ListFeatures(client, ctx, cfg.Tenant, featureTag, izanami.ParseFeatures)
		if err != nil {
			return err
		}

		// Client-side filtering by project
		// Note: Izanami API does not support project filtering on the list features endpoint,
		// so we filter the results here on the client side
		if featureProject != "" {
			filtered := make([]izanami.Feature, 0, len(features))
			for _, f := range features {
				if f.Project == featureProject {
					filtered = append(filtered, f)
				}
			}
			features = filtered
		}

		return output.Print(features, output.Format(outputFormat))
	},
}

// featuresGetCmd gets a specific feature
var featuresGetCmd = &cobra.Command{
	Use:   "get <feature-id>",
	Short: "Get a specific feature",
	Long:  `Get detailed information about a specific feature, including all context overloads.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.Validate(); err != nil {
			return err
		}
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()

		// For JSON output, use Identity mapper to get raw JSON
		if outputFormat == "json" {
			raw, err := izanami.GetFeature(client, ctx, cfg.Tenant, args[0], izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseFeature mapper
		feature, err := izanami.GetFeature(client, ctx, cfg.Tenant, args[0], izanami.ParseFeature)
		if err != nil {
			return err
		}

		return output.Print(feature, output.Format(outputFormat))
	},
}

// featuresCreateCmd creates a new feature
var featuresCreateCmd = &cobra.Command{
	Use:   "create <feature-name>",
	Short: "Create a new feature",
	Long: `Create a new feature flag.

You can provide the feature definition via:
  - Command-line flags (for simple features)
  - JSON data via --data flag
  - JSON data via stdin

Examples:
  # Create a simple boolean feature
  iz features create my-feature --project my-project --description "My feature" --enabled

  # Create from JSON file
  iz features create my-feature --project my-project --data @feature.json

  # Create from stdin
  cat feature.json | iz features create my-feature --project my-project --data -`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.Validate(); err != nil {
			return err
		}
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		// Determine project
		proj := featureProject
		if proj == "" {
			proj = cfg.Project
		}
		if proj == "" {
			return fmt.Errorf("project is required (use --project flag or IZ_PROJECT)")
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		featureName := args[0]
		var payload interface{}

		// Parse feature data
		if cmd.Flags().Changed("data") {
			if err := parseJSONData(featureData, &payload); err != nil {
				return err
			}
			// Merge required fields into the payload
			if payloadMap, ok := payload.(map[string]interface{}); ok {
				// Only set name if not already present in the JSON
				if _, hasName := payloadMap["name"]; !hasName {
					payloadMap["name"] = featureName
				}
				// Ensure resultType is set (default to boolean)
				if _, hasResultType := payloadMap["resultType"]; !hasResultType {
					payloadMap["resultType"] = "boolean"
				}
				// Ensure conditions is set (default to empty array)
				if _, hasConditions := payloadMap["conditions"]; !hasConditions {
					payloadMap["conditions"] = []interface{}{}
				}
				// Ensure description is set (default to empty string)
				if _, hasDescription := payloadMap["description"]; !hasDescription {
					payloadMap["description"] = ""
				}
				// Ensure metadata is set (default to empty object)
				if _, hasMetadata := payloadMap["metadata"]; !hasMetadata {
					payloadMap["metadata"] = map[string]interface{}{}
				}
				payload = payloadMap
			}
		} else {
			// Build simple feature from flags
			payload = map[string]interface{}{
				"name":        featureName,
				"description": featureDesc,
				"enabled":     featureEnabled,
				"resultType":  "boolean",
				"conditions":  []interface{}{},
				"metadata":    map[string]interface{}{},
			}
			if len(featureTags) > 0 {
				payload.(map[string]interface{})["tags"] = featureTags
			}
		}

		ctx := context.Background()
		created, err := client.CreateFeature(ctx, cfg.Tenant, proj, payload)
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Feature created successfully: %s\n", created.ID)
		return output.Print(created, output.Format(outputFormat))
	},
}

// featuresUpdateCmd updates an existing feature
var featuresUpdateCmd = &cobra.Command{
	Use:   "update <feature-id>",
	Short: "Update an existing feature",
	Long: `Update an existing feature flag.

Provide the updated feature definition via:
  - JSON data via --data flag
  - JSON data via stdin

Examples:
  # Update from JSON file
  iz features update my-feature --data @feature.json

  # Update from stdin
  cat feature.json | iz features update my-feature --data -`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.Validate(); err != nil {
			return err
		}
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		if !cmd.Flags().Changed("data") {
			return fmt.Errorf("feature data is required (use --data flag)")
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		var updateData interface{}
		if err := parseJSONData(featureData, &updateData); err != nil {
			return err
		}

		featureID := args[0]

		// Determine project from flags or config
		proj := featureProject
		if proj == "" {
			proj = cfg.Project
		}

		// Merge required fields into the payload
		if updateMap, ok := updateData.(map[string]interface{}); ok {
			// Set the ID field if not already present
			if _, hasID := updateMap["id"]; !hasID {
				updateMap["id"] = featureID
			}
			// Set the project field if not already present and we have a project
			if proj != "" {
				if _, hasProject := updateMap["project"]; !hasProject {
					updateMap["project"] = proj
				}
			}
			updateData = updateMap
		}

		// Validate that required fields are present
		if updateMap, ok := updateData.(map[string]interface{}); ok {
			missingFields := []string{}
			if _, hasName := updateMap["name"]; !hasName {
				missingFields = append(missingFields, "name")
			}
			if _, hasProject := updateMap["project"]; !hasProject {
				missingFields = append(missingFields, "project")
			}
			if _, hasResultType := updateMap["resultType"]; !hasResultType {
				missingFields = append(missingFields, "resultType")
			}
			if _, hasConditions := updateMap["conditions"]; !hasConditions {
				missingFields = append(missingFields, "conditions")
			}
			if _, hasDescription := updateMap["description"]; !hasDescription {
				missingFields = append(missingFields, "description")
			}
			if _, hasMetadata := updateMap["metadata"]; !hasMetadata {
				missingFields = append(missingFields, "metadata")
			}

			if len(missingFields) > 0 {
				// Fetch current feature to show the user
				ctx := context.Background()
				currentFeature, err := izanami.GetFeature(client, ctx, cfg.Tenant, featureID, izanami.ParseFeature)
				if err != nil {
					return fmt.Errorf("missing required fields (%v) and failed to fetch current feature: %w", missingFields, err)
				}

				// Print current feature structure
				fmt.Fprintf(cmd.OutOrStderr(), "❌ Missing required fields: %v\n\n", missingFields)
				fmt.Fprintf(cmd.OutOrStderr(), "Current feature structure:\n")
				output.Print(currentFeature, output.JSON)
				fmt.Fprintf(cmd.OutOrStderr(), "\nPlease include all required fields in your update.\n")
				return fmt.Errorf("missing required fields: %v", missingFields)
			}
		}

		ctx := context.Background()
		if err := client.UpdateFeature(ctx, cfg.Tenant, featureID, updateData, false); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Feature updated successfully: %s\n", featureID)
		return nil
	},
}

// featuresDeleteCmd deletes a feature
var featuresDeleteCmd = &cobra.Command{
	Use:   "delete <feature-id>",
	Short: "Delete a feature",
	Long:  `Delete a feature flag. This operation cannot be undone.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.Validate(); err != nil {
			return err
		}
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		featureID := args[0]

		// Confirm deletion unless --force is used
		if !featuresDeleteForce {
			if !confirmDeletion(cmd, "feature", featureID) {
				return nil
			}
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.DeleteFeature(ctx, cfg.Tenant, featureID); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Feature deleted successfully: %s\n", featureID)
		return nil
	},
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
    - --project flag is optional (helps disambiguate if multiple features have same name)
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
		// Build projects list for credential resolution
		var projects []string
		if cfg.Project != "" {
			projects = append(projects, cfg.Project)
		}
		if featureProject != "" && featureProject != cfg.Project {
			projects = append(projects, featureProject)
		}

		resolveClientCredentials(cmd, cfg, checkClientID, checkClientSecret, projects)

		if err := cfg.Validate(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		featureIDOrName := args[0]
		var featureID string

		// Determine if input is a UUID or name
		if isUUID(featureIDOrName) {
			// UUID mode: use directly
			featureID = featureIDOrName
			if cfg.Verbose {
				fmt.Fprintf(cmd.OutOrStderr(), "Using feature UUID: %s\n", featureID)
			}
		} else {
			if err := cfg.ValidateTenant(); err != nil {
				return fmt.Errorf("feature name requires --tenant flag: %w", err)
			}

			if cfg.Verbose {
				fmt.Fprintf(cmd.OutOrStderr(), "Resolving feature name '%s' in tenant '%s'...\n", featureIDOrName, cfg.Tenant)
			}

			// List all features for the tenant
			features, err := izanami.ListFeatures(client, ctx, cfg.Tenant, "", izanami.ParseFeatures)
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

			// Further filter by project if specified
			if featureProject != "" {
				var projectMatches []izanami.Feature
				for _, f := range matches {
					if f.Project == featureProject {
						projectMatches = append(projectMatches, f)
					}
				}
				matches = projectMatches
			}

			// Validate matches
			if len(matches) == 0 {
				if featureProject != "" {
					return fmt.Errorf("no feature named '%s' found in tenant '%s' and project '%s'", featureIDOrName, cfg.Tenant, featureProject)
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

		result, err := client.CheckFeature(ctx, featureID, featureUser, contextPath, payload)
		if err != nil {
			return err
		}

		// Populate tenant and id fields (not returned by the API)
		result.Tenant = cfg.Tenant
		result.ID = featureID

		return output.Print(result, output.Format(outputFormat))
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

		resolveClientCredentials(cmd, cfg, checkClientID, checkClientSecret, projects)

		if err := cfg.Validate(); err != nil {
			return err
		}

		// Validate that at least one filter is provided
		if len(checkFeatures) == 0 && len(checkProjects) == 0 {
			return fmt.Errorf("at least one filter is required: --features or --projects")
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()

		// Resolve tag names to UUIDs if needed
		resolvedOneTagIn, err := resolveTagNames(ctx, client, cfg.Tenant, checkOneTagIn)
		if err != nil {
			return err
		}
		resolvedAllTagsIn, err := resolveTagNames(ctx, client, cfg.Tenant, checkAllTagsIn)
		if err != nil {
			return err
		}
		resolvedNoTagIn, err := resolveTagNames(ctx, client, cfg.Tenant, checkNoTagIn)
		if err != nil {
			return err
		}

		// Resolve project names to UUIDs if needed
		resolvedProjects := make([]string, 0, len(checkProjects))
		var projectsToResolve []string

		for _, projectIDOrName := range checkProjects {
			if isUUID(projectIDOrName) {
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
			projects, err := izanami.ListProjects(client, ctx, cfg.Tenant, izanami.ParseProjects)
			if err != nil {
				return fmt.Errorf("failed to list projects: %w", err)
			}

			// Build name->ID map
			nameToID := make(map[string]string)
			for _, p := range projects {
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
			if isUUID(featureIDOrName) {
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
			allFeatures, err := izanami.ListFeatures(client, ctx, cfg.Tenant, "", izanami.ParseFeatures)
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

		results, err := client.CheckFeatures(ctx, request)
		if err != nil {
			return err
		}

		// For table format, convert to table view
		format := output.Format(outputFormat)
		if format == output.Table {
			tableView := results.ToTableView()
			return output.Print(tableView, format)
		}

		return output.Print(results, format)
	},
}

// resolveTagNames resolves tag names to UUIDs
// Supports mixing UUIDs and names; requires tenant for name resolution
// Uses the dedicated GET /api/admin/tenants/:tenant/tags/:name endpoint for individual lookups
func resolveTagNames(ctx context.Context, client *izanami.Client, tenant string, tags []string) ([]string, error) {
	if len(tags) == 0 {
		return nil, nil
	}

	resolved := make([]string, 0, len(tags))

	for _, tag := range tags {
		if isUUID(tag) {
			// Already a UUID, use as-is
			resolved = append(resolved, tag)
		} else {
			// Not a UUID, need to resolve using dedicated endpoint
			if tenant == "" {
				return nil, fmt.Errorf("tag names require --tenant flag")
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
// 1. Command-specific flags (--client-id/--client-secret)
// 2. Environment variables (IZ_CLIENT_ID/IZ_CLIENT_SECRET) - already in cfg via viper
// 3. Config file (client-keys section) - fallback if both are empty
func resolveClientCredentials(cmd *cobra.Command, cfg *izanami.Config, flagClientID, flagClientSecret string, projects []string) {
	// First, apply command-specific flags if provided
	if flagClientID != "" {
		cfg.ClientID = flagClientID
	}
	if flagClientSecret != "" {
		cfg.ClientSecret = flagClientSecret
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

// isUUID checks if a string matches the UUID format (8-4-4-4-12)
func isUUID(s string) bool {
	uuidPattern := `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`
	matched, _ := regexp.MatchString(uuidPattern, s)
	return matched
}

// ensureLeadingSlash adds a leading slash to the context path if it's not empty and doesn't have one
func ensureLeadingSlash(context string) string {
	if context != "" && context[0] != '/' {
		return "/" + context
	}
	return context
}

// parseJSONData parses JSON data from a file, stdin, or string
func parseJSONData(dataStr string, target interface{}) error {
	var data []byte
	var err error

	if dataStr == "-" {
		// Read from stdin
		data, err = os.ReadFile("/dev/stdin")
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
	} else if len(dataStr) > 0 && dataStr[0] == '@' {
		// Read from file
		data, err = os.ReadFile(dataStr[1:])
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", dataStr[1:], err)
		}
	} else {
		// Use string directly
		data = []byte(dataStr)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	return nil
}

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

func init() {
	// Register admin features commands (added in admin.go init)
	// Register root-level features command for client operations
	rootCmd.AddCommand(rootFeaturesCmd)
	rootFeaturesCmd.AddCommand(featuresCheckCmd)
	rootFeaturesCmd.AddCommand(featuresCheckBulkCmd)

	// List flags
	featuresListCmd.Flags().StringVar(&featureTag, "tag", "", "Filter by tag (server-side)")
	featuresListCmd.Flags().StringVar(&featureProject, "project", "", "Filter by project (client-side)")

	// Create flags
	featuresCreateCmd.Flags().StringVar(&featureProject, "project", "", "Project for the feature (required)")
	featuresCreateCmd.Flags().StringVar(&featureDesc, "description", "", "Feature description")
	featuresCreateCmd.Flags().BoolVar(&featureEnabled, "enabled", false, "Enable the feature")
	featuresCreateCmd.Flags().StringSliceVar(&featureTags, "tags", []string{}, "Feature tags")
	featuresCreateCmd.Flags().StringVar(&featureData, "data", "", "JSON feature data (from file with @file.json, stdin with -, or inline)")

	// Update flags
	featuresUpdateCmd.Flags().StringVar(&featureData, "data", "", "JSON feature data (from file with @file.json, stdin with -, or inline)")
	featuresUpdateCmd.MarkFlagRequired("data")

	// Delete flags
	featuresDeleteCmd.Flags().BoolVarP(&featuresDeleteForce, "force", "f", false, "Skip confirmation prompt")

	// Check flags
	featuresCheckCmd.Flags().StringVar(&featureUser, "user", "", "User ID for evaluation")
	featuresCheckCmd.Flags().StringVar(&featureContextStr, "context", "", "Context path for evaluation")
	featuresCheckCmd.Flags().StringVar(&featureProject, "project", "", "Project name (for disambiguating feature names)")
	featuresCheckCmd.Flags().StringVar(&checkClientID, "client-id", "", "Client ID for authentication (env: IZ_CLIENT_ID)")
	featuresCheckCmd.Flags().StringVar(&checkClientSecret, "client-secret", "", "Client secret for authentication (env: IZ_CLIENT_SECRET)")
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
	featuresCheckBulkCmd.Flags().StringVar(&featureData, "data", "", "JSON payload for script features (from file with @file.json, stdin with -, or inline)")
}
