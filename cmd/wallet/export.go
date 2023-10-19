package wallet

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/cmd/common"
	"github.com/oasisprotocol/cli/config"
)

var exportCmd = &cobra.Command{
	Use:   "export <name>",
	Short: "Export secret account information",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		fmt.Printf("WARNING: Exporting the account will expose secret key material!\n")
		acc := common.LoadAccount(config.Global(), name)

		showPublicWalletInfo(name, acc)

		fmt.Printf("Export:\n")
		fmt.Println(acc.UnsafeExport())
	},
}
