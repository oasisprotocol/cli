package network

import (
	"context"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	cliConfig "github.com/oasisprotocol/cli/config"
)

var addCmd = &cobra.Command{
	Use:   "add <name> <rpc-endpoint> [chain-context]",
	Short: "Add a new network",
	Args:  cobra.RangeArgs(2, 3),
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		name, rpc := args[0], args[1]

		// Validate initial network configuration early.
		cobra.CheckErr(config.ValidateIdentifier(name))

		isLocal := strings.HasPrefix(rpc, "unix:")
		if !isLocal {
			// Check if a local file name was given without protocol.
			if info, err := os.Stat(rpc); err == nil {
				isLocal = !info.IsDir()
				if isLocal {
					rpc = "unix:" + rpc
				}
			}
		}
		if isLocal {
			AddLocalNetwork(name, rpc)
			return
		}

		net := config.Network{
			RPC: rpc,
		}

		if len(args) >= 3 {
			net.ChainContext = args[2]
		} else {
			// Connect to the network and query the chain context.
			network := config.Network{
				RPC: net.RPC,
			}
			ctx := context.Background()
			conn, err := connection.ConnectNoVerify(ctx, &network)
			cobra.CheckErr(err)
			chainCtx, err := conn.Consensus().GetChainContext(ctx)
			cobra.CheckErr(err)
			net.ChainContext = chainCtx
			cobra.CheckErr(net.Validate())
		}

		cobra.CheckErr(net.Validate())

		// Ask user for some additional parameters.
		networkDetailsFromSurvey(&net)

		err := cfg.Networks.Add(name, &net)
		cobra.CheckErr(err)

		err = cfg.Save()
		cobra.CheckErr(err)
	},
}
