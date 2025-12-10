package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"github.com/webskin/izanami-go-cli/internal/output"
)

var (
	// Tag flags
	tagDesc string
	tagData string
	// Delete confirmation flag
	tagsDeleteForce bool
)

var adminTagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "Manage tags",
	Long:  `Manage feature tags. Tags help organize and categorize features.`,
}

var adminTagsListCmd = &cobra.Command{
	Use:         "list",
	Short:       "List all tags",
	Annotations: map[string]string{"route": "GET /api/admin/tenants/:tenant/tags"},
	RunE: func(cmd *cobra.Command, args []string) error {
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
			raw, err := izanami.ListTags(client, ctx, cfg.Tenant, izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseTags mapper
		tags, err := izanami.ListTags(client, ctx, cfg.Tenant, izanami.ParseTags)
		if err != nil {
			return err
		}

		return output.PrintTo(cmd.OutOrStdout(), tags, output.Format(outputFormat))
	},
}

var adminTagsGetCmd = &cobra.Command{
	Use:         "get <tag-name>",
	Short:       "Get a specific tag",
	Annotations: map[string]string{"route": "GET /api/admin/tenants/:tenant/tags/:name"},
	Args:        cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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
			raw, err := izanami.GetTag(client, ctx, cfg.Tenant, args[0], izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseTag mapper
		tag, err := izanami.GetTag(client, ctx, cfg.Tenant, args[0], izanami.ParseTag)
		if err != nil {
			return err
		}

		return output.PrintTo(cmd.OutOrStdout(), tag, output.Format(outputFormat))
	},
}

var adminTagsCreateCmd = &cobra.Command{
	Use:         "create <tag-name>",
	Short:       "Create a new tag",
	Annotations: map[string]string{"route": "POST /api/admin/tenants/:tenant/tags"},
	Args:        cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		tagName := args[0]
		var data interface{}

		if cmd.Flags().Changed("data") {
			if err := parseJSONData(tagData, &data); err != nil {
				return err
			}
		} else {
			data = map[string]interface{}{
				"name":        tagName,
				"description": tagDesc,
			}
		}

		ctx := context.Background()
		if err := client.CreateTag(ctx, cfg.Tenant, data); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Tag created successfully: %s\n", tagName)
		return nil
	},
}

var adminTagsDeleteCmd = &cobra.Command{
	Use:         "delete <tag-name>",
	Short:       "Delete a tag",
	Annotations: map[string]string{"route": "DELETE /api/admin/tenants/:tenant/tags/:name"},
	Args:        cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		tagName := args[0]

		// Confirm deletion unless --force is used
		if !tagsDeleteForce {
			if !confirmDeletion(cmd, "tag", tagName) {
				return nil
			}
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.DeleteTag(ctx, cfg.Tenant, tagName); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Tag deleted successfully: %s\n", tagName)
		return nil
	},
}

func init() {
	// Tags
	adminCmd.AddCommand(adminTagsCmd)
	adminTagsCmd.AddCommand(adminTagsListCmd)
	adminTagsCmd.AddCommand(adminTagsGetCmd)
	adminTagsCmd.AddCommand(adminTagsCreateCmd)
	adminTagsCmd.AddCommand(adminTagsDeleteCmd)

	// Dynamic completion for tag name argument
	adminTagsGetCmd.ValidArgsFunction = completeTagNames
	adminTagsDeleteCmd.ValidArgsFunction = completeTagNames

	adminTagsCreateCmd.Flags().StringVar(&tagDesc, "description", "", "Tag description")
	adminTagsCreateCmd.Flags().StringVar(&tagData, "data", "", "JSON tag data")
	adminTagsDeleteCmd.Flags().BoolVarP(&tagsDeleteForce, "force", "f", false, "Skip confirmation prompt")
}
