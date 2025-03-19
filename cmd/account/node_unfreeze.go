package account

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var nodeUnfreezeCmd = &cobra.Command{
	Use:   "node-unfreeze <node-id>",
	Short: "Unfreeze a frozen validator node",
	Args:  cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)
		txCfg := common.GetTransactionConfig()
		rawNodeID := args[0]

		npa.MustHaveAccount()

		// When not in offline mode, connect to the given network endpoint.
		ctx := context.Background()
		var conn connection.Connection
		if !txCfg.Offline {
			var err error
			conn, err = connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)
		}

		// Parse node identifier.
		var nodeID signature.PublicKey
		err := nodeID.UnmarshalText([]byte(rawNodeID))
		cobra.CheckErr(err)

		// Prepare transaction.
		tx := registry.NewUnfreezeNodeTx(0, nil, &registry.UnfreezeNode{
			NodeID: nodeID,
		})

		acc := common.LoadAccount(cfg, npa.AccountName)
		sigTx, err := common.SignConsensusTransaction(ctx, npa, acc, conn, tx)
		cobra.CheckErr(err)

		common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, nil, nil)
	},
}

func init() {
	nodeUnfreezeCmd.Flags().AddFlagSet(common.SelectorNAFlags)
	nodeUnfreezeCmd.Flags().AddFlagSet(common.TxFlags)
}
