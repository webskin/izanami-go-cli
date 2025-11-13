package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// commandsCmd shows all available commands
var commandsCmd = &cobra.Command{
	Use:   "commands",
	Short: "List all available commands",
	Long: `Display a comprehensive list of all available commands and subcommands.

This shows the complete command tree in an easy-to-read format.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Available commands:")
		fmt.Println()
		printCommandTree(rootCmd, "", true)
	},
}

// printCommandTree recursively prints all commands in a tree structure
func printCommandTree(cmd *cobra.Command, prefix string, isRoot bool) {
	// Get all subcommands
	commands := cmd.Commands()

	// Filter out help and completion commands for cleaner output
	var visibleCommands []*cobra.Command
	for _, c := range commands {
		if c.Name() != "help" && c.Name() != "completion" {
			visibleCommands = append(visibleCommands, c)
		}
	}

	// Sort commands alphabetically
	sort.Slice(visibleCommands, func(i, j int) bool {
		return visibleCommands[i].Name() < visibleCommands[j].Name()
	})

	// Print each command
	for i, c := range visibleCommands {
		isLast := i == len(visibleCommands)-1

		// Build the tree characters
		var branch string
		if isRoot {
			branch = ""
		} else if isLast {
			branch = "└── "
		} else {
			branch = "├── "
		}

		// Print command name and short description
		fullCommand := getFullCommandPath(c)
		fmt.Printf("%s%s%s", prefix, branch, fullCommand)

		// Add short description if available
		if c.Short != "" {
			fmt.Printf(" - %s", c.Short)
		}
		fmt.Println()

		// Print subcommands with appropriate indentation
		if c.HasSubCommands() {
			var newPrefix string
			if isRoot {
				newPrefix = "  "
			} else if isLast {
				newPrefix = prefix + "    "
			} else {
				newPrefix = prefix + "│   "
			}
			printCommandTree(c, newPrefix, false)
		}
	}
}

// getFullCommandPath returns the command name with its usage
func getFullCommandPath(cmd *cobra.Command) string {
	parts := []string{}

	// Walk up the command tree to build the full path
	current := cmd
	for current != nil && current.Name() != "iz" {
		// Get the command name
		name := current.Name()

		// Add args if specified in Use
		useParts := strings.Fields(current.Use)
		if len(useParts) > 1 {
			// Combine name with args
			name = strings.Join(useParts, " ")
		}

		parts = append([]string{name}, parts...)
		current = current.Parent()
	}

	return strings.Join(parts, " ")
}

func init() {
	rootCmd.AddCommand(commandsCmd)
}
