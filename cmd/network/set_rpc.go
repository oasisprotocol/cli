package network

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var setRPCCmd = &cobra.Command{
	Use:               "set-rpc <name> <rpc-endpoint>",
	Short:             "Sets the RPC endpoint of the given network",
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: common.CompleteNetworkNames,
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		name, rpc := args[0], args[1]

		net := cfg.Networks.All[name]
		if net == nil {
			cobra.CheckErr(fmt.Errorf("network '%s' does not exist", name))
			return // To make staticcheck happy as it doesn't know CheckErr exits.
		}

		net.RPC = rpc

		err := cfg.Save()
		cobra.CheckErr(err)
	},
}
