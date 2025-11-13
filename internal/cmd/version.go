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
		fmt.Printf("iz version %s\n", Version)
		fmt.Printf("  Commit:    %s\n", GitCommit)
		fmt.Printf("  Built:     %s\n", BuildDate)
		fmt.Printf("  Go:        %s\n", runtime.Version())
		fmt.Printf("  Platform:  %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
