package wallet

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/cmd/common"
	"github.com/oasisprotocol/cli/config"
)

var exportCmd = &cobra.Command{
	Use:               "export <name>",
	Short:             "Export secret account information",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: common.CompleteAccountNames,
	Run: func(_ *cobra.Command, args []string) {
		name := args[0]

		common.Warn("WARNING: Exporting the account will expose secret key material!")
		acc := common.LoadAccount(config.Global(), name)
		accCfg, _ := common.LoadAccountConfig(config.Global(), name)

		showPublicWalletInfo(name, acc, accCfg)

		key, mnemonic := acc.UnsafeExport()
		if mnemonic != "" {
			fmt.Printf("Secret mnemonic:\n")
			fmt.Println(mnemonic)
			if key != "" {
				fmt.Printf("Derived secret key for account number %d:\n", accCfg.Config["number"])
				fmt.Println(key)
			}
		}
		if mnemonic == "" && key != "" {
			fmt.Printf("Secret key:\n")
			fmt.Println(key)
		}
	},
}

func init() {
	exportCmd.Flags().AddFlagSet(common.AnswerYesFlag)
}
