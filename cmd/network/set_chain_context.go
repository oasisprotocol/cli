package network

import (
	"fmt"

	"github.com/spf13/cobra"

	cliConfig "github.com/oasisprotocol/cli/config"
)

var setChainContextCmd = &cobra.Command{
	Use:   "set-chain-context <name> <chain-context>",
	Short: "Sets the chain context of the given network",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		name, chainContext := args[0], args[1]

		net := cfg.Networks.All[name]
		if net == nil {
			cobra.CheckErr(fmt.Errorf("network '%s' does not exist", name))
			return // To make staticcheck happy as it doesn't know CheckErr exits.
		}

		net.ChainContext = chainContext

		err := cfg.Save()
		cobra.CheckErr(err)
	},
}
