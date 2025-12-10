package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"github.com/webskin/izanami-go-cli/internal/output"
)

var (
	featureTag          string   // For filtering features list by tag
	featureTags         []string // For assigning tags to a feature when creating
	featureName         string
	featureDesc         string
	featureEnabled      bool
	featuresDeleteForce bool

	// Test command flags
	featureTestDate      string   // Date for feature evaluation (ISO 8601)
	featureTestFeatures  []string // Feature IDs for bulk testing
	featureTestProjects  []string // Project IDs for bulk testing
	featureTestOneTagIn  []string // Tag filter: at least one must match
	featureTestAllTagsIn []string // Tag filter: all must match
	featureTestNoTagIn   []string // Tag filter: none can match
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
	Use:         "list",
	Short:       "List all features",
	Annotations: map[string]string{"route": "GET /api/admin/tenants/:tenant/features"},
	Long: `List all features in a tenant.

The list endpoint supports filtering by:
  --tag: Filter by tag (server-side filtering by Izanami API)
  --project: Filter by project (client-side filtering, use global --project flag)`,
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

		// Client-side filtering by project (uses global --project flag)
		// Note: Izanami API does not support project filtering on the list features endpoint,
		// so we filter the results here on the client side
		if cfg.Project != "" {
			filtered := make([]izanami.Feature, 0, len(features))
			for _, f := range features {
				if f.Project == cfg.Project {
					filtered = append(filtered, f)
				}
			}
			features = filtered
		}

		return output.PrintTo(cmd.OutOrStdout(), features, output.Format(outputFormat))
	},
}

// featuresGetCmd gets a specific feature
var featuresGetCmd = &cobra.Command{
	Use:         "get <feature-id>",
	Short:       "Get a specific feature",
	Annotations: map[string]string{"route": "GET /api/admin/tenants/:tenant/features/:id"},
	Long:        `Get detailed information about a specific feature, including all context overloads.`,
	Args:        cobra.ExactArgs(1),
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

		return output.PrintTo(cmd.OutOrStdout(), feature, output.Format(outputFormat))
	},
}

// featuresCreateCmd creates a new feature
var featuresCreateCmd = &cobra.Command{
	Use:         "create <feature-name>",
	Short:       "Create a new feature",
	Annotations: map[string]string{"route": "POST /api/admin/tenants/:tenant/projects/:project/features"},
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

		// Project is required for creating features (use global --project flag)
		if cfg.Project == "" {
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
		created, err := client.CreateFeature(ctx, cfg.Tenant, cfg.Project, payload)
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Feature created successfully: %s\n", created.ID)
		return output.PrintTo(cmd.OutOrStdout(), created, output.Format(outputFormat))
	},
}

// featuresUpdateCmd updates an existing feature
var featuresUpdateCmd = &cobra.Command{
	Use:         "update <feature-id>",
	Short:       "Update an existing feature",
	Annotations: map[string]string{"route": "PUT /api/admin/tenants/:tenant/features/:id"},
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

		// Merge required fields into the payload
		if updateMap, ok := updateData.(map[string]interface{}); ok {
			// Set the ID field if not already present
			if _, hasID := updateMap["id"]; !hasID {
				updateMap["id"] = featureID
			}
			// Set the project field if not already present and we have a project (from global --project flag)
			if cfg.Project != "" {
				if _, hasProject := updateMap["project"]; !hasProject {
					updateMap["project"] = cfg.Project
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
				output.PrintTo(cmd.OutOrStderr(), currentFeature, output.JSON)
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
	Use:         "delete <feature-id>",
	Short:       "Delete a feature",
	Annotations: map[string]string{"route": "DELETE /api/admin/tenants/:tenant/features/:id"},
	Long:        `Delete a feature flag. This operation cannot be undone.`,
	Args:        cobra.ExactArgs(1),
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

// featuresPatchCmd applies batch patches to multiple features
var featuresPatchCmd = &cobra.Command{
	Use:         "patch",
	Short:       "Batch update multiple features",
	Annotations: map[string]string{"route": "PATCH /api/admin/tenants/:tenant/features"},
	Long: `Apply batch patches to multiple features in a single request.

Patch operations allow you to update specific fields across multiple features
without providing full feature definitions. This is useful for:
- Enabling/disabling multiple features at once
- Moving features between projects
- Updating tags across multiple features
- Deleting multiple features

Patch format (JSON array of operations):
  [
    {"op": "replace", "path": "/<feature-id>/enabled", "value": true},
    {"op": "replace", "path": "/<feature-id>/project", "value": "<project-id>"},
    {"op": "replace", "path": "/<feature-id>/tags", "value": ["tag1", "tag2"]},
    {"op": "remove", "path": "/<feature-id>"}
  ]

Examples:
  # Disable multiple features
  iz admin features patch --data '[{"op":"replace","path":"/feat1/enabled","value":false},{"op":"replace","path":"/feat2/enabled","value":false}]'

  # Patch from file
  iz admin features patch --data @patches.json

  # Move features to different project
  iz admin features patch --data '[{"op":"replace","path":"/feat1/project","value":"new-project-id"}]'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.Validate(); err != nil {
			return err
		}
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		if !cmd.Flags().Changed("data") {
			return fmt.Errorf("patch data is required (use --data flag)")
		}

		var patches interface{}
		if err := parseJSONData(featureData, &patches); err != nil {
			return fmt.Errorf("invalid JSON patch data: %w", err)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.PatchFeatures(ctx, cfg.Tenant, patches); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Features patched successfully\n")
		return nil
	},
}

// featuresTestCmd tests an existing feature's evaluation
var featuresTestCmd = &cobra.Command{
	Use:         "test <feature-id>",
	Short:       "Test an existing feature's evaluation",
	Annotations: map[string]string{"route": "POST /api/admin/tenants/:tenant/features/:id/test"},
	Long: `Test how a feature evaluates for a given user and context without making changes.

This is useful for debugging feature behavior, testing activation conditions,
and verifying feature configuration before deployment.

The --date flag defaults to "now" (current time). You can also specify an ISO 8601
datetime (e.g., 2025-01-01T00:00:00Z) to test activation at a specific time.

For WASM/script features, you can provide a JSON payload via --data.

Examples:
  # Test feature evaluation (uses current time)
  iz admin features test feat-id

  # Test feature for a specific user
  iz admin features test feat-id --user user123

  # Test with specific date
  iz admin features test feat-id --user user123 --date 2025-06-01T12:00:00Z

  # Test with context path
  iz admin features test feat-id --user user123 --context /prod/region1

  # Test WASM feature with payload
  iz admin features test feat-id --user user123 --data '{"age": 25}'`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.Validate(); err != nil {
			return err
		}
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		// Handle "now" shortcut
		date := featureTestDate
		if date == "now" {
			date = nowISO8601()
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		featureID := args[0]
		contextPath := ensureLeadingSlash(featureContextStr)

		// Parse payload if provided
		var payload string
		if featureData != "" {
			var payloadData interface{}
			if err := parseJSONData(featureData, &payloadData); err != nil {
				return fmt.Errorf("invalid JSON payload: %w", err)
			}
			payloadBytes, _ := marshalJSON(payloadData)
			payload = string(payloadBytes)
		}

		ctx := context.Background()

		// For JSON output, use Identity mapper
		if outputFormat == "json" {
			raw, err := izanami.TestFeature(client, ctx, cfg.Tenant, featureID, contextPath, featureUser, date, payload, izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseFeatureTestResult mapper
		result, err := izanami.TestFeature(client, ctx, cfg.Tenant, featureID, contextPath, featureUser, date, payload, izanami.ParseFeatureTestResult)
		if err != nil {
			return err
		}

		return output.PrintTo(cmd.OutOrStdout(), result, output.Format(outputFormat))
	},
}

// featuresTestDefinitionCmd tests a feature definition without saving
var featuresTestDefinitionCmd = &cobra.Command{
	Use:         "test-definition",
	Short:       "Test a feature definition without saving",
	Annotations: map[string]string{"route": "POST /api/admin/tenants/:tenant/test"},
	Long: `Test how a feature definition would evaluate without saving it.

This is useful for validating feature configurations before creating or updating
a feature, especially for complex activation conditions.

The --date flag defaults to "now" (current time). You can also specify an ISO 8601
datetime (e.g., 2025-01-01T00:00:00Z) to test activation at a specific time.

The --data flag is required and should contain the feature definition to test.
Required fields: name, enabled, resultType (boolean|string|number), conditions (array).

Examples:
  # Test a simple boolean feature definition
  iz admin features test-definition --data '{"name":"test","enabled":true,"resultType":"boolean","conditions":[]}'

  # Test with user context
  iz admin features test-definition --data @feature.json --user user123

  # Test with specific date
  iz admin features test-definition --data @feature.json --date 2025-06-01T12:00:00Z`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.Validate(); err != nil {
			return err
		}
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		if !cmd.Flags().Changed("data") {
			return fmt.Errorf("--data is required (feature definition to test)")
		}

		// Handle "now" shortcut
		date := featureTestDate
		if date == "now" {
			date = nowISO8601()
		}

		var definition interface{}
		if err := parseJSONData(featureData, &definition); err != nil {
			return fmt.Errorf("invalid JSON feature definition: %w", err)
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()

		// For JSON output, use Identity mapper
		if outputFormat == "json" {
			raw, err := izanami.TestFeatureDefinition(client, ctx, cfg.Tenant, featureUser, date, definition, izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseFeatureTestResult mapper
		result, err := izanami.TestFeatureDefinition(client, ctx, cfg.Tenant, featureUser, date, definition, izanami.ParseFeatureTestResult)
		if err != nil {
			return err
		}

		return output.PrintTo(cmd.OutOrStdout(), result, output.Format(outputFormat))
	},
}

// featuresTestBulkCmd tests multiple features at once
var featuresTestBulkCmd = &cobra.Command{
	Use:         "test-bulk",
	Short:       "Test multiple features at once",
	Annotations: map[string]string{"route": "GET /api/admin/tenants/:tenant/features/_test"},
	Long: `Test evaluation of multiple features for a given context.

This is useful for understanding how a set of features will behave for a specific
user and context, useful for debugging and validation.

You can filter which features to test using:
  --features: Feature IDs or names (names require --project to be set)
  --projects: All features in these projects
  --one-tag-in: Features with at least one of these tags
  --all-tags-in: Features with all of these tags
  --no-tag-in: Features without any of these tags

At least one filter (--features or --projects) is required.

When using feature names instead of UUIDs, the --project flag must be set (via flag
or config) so that feature names can be resolved to UUIDs.

Examples:
  # Test specific features by UUID
  iz admin features test-bulk --features feat-uuid1,feat-uuid2 --user user123

  # Test features by name (requires --project)
  iz admin features test-bulk --project my-project --features my-feature,other-feature

  # Test all features in a project
  iz admin features test-bulk --projects proj1 --user user123

  # Test with context
  iz admin features test-bulk --projects proj1 --context /prod --user user123

  # Test with tag filters
  iz admin features test-bulk --projects proj1 --one-tag-in beta,experimental`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.Validate(); err != nil {
			return err
		}
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		if len(featureTestFeatures) == 0 && len(featureTestProjects) == 0 {
			return fmt.Errorf("at least one filter is required: --features or --projects")
		}

		// Handle "now" shortcut for date
		date := featureTestDate
		if date == "now" {
			date = nowISO8601()
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()

		// Resolve feature names to UUIDs if project is available
		resolvedFeatures, err := resolveFeaturesToUUIDs(ctx, client, cfg.Tenant, cfg.Project, featureTestFeatures, cfg.Verbose, cmd)
		if err != nil {
			return err
		}

		// Resolve project names to UUIDs
		resolvedProjects, err := resolveProjectsToUUIDs(ctx, client, cfg.Tenant, featureTestProjects, cfg.Verbose, cmd)
		if err != nil {
			return err
		}

		// Resolve tag names to UUIDs
		resolvedOneTagIn, err := resolveTagsToUUIDs(ctx, client, cfg.Tenant, featureTestOneTagIn, cfg.Verbose, cmd)
		if err != nil {
			return err
		}
		resolvedAllTagsIn, err := resolveTagsToUUIDs(ctx, client, cfg.Tenant, featureTestAllTagsIn, cfg.Verbose, cmd)
		if err != nil {
			return err
		}
		resolvedNoTagIn, err := resolveTagsToUUIDs(ctx, client, cfg.Tenant, featureTestNoTagIn, cfg.Verbose, cmd)
		if err != nil {
			return err
		}

		request := izanami.TestFeaturesAdminRequest{
			User:      featureUser,
			Date:      date,
			Features:  resolvedFeatures,
			Projects:  resolvedProjects,
			Context:   featureContextStr,
			OneTagIn:  resolvedOneTagIn,
			AllTagsIn: resolvedAllTagsIn,
			NoTagIn:   resolvedNoTagIn,
		}

		// For JSON output, use Identity mapper
		if outputFormat == "json" {
			raw, err := izanami.TestFeaturesBulk(client, ctx, cfg.Tenant, request, izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseFeatureTestResults mapper
		results, err := izanami.TestFeaturesBulk(client, ctx, cfg.Tenant, request, izanami.ParseFeatureTestResults)
		if err != nil {
			return err
		}

		// Convert to table view
		tableView := results.ToTableView()
		return output.PrintTo(cmd.OutOrStdout(), tableView, output.Table)
	},
}

func init() {
	// List flags
	featuresListCmd.Flags().StringVar(&featureTag, "tag", "", "Filter by tag (server-side)")
	// Project filtering uses global --project flag

	// Create flags
	// Project uses global --project flag
	featuresCreateCmd.Flags().StringVar(&featureDesc, "description", "", "Feature description")
	featuresCreateCmd.Flags().BoolVar(&featureEnabled, "enabled", false, "Enable the feature")
	featuresCreateCmd.Flags().StringSliceVar(&featureTags, "tags", []string{}, "Feature tags")
	featuresCreateCmd.Flags().StringVar(&featureData, "data", "", "JSON feature data (from file with @file.json, stdin with -, or inline)")

	// Update flags
	featuresUpdateCmd.Flags().StringVar(&featureData, "data", "", "JSON feature data (from file with @file.json, stdin with -, or inline)")
	featuresUpdateCmd.MarkFlagRequired("data")

	// Delete flags
	featuresDeleteCmd.Flags().BoolVarP(&featuresDeleteForce, "force", "f", false, "Skip confirmation prompt")

	// Patch flags
	featuresPatchCmd.Flags().StringVar(&featureData, "data", "", "JSON patch data (from file with @file.json, stdin with -, or inline)")
	featuresPatchCmd.MarkFlagRequired("data")

	// Test flags (for testing an existing feature)
	featuresTestCmd.Flags().StringVar(&featureUser, "user", "", "User ID for evaluation")
	featuresTestCmd.Flags().StringVar(&featureTestDate, "date", "now", "Evaluation date (ISO 8601 format or 'now')")
	featuresTestCmd.Flags().StringVar(&featureContextStr, "context", "", "Context path for evaluation")
	featuresTestCmd.Flags().StringVar(&featureData, "data", "", "JSON payload for WASM features (from file with @file.json, stdin with -, or inline)")

	// Test-definition flags
	featuresTestDefinitionCmd.Flags().StringVar(&featureUser, "user", "", "User ID for evaluation")
	featuresTestDefinitionCmd.Flags().StringVar(&featureTestDate, "date", "now", "Evaluation date (ISO 8601 format or 'now')")
	featuresTestDefinitionCmd.Flags().StringVar(&featureData, "data", "", "Feature definition JSON (from file with @file.json, stdin with -, or inline) - required")
	featuresTestDefinitionCmd.MarkFlagRequired("data")

	// Test-bulk flags
	featuresTestBulkCmd.Flags().StringVar(&featureUser, "user", "", "User ID for evaluation")
	featuresTestBulkCmd.Flags().StringVar(&featureTestDate, "date", "now", "Evaluation date (ISO 8601 format or 'now')")
	featuresTestBulkCmd.Flags().StringVar(&featureContextStr, "context", "", "Context path for evaluation")
	featuresTestBulkCmd.Flags().StringSliceVar(&featureTestFeatures, "features", []string{}, "Feature IDs to test (comma-separated)")
	featuresTestBulkCmd.Flags().StringSliceVar(&featureTestProjects, "projects", []string{}, "Project IDs to test all features from (comma-separated)")
	featuresTestBulkCmd.Flags().StringSliceVar(&featureTestOneTagIn, "one-tag-in", []string{}, "Features must have at least one of these tags (comma-separated)")
	featuresTestBulkCmd.Flags().StringSliceVar(&featureTestAllTagsIn, "all-tags-in", []string{}, "Features must have all of these tags (comma-separated)")
	featuresTestBulkCmd.Flags().StringSliceVar(&featureTestNoTagIn, "no-tag-in", []string{}, "Features must not have any of these tags (comma-separated)")

	// Register new subcommands with featuresCmd
	featuresCmd.AddCommand(featuresPatchCmd)
	featuresCmd.AddCommand(featuresTestCmd)
	featuresCmd.AddCommand(featuresTestDefinitionCmd)
	featuresCmd.AddCommand(featuresTestBulkCmd)
}
