package account

import (
	"context"

	"github.com/spf13/cobra"

	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var burnCmd = &cobra.Command{
	Use:   "burn <amount>",
	Short: "Burn given amount of tokens",
	Args:  cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)
		txCfg := common.GetTransactionConfig()
		amountStr := args[0]

		npa.MustHaveAccount()

		// When not in offline mode, connect to the given network endpoint.
		ctx := context.Background()
		var conn connection.Connection
		if !txCfg.Offline {
			var err error
			conn, err = connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)
		}

		acc := common.LoadAccount(cfg, npa.AccountName)

		// Consensus layer transfer.
		amount, err := helpers.ParseConsensusDenomination(npa.Network, amountStr)
		cobra.CheckErr(err)

		// Prepare transaction.
		tx := staking.NewBurnTx(0, nil, &staking.Burn{
			Amount: *amount,
		})

		sigTx, err := common.SignConsensusTransaction(ctx, npa, acc, conn, tx)
		cobra.CheckErr(err)

		common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, nil, nil)
	},
}

func init() {
	burnCmd.Flags().AddFlagSet(common.SelectorNAFlags)
	burnCmd.Flags().AddFlagSet(common.TxFlags)
}
