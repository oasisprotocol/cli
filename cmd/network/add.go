package network

import (
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	cliConfig "github.com/oasisprotocol/cli/config"
)

var addCmd = &cobra.Command{
	Use:   "add <name> <chain-context> <rpc-endpoint>",
	Short: "Add a new network",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		name, chainContext, rpc := args[0], args[1], args[2]

		net := config.Network{
			ChainContext: chainContext,
			RPC:          rpc,
		}
		// Validate initial network configuration early.
		cobra.CheckErr(config.ValidateIdentifier(name))
		cobra.CheckErr(net.Validate())

		// Ask user for some additional parameters.
		networkDetailsFromSurvey(&net)

		err := cfg.Networks.Add(name, &net)
		cobra.CheckErr(err)

		err = cfg.Save()
		cobra.CheckErr(err)
	},
}
