package cmd

import (
	"fmt"
	"io"
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
		w := cmd.OutOrStdout()
		fmt.Fprintln(w, "Available commands:")
		fmt.Fprintln(w)
		printCommandTree(w, rootCmd, "", true)
	},
}

// filterVisibleCommands returns commands excluding help and completion
func filterVisibleCommands(commands []*cobra.Command) []*cobra.Command {
	var visible []*cobra.Command
	for _, c := range commands {
		if c.Name() != "help" && c.Name() != "completion" {
			visible = append(visible, c)
		}
	}
	return visible
}

// getBranchPrefix returns the tree branch character based on position
func getBranchPrefix(isRoot, isLast bool) string {
	if isRoot {
		return ""
	}
	if isLast {
		return "└── "
	}
	return "├── "
}

// getChildPrefix returns the prefix for child commands
func getChildPrefix(prefix string, isRoot, isLast bool) string {
	if isRoot {
		return "  "
	}
	if isLast {
		return prefix + "    "
	}
	return prefix + "│   "
}

// printCommandTree recursively prints all commands in a tree structure
func printCommandTree(w io.Writer, cmd *cobra.Command, prefix string, isRoot bool) {
	commands := cmd.Commands()
	visibleCommands := filterVisibleCommands(commands)

	// Sort commands alphabetically
	sort.Slice(visibleCommands, func(i, j int) bool {
		return visibleCommands[i].Name() < visibleCommands[j].Name()
	})

	// Print each command
	for i, c := range visibleCommands {
		isLast := i == len(visibleCommands)-1

		// Print command with tree structure
		branch := getBranchPrefix(isRoot, isLast)
		fullCommand := getFullCommandPath(c)
		fmt.Fprintf(w, "%s%s%s", prefix, branch, fullCommand)

		if c.Short != "" {
			fmt.Fprintf(w, " - %s", c.Short)
		}
		fmt.Fprintln(w)

		// Print subcommands recursively
		if c.HasSubCommands() {
			newPrefix := getChildPrefix(prefix, isRoot, isLast)
			printCommandTree(w, c, newPrefix, false)
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
