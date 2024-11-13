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
	Run: func(_ *cobra.Command, args []string) {
		name := args[0]

		acc := common.LoadAccount(config.Global(), name)
		accCfg, _ := common.LoadAccountConfig(config.Global(), name)
		showPublicWalletInfo(name, acc, accCfg)
	},
}

func showPublicWalletInfo(name string, wallet wallet.Account, accCfg *config.Account) {
	kind := "<unknown>"
	if accCfg != nil {
		kind = accCfg.PrettyKind()
	}

	fmt.Printf("Name:             %s\n", name)
	fmt.Printf("Kind:             %s\n", kind)
	if signer := wallet.Signer(); signer != nil {
		fmt.Printf("Public Key:       %s\n", signer.Public())
	}
	if ethAddr := wallet.EthAddress(); ethAddr != nil {
		fmt.Printf("Ethereum address: %s\n", ethAddr.Hex())
	}
	fmt.Printf("Native address:   %s\n", wallet.Address())
}

func init() {
	showCmd.Flags().AddFlagSet(common.AnswerYesFlag)
}
