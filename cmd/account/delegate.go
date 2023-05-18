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

var delegateCmd = &cobra.Command{
	Use:   "delegate <amount> <to>",
	Short: "Delegate given amount of tokens to a validator",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)
		txCfg := common.GetTransactionConfig()
		amount, to := args[0], args[1]

		if npa.Account == nil {
			cobra.CheckErr("no accounts configured in your wallet")
		}

		// When not in offline mode, connect to the given network endpoint.
		ctx := context.Background()
		var conn connection.Connection
		if !txCfg.Offline {
			var err error
			conn, err = connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)
		}

		// Resolve destination address.
		toAddr, _, err := common.ResolveLocalAccountOrAddress(npa.Network, to)
		cobra.CheckErr(err)

		acc := common.LoadAccount(cfg, npa.AccountName)

		var sigTx interface{}
		switch npa.ParaTime {
		case nil:
			// Consensus layer delegation.
			amount, err := helpers.ParseConsensusDenomination(npa.Network, amount)
			cobra.CheckErr(err)

			// Prepare transaction.
			tx := staking.NewAddEscrowTx(0, nil, &staking.Escrow{
				Account: toAddr.ConsensusAddress(),
				Amount:  *amount,
			})

			sigTx, err = common.SignConsensusTransaction(ctx, npa, acc, conn, tx)
			cobra.CheckErr(err)
		default:
			// ParaTime delegation.
			cobra.CheckErr("delegations within ParaTimes are not supported; use --no-paratime")
		}

		common.BroadcastOrExportTransaction(ctx, npa.ParaTime, conn, sigTx, nil, nil)
	},
}

func init() {
	delegateCmd.Flags().AddFlagSet(common.SelectorNAFlags)
	delegateCmd.Flags().AddFlagSet(common.TxFlags)
}
