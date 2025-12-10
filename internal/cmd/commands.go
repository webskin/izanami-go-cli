package cmd

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
)

var (
	// commandsShowRoutes shows API routes for each command
	commandsShowRoutes bool
)

// commandEntry holds information about a command for display
type commandEntry struct {
	prefix  string
	branch  string
	cmdPath string
	short   string
	route   string
}

// commandsCmd shows all available commands
var commandsCmd = &cobra.Command{
	Use:   "commands",
	Short: "List all available commands",
	Long: `Display a comprehensive list of all available commands and subcommands.

This shows the complete command tree in an easy-to-read format.

Use --routes to also display the underlying API endpoint for each command.`,
	Run: func(cmd *cobra.Command, args []string) {
		w := cmd.OutOrStdout()
		fmt.Fprintln(w, "Available commands:")
		fmt.Fprintln(w)

		if commandsShowRoutes {
			// Collect all entries first to calculate alignment
			entries := collectCommandEntries(rootCmd, "", true)
			printAlignedCommands(w, entries)
		} else {
			// Simple output without alignment
			printCommandTree(w, rootCmd, "", true)
		}
	},
}

// collectCommandEntries recursively collects all command entries
func collectCommandEntries(cmd *cobra.Command, prefix string, isRoot bool) []commandEntry {
	var entries []commandEntry

	commands := cmd.Commands()
	visibleCommands := filterVisibleCommands(commands)

	sort.Slice(visibleCommands, func(i, j int) bool {
		return visibleCommands[i].Name() < visibleCommands[j].Name()
	})

	for i, c := range visibleCommands {
		isLast := i == len(visibleCommands)-1

		entry := commandEntry{
			prefix:  prefix,
			branch:  getBranchPrefix(isRoot, isLast),
			cmdPath: getFullCommandPath(c),
			short:   c.Short,
		}

		if route, ok := c.Annotations["route"]; ok {
			entry.route = route
		}

		entries = append(entries, entry)

		if c.HasSubCommands() {
			newPrefix := getChildPrefix(prefix, isRoot, isLast)
			entries = append(entries, collectCommandEntries(c, newPrefix, false)...)
		}
	}

	return entries
}

// displayWidth returns the display width of a string (runes, not bytes)
// This handles multi-byte UTF-8 characters like box-drawing characters
func displayWidth(s string) int {
	return utf8.RuneCountInString(s)
}

// formatRoute formats a route with aligned verb and path
// e.g., "POST /api/foo" -> "POST   /api/foo" (verb padded to maxVerbLen)
func formatRoute(route string, maxVerbLen int) string {
	parts := strings.SplitN(route, " ", 2)
	if len(parts) != 2 {
		return route
	}
	verb := parts[0]
	path := parts[1]
	return fmt.Sprintf("%-*s %s", maxVerbLen, verb, path)
}

// printAlignedCommands prints commands with routes aligned
func printAlignedCommands(w io.Writer, entries []commandEntry) {
	// Calculate max width of the left part (prefix + branch + cmdPath + " - " + short)
	maxWidth := 0
	maxVerbLen := 0
	for _, e := range entries {
		width := displayWidth(e.prefix) + displayWidth(e.branch) + displayWidth(e.cmdPath)
		if e.short != "" {
			width += 3 + displayWidth(e.short) // " - " + short
		}
		if width > maxWidth {
			maxWidth = width
		}
		// Track max verb length for route alignment
		if e.route != "" {
			parts := strings.SplitN(e.route, " ", 2)
			if len(parts) >= 1 && len(parts[0]) > maxVerbLen {
				maxVerbLen = len(parts[0])
			}
		}
	}

	// Print with alignment
	for _, e := range entries {
		left := e.prefix + e.branch + e.cmdPath
		if e.short != "" {
			left += " - " + e.short
		}

		if e.route != "" {
			// Pad and add route with aligned verb
			padding := maxWidth - displayWidth(left) + 2 // +2 for minimum spacing
			formattedRoute := formatRoute(e.route, maxVerbLen)
			fmt.Fprintf(w, "%s%s[%s]\n", left, strings.Repeat(" ", padding), formattedRoute)
		} else {
			fmt.Fprintln(w, left)
		}
	}
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

// printCommandTree recursively prints all commands in a tree structure (without routes)
func printCommandTree(w io.Writer, cmd *cobra.Command, prefix string, isRoot bool) {
	commands := cmd.Commands()
	visibleCommands := filterVisibleCommands(commands)

	sort.Slice(visibleCommands, func(i, j int) bool {
		return visibleCommands[i].Name() < visibleCommands[j].Name()
	})

	for i, c := range visibleCommands {
		isLast := i == len(visibleCommands)-1

		branch := getBranchPrefix(isRoot, isLast)
		fullCommand := getFullCommandPath(c)
		fmt.Fprintf(w, "%s%s%s", prefix, branch, fullCommand)

		if c.Short != "" {
			fmt.Fprintf(w, " - %s", c.Short)
		}

		fmt.Fprintln(w)

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
	commandsCmd.Flags().BoolVarP(&commandsShowRoutes, "routes", "r", false, "Show API routes for each command")
}
