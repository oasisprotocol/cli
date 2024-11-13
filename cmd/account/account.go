package account

import (
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/oasisprotocol/cli/cmd/account/show"
)

var (
	subtractFee bool
	// SubtractFeeFlags enables makes the provided amount also contain any transaction fees.
	SubtractFeeFlags *flag.FlagSet

	Cmd = &cobra.Command{
		Use:     "account",
		Short:   "Account operations",
		Aliases: []string{"a", "acc", "accounts"},
	}
)

func init() {
	SubtractFeeFlags = flag.NewFlagSet("", flag.ContinueOnError)
	SubtractFeeFlags.BoolVar(&subtractFee, "subtract-fee", false, "subtract fee from the amount")

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
