package cmd

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// confirmDeletion prompts the user for confirmation before deleting a resource.
// Returns true if user confirms (types 'y'), false otherwise.
//
// The prompt uses cmd.OutOrStdout() and cmd.InOrStdin() for testability.
// Handles EOF gracefully for non-interactive environments.
func confirmDeletion(cmd *cobra.Command, resourceType, resourceName string) bool {
	fmt.Fprintf(cmd.OutOrStdout(), "Delete %s '%s'? (y/N): ", resourceType, resourceName)
	reader := bufio.NewReader(cmd.InOrStdin())
	response, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		fmt.Fprintf(cmd.OutOrStdout(), "Failed to read input: %v\n", err)
		return false
	}
	response = strings.ToLower(strings.TrimSpace(response))

	if response != "y" {
		fmt.Fprintln(cmd.OutOrStdout(), "Cancelled")
		return false
	}
	return true
}
