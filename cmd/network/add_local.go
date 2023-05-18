package network

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	cliConfig "github.com/oasisprotocol/cli/config"
)

var addLocalCmd = &cobra.Command{
	Use:   "add-local <name> <rpc-endpoint>",
	Short: "Add a new local network",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		name, rpc := args[0], args[1]

		net := config.Network{
			RPC: rpc,
		}
		// Validate initial network configuration early.
		cobra.CheckErr(config.ValidateIdentifier(name))
		if !net.IsLocalRPC() {
			cobra.CheckErr(fmt.Errorf("rpc-endpoint '%s' is not local", rpc))
		}

		// Connect to the network and query the chain context.
		ctx := context.Background()
		conn, err := connection.ConnectNoVerify(ctx, &net)
		cobra.CheckErr(err)

		chainContext, err := conn.Consensus().GetChainContext(ctx)
		cobra.CheckErr(err)
		net.ChainContext = chainContext
		cobra.CheckErr(net.Validate())

		// With a very high probability, the user is going to be
		// adding a local endpoint for an existing network, so try
		// to clone config details from any of the hardcoded
		// defaults.
		var clonedDefault bool
		for _, defaultNet := range config.DefaultNetworks.All {
			if defaultNet.ChainContext != chainContext {
				continue
			}

			// Yep.
			net.Denomination = defaultNet.Denomination
			net.ParaTimes = defaultNet.ParaTimes
			clonedDefault = true
			break
		}

		// If we failed to crib details from a hardcoded config,
		// ask the user.
		if !clonedDefault {
			networkDetailsFromSurvey(&net)
		}

		err = cfg.Networks.Add(name, &net)
		cobra.CheckErr(err)

		err = cfg.Save()
		cobra.CheckErr(err)
	},
}
