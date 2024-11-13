package network

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	cliConfig "github.com/oasisprotocol/cli/config"
)

var setChainContextCmd = &cobra.Command{
	Use:   "set-chain-context <name> [chain-context]",
	Short: "Sets the chain context of the given network",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		name := args[0]

		net := cfg.Networks.All[name]
		if net == nil {
			cobra.CheckErr(fmt.Errorf("network '%s' does not exist", name))
			return // To make staticcheck happy as it doesn't know CheckErr exits.
		}

		if len(args) >= 2 {
			net.ChainContext = args[1]
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

		err := cfg.Save()
		cobra.CheckErr(err)
	},
}
