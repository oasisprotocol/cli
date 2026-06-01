package rofl

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

// trustRoot is the consensus trust root for a ROFL application.
type trustRoot struct {
	Height       uint64 `json:"height"`
	Hash         string `json:"hash"`
	RuntimeID    string `json:"runtime_id"`
	ChainContext string `json:"chain_context"`
}

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
			conn.Consensus().Core(),
		)
		cobra.CheckErr(err)

		blk, err := conn.Consensus().Core().GetBlock(ctx, height)
		cobra.CheckErr(err)

		tr := trustRoot{
			Height:       uint64(height),
			Hash:         blk.Hash.Hex(),
			RuntimeID:    npa.ParaTime.ID,
			ChainContext: npa.Network.ChainContext,
		}

		out, err := common.PrettyJSONMarshal(tr)
		cobra.CheckErr(err)
		fmt.Println(string(out))
	},
}

func init() {
	common.AddSelectorNPFlags(trustRootCmd)
	trustRootCmd.Flags().AddFlagSet(common.HeightFlag)
}
