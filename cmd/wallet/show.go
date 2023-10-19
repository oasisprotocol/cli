package wallet

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/cmd/common"
	"github.com/oasisprotocol/cli/config"
	"github.com/oasisprotocol/cli/wallet"
)

var showCmd = &cobra.Command{
	Use:     "show <name>",
	Short:   "Show public account information",
	Aliases: []string{"s"},
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		acc := common.LoadAccount(config.Global(), name)
		showPublicWalletInfo(name, acc)
	},
}

func showPublicWalletInfo(name string, wallet wallet.Account) {
	fmt.Printf("Name:             %s\n", name)
	if signer := wallet.Signer(); signer != nil {
		fmt.Printf("Public Key:       %s\n", signer.Public())
	}
	if ethAddr := wallet.EthAddress(); ethAddr != nil {
		fmt.Printf("Ethereum address: %s\n", ethAddr.Hex())
	}
	fmt.Printf("Native address:   %s\n", wallet.Address())
}
