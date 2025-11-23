package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	// Version is the CLI version (set during build)
	Version = "dev"
	// GitCommit is the git commit hash (set during build)
	GitCommit = "unknown"
	// BuildDate is the build date (set during build)
	BuildDate = "unknown"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display the version of the Izanami CLI, along with build information.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "iz version %s\n", Version)
		fmt.Fprintf(cmd.OutOrStdout(), "  Commit:    %s\n", GitCommit)
		fmt.Fprintf(cmd.OutOrStdout(), "  Built:     %s\n", BuildDate)
		fmt.Fprintf(cmd.OutOrStdout(), "  Go:        %s\n", runtime.Version())
		fmt.Fprintf(cmd.OutOrStdout(), "  Platform:  %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
