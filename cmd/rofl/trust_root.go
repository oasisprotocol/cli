package rofl

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var trustRootCmd = &cobra.Command{
	Use:   "trust-root",
	Short: "Show a recent trust root for a ROFL application",
	Args:  cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)
		npa.MustHaveParaTime()

		// Establish connection with the target network.
		ctx := context.Background()
		conn, err := connection.Connect(ctx, npa.Network)
		cobra.CheckErr(err)

		// Fetch latest consensus block.
		height, err := common.GetActualHeight(
			ctx,
			conn.Consensus(),
		)
		cobra.CheckErr(err)

		blk, err := conn.Consensus().GetBlock(ctx, height)
		cobra.CheckErr(err)

		// TODO: Support different output formats.
		fmt.Printf("TrustRoot {\n")
		fmt.Printf("    height: %d,\n", height)
		fmt.Printf("    hash: \"%s\".into(),\n", blk.Hash)
		fmt.Printf("    runtime_id: \"%s\".into(),\n", npa.ParaTime.ID)
		fmt.Printf("    chain_context: \"%s\".to_string(),\n", npa.Network.ChainContext)
		fmt.Printf("}\n")
	},
}

func init() {
	trustRootCmd.Flags().AddFlagSet(common.SelectorNPFlags)
	trustRootCmd.Flags().AddFlagSet(common.HeightFlag)
}
