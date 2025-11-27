package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"github.com/webskin/izanami-go-cli/internal/output"
)

var (
	// Project flags
	projectDesc string
	projectData string
	// Delete confirmation flag
	projectsDeleteForce bool
)

var adminProjectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "Manage projects",
	Long:  `Manage Izanami projects. Projects organize features within a tenant.`,
}

var adminProjectsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
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
			raw, err := izanami.ListProjects(client, ctx, cfg.Tenant, izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseProjects mapper
		projects, err := izanami.ListProjects(client, ctx, cfg.Tenant, izanami.ParseProjects)
		if err != nil {
			return err
		}

		return output.Print(projects, output.Format(outputFormat))
	},
}

var adminProjectsGetCmd = &cobra.Command{
	Use:   "get <project-name>",
	Short: "Get a specific project",
	Args:  cobra.ExactArgs(1),
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
			raw, err := izanami.GetProject(client, ctx, cfg.Tenant, args[0], izanami.Identity)
			if err != nil {
				return err
			}
			return output.PrintRawJSON(cmd.OutOrStdout(), raw, compactJSON)
		}

		// For table output, use ParseProject mapper
		project, err := izanami.GetProject(client, ctx, cfg.Tenant, args[0], izanami.ParseProject)
		if err != nil {
			return err
		}

		return output.Print(project, output.Format(outputFormat))
	},
}

var adminProjectsCreateCmd = &cobra.Command{
	Use:   "create <project-name>",
	Short: "Create a new project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		projectName := args[0]
		var data interface{}

		if cmd.Flags().Changed("data") {
			if err := parseJSONData(projectData, &data); err != nil {
				return err
			}
		} else {
			data = map[string]interface{}{
				"name":        projectName,
				"description": projectDesc,
			}
		}

		ctx := context.Background()
		if err := client.CreateProject(ctx, cfg.Tenant, data); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Project created successfully: %s\n", projectName)
		return nil
	},
}

var adminProjectsDeleteCmd = &cobra.Command{
	Use:   "delete <project-name>",
	Short: "Delete a project",
	Long:  `Delete a project. WARNING: This will delete all features in the project.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cfg.ValidateTenant(); err != nil {
			return err
		}

		projectName := args[0]

		// Confirm deletion unless --force is used
		if !projectsDeleteForce {
			if !confirmDeletion(cmd, "project", projectName) {
				return nil
			}
		}

		client, err := izanami.NewClient(cfg)
		if err != nil {
			return err
		}

		ctx := context.Background()
		if err := client.DeleteProject(ctx, cfg.Tenant, projectName); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Project deleted successfully: %s\n", projectName)
		return nil
	},
}

func init() {
	// Projects
	adminCmd.AddCommand(adminProjectsCmd)
	adminProjectsCmd.AddCommand(adminProjectsListCmd)
	adminProjectsCmd.AddCommand(adminProjectsGetCmd)
	adminProjectsCmd.AddCommand(adminProjectsCreateCmd)
	adminProjectsCmd.AddCommand(adminProjectsDeleteCmd)

	adminProjectsCreateCmd.Flags().StringVar(&projectDesc, "description", "", "Project description")
	adminProjectsCreateCmd.Flags().StringVar(&projectData, "data", "", "JSON project data")
	adminProjectsDeleteCmd.Flags().BoolVarP(&projectsDeleteForce, "force", "f", false, "Skip confirmation prompt")
}
