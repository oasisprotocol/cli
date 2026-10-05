package cmd

import (
	"github.com/spf13/cobra"
)

const (
	argBash       = "bash"
	argZsh        = "zsh"
	argFish       = "fish"
	argPowerShell = "powershell"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
	Long: `Generate shell completion script for the specified shell.

To load completions:

Bash:
  $ source <(oasis completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ oasis completion bash > /etc/bash_completion.d/oasis
  # macOS:
  $ oasis completion bash > $(brew --prefix)/etc/bash_completion.d/oasis

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ oasis completion zsh > "${fpath[1]}/_oasis"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ oasis completion fish | source

  # To load completions for each session, execute once:
  $ oasis completion fish > ~/.config/fish/completions/oasis.fish

PowerShell:
  PS> oasis completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> oasis completion powershell > oasis.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{argBash, argZsh, argFish, argPowerShell},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		switch args[0] {
		case argBash:
			return cmd.Root().GenBashCompletion(out)
		case argZsh:
			return cmd.Root().GenZshCompletion(out)
		case argFish:
			return cmd.Root().GenFishCompletion(out, true)
		case argPowerShell:
			return cmd.Root().GenPowerShellCompletionWithDesc(out)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
