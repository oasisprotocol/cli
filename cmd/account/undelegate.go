package account

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var undelegateCmd = &cobra.Command{
	Use:   "undelegate <shares> <from>",
	Short: "Undelegate given amount of shares from a validator",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)
		txCfg := common.GetTransactionConfig()
		amount, from := args[0], args[1]

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
		fromAddr, _, err := common.ResolveLocalAccountOrAddress(npa.Network, from)
		cobra.CheckErr(err)

		acc := common.LoadAccount(cfg, npa.AccountName)

		var sigTx interface{}
		switch npa.ParaTime {
		case nil:
			// Consensus layer delegation.
			var shares quantity.Quantity
			err = shares.UnmarshalText([]byte(amount))
			cobra.CheckErr(err)

			// Prepare transaction.
			tx := staking.NewReclaimEscrowTx(0, nil, &staking.ReclaimEscrow{
				Account: fromAddr.ConsensusAddress(),
				Shares:  shares,
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
	undelegateCmd.Flags().AddFlagSet(common.SelectorNAFlags)
	undelegateCmd.Flags().AddFlagSet(common.TransactionFlags)
}
