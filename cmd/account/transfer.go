package account

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/quantity"
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
	Use:     "transfer <amount> [<denom>] <to>",
	Short:   "Transfer given amount of tokens",
	Aliases: []string{"t"},
	Args:    cobra.RangeArgs(2, 3),
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)
		txCfg := common.GetTransactionConfig()
		var amount, denom, to string
		switch len(args) {
		case 2:
			amount, to = args[0], args[1]
		case 3:
			amount, denom, to = args[0], args[1], args[2]
		default:
			cobra.CheckErr("unexpected number of arguments") // Should never happen.
		}

		npa.MustHaveAccount()

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
			// Consensus layer transfer.
			common.CheckForceErr(common.CheckAddressIsConsensusCapable(cfg, toAddr.String()))
			if toEthAddr != nil {
				common.CheckForceErr(common.CheckAddressIsConsensusCapable(cfg, toEthAddr.Hex()))
			}

			if denom != "" {
				cobra.CheckErr("consensus layer only supports the native denomination")
			}

			amt, err := helpers.ParseConsensusDenomination(npa.Network, amount)
			cobra.CheckErr(err)

			// Prepare transaction.
			innerTx := staking.Transfer{
				To:     toAddr.ConsensusAddress(),
				Amount: *amt,
			}
			tx := staking.NewTransferTx(0, nil, &innerTx)
			if subtractFee {
				var fee *quantity.Quantity
				_, fee, err = common.PrepareConsensusTransaction(ctx, npa, acc.ConsensusSigner(), conn, tx)
				cobra.CheckErr(err)
				err = amt.Sub(fee)
				cobra.CheckErr(err)
				innerTx.Amount = *amt
				tx = staking.NewTransferTx(0, nil, &innerTx)
			}
			sigTx, err = common.SignConsensusTransaction(ctx, npa, acc, conn, tx)
			cobra.CheckErr(err)
		default:
			// ParaTime transfer.
			amtBaseUnits, err := helpers.ParseParaTimeDenomination(npa.ParaTime, amount, types.Denomination(denom))
			cobra.CheckErr(err)

			// Prepare transaction.
			innerTx := accounts.Transfer{
				To:     *toAddr,
				Amount: *amtBaseUnits,
			}
			tx := accounts.NewTransferTx(nil, &innerTx)
			if subtractFee {
				var fee *quantity.Quantity
				_, fee, _, err = common.PrepareParatimeTransaction(ctx, npa, acc, conn, tx)
				cobra.CheckErr(err)
				err = amtBaseUnits.Amount.Sub(fee)
				cobra.CheckErr(err)
				innerTx.Amount = *amtBaseUnits
				tx = accounts.NewTransferTx(nil, &innerTx)
			}
			txDetails := sdkSignature.TxDetails{OrigTo: toEthAddr}
			sigTx, meta, err = common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, &txDetails)
			cobra.CheckErr(err)
		}

		common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, nil)
	},
}

func init() {
	transferCmd.Flags().AddFlagSet(SubtractFeeFlags)
	transferCmd.Flags().AddFlagSet(common.SelectorFlags)
	transferCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	transferCmd.Flags().AddFlagSet(common.ForceFlag)
}
