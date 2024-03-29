package account

import (
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/cmd/account/show"
)

var Cmd = &cobra.Command{
	Use:     "account",
	Short:   "Account operations",
	Aliases: []string{"a", "acc", "accounts"},
}

func init() {
	Cmd.AddCommand(allowCmd)
	Cmd.AddCommand(amendCommissionScheduleCmd)
	Cmd.AddCommand(burnCmd)
	Cmd.AddCommand(delegateCmd)
	Cmd.AddCommand(depositCmd)
	Cmd.AddCommand(entityCmd)
	Cmd.AddCommand(fromPublicKeyCmd)
	Cmd.AddCommand(nodeUnfreezeCmd)
	Cmd.AddCommand(show.Cmd)
	Cmd.AddCommand(transferCmd)
	Cmd.AddCommand(undelegateCmd)
	Cmd.AddCommand(withdrawCmd)
}
