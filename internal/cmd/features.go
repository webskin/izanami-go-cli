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
	featureTags       []string
	featureUser       string
	featureContextStr string
	featureName       string
	featureDesc       string
	featureEnabled    bool
	featureData       string
)

// featuresCmd represents the features command
var featuresCmd = &cobra.Command{
	Use:   "features",
	Short: "Manage feature flags",
	Long: `Manage feature flags in Izanami.

Features are the core of Izanami - they represent toggleable functionality
or configuration values that can be controlled dynamically.

Features can have:
- Different activation strategies (all users, percentage, specific users)
- Time-based activation (specific periods, days, hours)
- Context-specific overrides (different behavior per environment)
- Tags for organization`,
}

// featuresListCmd lists features
var featuresListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all features",
	Long:  `List all features in a tenant, optionally filtered by project or tags.`,
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
		features, err := client.ListFeatures(ctx, cfg.Tenant, featureProject, featureTags)
		if err != nil {
			return err
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

		ctx := context.Background()
		if err := client.UpdateFeature(ctx, cfg.Tenant, args[0], updateData, false); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Feature updated successfully: %s\n", args[0])
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

func init() {
	rootCmd.AddCommand(featuresCmd)

	// Add subcommands
	featuresCmd.AddCommand(featuresListCmd)
	featuresCmd.AddCommand(featuresGetCmd)
	featuresCmd.AddCommand(featuresCreateCmd)
	featuresCmd.AddCommand(featuresUpdateCmd)
	featuresCmd.AddCommand(featuresDeleteCmd)
	featuresCmd.AddCommand(featuresCheckCmd)

	// List flags
	featuresListCmd.Flags().StringVar(&featureProject, "project", "", "Filter by project")
	featuresListCmd.Flags().StringSliceVar(&featureTags, "tags", []string{}, "Filter by tags")

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
