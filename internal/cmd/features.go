package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"github.com/webskin/izanami-go-cli/internal/output"
)

var (
	featureProject    string
	featureTag        string // For filtering features list by tag
	featureTags       []string // For assigning tags to a feature when creating
	featureUser       string
	featureContextStr string
	featureName       string
	featureDesc       string
	featureEnabled    bool
	featureData       string
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
		features, err := client.ListFeatures(ctx, cfg.Tenant, featureTag)
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
		feature, err := client.GetFeature(ctx, cfg.Tenant, args[0])
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

		fmt.Fprintf(os.Stderr, "Feature created successfully: %s\n", created.ID)
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
				currentFeature, err := client.GetFeature(ctx, cfg.Tenant, featureID)
				if err != nil {
					return fmt.Errorf("missing required fields (%v) and failed to fetch current feature: %w", missingFields, err)
				}

				// Print current feature structure
				fmt.Fprintf(os.Stderr, "❌ Missing required fields: %v\n\n", missingFields)
				fmt.Fprintf(os.Stderr, "Current feature structure:\n")
				output.Print(currentFeature, output.JSON)
				fmt.Fprintf(os.Stderr, "\nPlease include all required fields in your update.\n")
				return fmt.Errorf("missing required fields: %v", missingFields)
			}
		}

		ctx := context.Background()
		if err := client.UpdateFeature(ctx, cfg.Tenant, featureID, updateData, false); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Feature updated successfully: %s\n", featureID)
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

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.DeleteFeature(ctx, cfg.Tenant, args[0]); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Feature deleted successfully: %s\n", args[0])
		return nil
	},
}

// featuresCheckCmd checks if a feature is active
var featuresCheckCmd = &cobra.Command{
	Use:   "check <feature-id>",
	Short: "Check if a feature is active",
	Long: `Check if a feature is active for a specific user and context.

This uses the client API (v2) to evaluate the feature, taking into account:
- Feature enabled status
- Activation conditions (user targeting, percentages)
- Time-based activation
- Context-specific overrides

Examples:
  # Check feature for a specific user
  iz features check my-feature --user user123

  # Check feature in a specific context
  iz features check my-feature --user user123 --context prod/eu/france`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.Validate(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		// Use context from flag or config
		contextPath := featureContextStr
		if contextPath == "" {
			contextPath = cfg.Context
		}

		result, err := client.CheckFeature(context.Background(), args[0], featureUser, contextPath)
		if err != nil {
			return err
		}

		return output.Print(result, output.Format(outputFormat))
	},
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

	// Check flags
	featuresCheckCmd.Flags().StringVar(&featureUser, "user", "", "User ID for evaluation")
	featuresCheckCmd.Flags().StringVar(&featureContextStr, "context", "", "Context path for evaluation")
}
