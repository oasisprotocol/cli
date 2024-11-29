package wallet

import (
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/config"
)

var setDefaultCmd = &cobra.Command{
	Use:   "set-default <name>",
	Short: "Sets the given account as the default account",
	Args:  cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		cfg := config.Global()
		name := args[0]

		err := cfg.Wallet.SetDefault(name)
		cobra.CheckErr(err)

		err = cfg.Save()
		cobra.CheckErr(err)
	},
}
