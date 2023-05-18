package account

import (
	"context"

	"github.com/spf13/cobra"

	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	sdkSignature "github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/accounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var transferCmd = &cobra.Command{
	Use:     "transfer <amount> <to>",
	Short:   "Transfer given amount of tokens",
	Aliases: []string{"t"},
	Args:    cobra.ExactArgs(2),
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
		toAddr, toEthAddr, err := common.ResolveLocalAccountOrAddress(npa.Network, to)
		cobra.CheckErr(err)

		// Check, if to address is known to be unspendable.
		common.CheckForceErr(common.CheckAddressNotReserved(cfg, toAddr.String()))

		acc := common.LoadAccount(cfg, npa.AccountName)

		var sigTx, meta interface{}
		switch npa.ParaTime {
		case nil:
			common.CheckForceErr(common.CheckAddressIsConsensusCapable(cfg, toAddr.String()))
			if toEthAddr != nil {
				common.CheckForceErr(common.CheckAddressIsConsensusCapable(cfg, toEthAddr.Hex()))
			}

			// Consensus layer transfer.
			amount, err := helpers.ParseConsensusDenomination(npa.Network, amount)
			cobra.CheckErr(err)

			// Prepare transaction.
			tx := staking.NewTransferTx(0, nil, &staking.Transfer{
				To:     toAddr.ConsensusAddress(),
				Amount: *amount,
			})

			sigTx, err = common.SignConsensusTransaction(ctx, npa, acc, conn, tx)
			cobra.CheckErr(err)
		default:
			// ParaTime transfer.
			// TODO: This should actually query the ParaTime (or config) to check what the consensus
			//       layer denomination is in the ParaTime. Assume NATIVE for now.
			amountBaseUnits, err := helpers.ParseParaTimeDenomination(npa.ParaTime, amount, types.NativeDenomination)
			cobra.CheckErr(err)

			// Prepare transaction.
			tx := accounts.NewTransferTx(nil, &accounts.Transfer{
				To:     *toAddr,
				Amount: *amountBaseUnits,
			})

			txDetails := sdkSignature.TxDetails{OrigTo: toEthAddr}
			sigTx, meta, err = common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, &txDetails)
			cobra.CheckErr(err)
		}

		common.BroadcastOrExportTransaction(ctx, npa.ParaTime, conn, sigTx, meta, nil)
	},
}

func init() {
	transferCmd.Flags().AddFlagSet(common.SelectorFlags)
	transferCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	transferCmd.Flags().AddFlagSet(common.ForceFlag)
}
