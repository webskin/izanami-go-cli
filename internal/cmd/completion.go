package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// completionCmd represents the completion command
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
	Long: `Generate shell completion script for the Izanami CLI.

To load completions:

Bash:

  # To test the completion once without permanently installing it:
  $ source <(iz completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ iz completion bash > /etc/bash_completion.d/iz
  # macOS:
  $ iz completion bash > $(brew --prefix)/etc/bash_completion.d/iz

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ iz completion zsh > "${fpath[1]}/_iz"

  # You will need to start a new shell for this setup to take effect.

  # To test the completion once without permanently installing it:
  # Generate and source the completion script directly
  $ source <(iz completion zsh)

  # After running this, you can immediately test iz <TAB> in the current shell session. The completion will be lost when you close
  # the terminal.

Fish:

  # To test the completion once without permanently installing it:
  $ iz completion fish | source

  # To load completions for each session, execute once:
  $ iz completion fish > ~/.config/fish/completions/iz.fish

PowerShell:

  # To test the completion once without permanently installing it:
  PS> iz completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> iz completion powershell > iz.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactValidArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
