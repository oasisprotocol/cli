package account

import (
	"context"
	"fmt"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var withdrawCmd = &cobra.Command{
	Use:   "withdraw <amount> [to]",
	Short: "Withdraw tokens from ParaTime",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)
		txCfg := common.GetTransactionConfig()
		amount := args[0]
		var to string
		if len(args) >= 2 {
			to = args[1]
		}

		npa.MustHaveAccount()
		npa.MustHaveParaTime()

		// When not in offline mode, connect to the given network endpoint.
		ctx := context.Background()
		var conn connection.Connection
		if !txCfg.Offline {
			var err error
			conn, err = connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)
		}

		// Resolve destination address when explicitly specified.
		var toAddr *types.Address
		var addrToCheck string
		if to != "" {
			var ethAddr *ethCommon.Address
			var err error
			toAddr, ethAddr, err = common.ResolveLocalAccountOrAddress(npa.Network, to)
			cobra.CheckErr(err)
			addrToCheck = toAddr.String()
			if ethAddr != nil {
				addrToCheck = ethAddr.Hex()
			}
		} else {
			// Destination address is implicit, but obtain it for safety check below nonetheless.
			addr, _, err := common.ResolveAddress(npa.Network, npa.Account.Address)
			cobra.CheckErr(err)
			addrToCheck = addr.String()
		}

		// Check, if to address is known to be unspendable.
		common.CheckForceErr(common.CheckAddressIsConsensusCapable(cfg, addrToCheck))
		common.CheckForceErr(common.CheckAddressNotReserved(cfg, addrToCheck))

		// Parse amount.
		amountBaseUnits, err := helpers.ParseParaTimeDenomination(npa.ParaTime, amount, npa.ConsensusDenomination())
		cobra.CheckErr(err)

		// Prepare transaction.
		innerTx := consensusaccounts.Withdraw{
			To:     toAddr,
			Amount: *amountBaseUnits,
		}
		tx := consensusaccounts.NewWithdrawTx(nil, &innerTx)

		acc := common.LoadAccount(cfg, npa.AccountName)
		if subtractFee {
			var fee *quantity.Quantity
			_, fee, _, err = common.PrepareParatimeTransaction(ctx, npa, acc, conn, tx)
			cobra.CheckErr(err)
			err = amountBaseUnits.Amount.Sub(fee)
			cobra.CheckErr(err)
			innerTx.Amount = *amountBaseUnits
			tx = consensusaccounts.NewWithdrawTx(nil, &innerTx)
		}
		sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
		cobra.CheckErr(err)

		if txCfg.Export {
			common.ExportTransaction(sigTx)
			return
		}

		decoder := conn.Runtime(npa.ParaTime).ConsensusAccounts
		waitCh := common.WaitForEvent(ctx, npa.ParaTime, conn, decoder, func(ev client.DecodedEvent) interface{} {
			ce, ok := ev.(*consensusaccounts.Event)
			if !ok || ce.Withdraw == nil {
				return nil
			}
			if !ce.Withdraw.From.Equal(acc.Address()) || ce.Withdraw.Nonce != tx.AuthInfo.SignerInfo[0].Nonce {
				return nil
			}
			return ce.Withdraw
		})

		common.BroadcastTransaction(ctx, npa, conn, sigTx, meta, nil)

		fmt.Printf("Waiting for withdraw result...\n")

		ev := <-waitCh
		if ev == nil {
			cobra.CheckErr("Failed to wait for event.")
		}
		we := ev.(*consensusaccounts.WithdrawEvent)

		// Check for result.
		switch we.IsSuccess() {
		case true:
			fmt.Printf("Withdraw succeeded.\n")
		case false:
			cobra.CheckErr(fmt.Errorf("withdraw failed with error code %d from module %s",
				we.Error.Code,
				we.Error.Module,
			))
		}
	},
}

func init() {
	withdrawCmd.Flags().AddFlagSet(SubtractFeeFlags)
	withdrawCmd.Flags().AddFlagSet(common.SelectorFlags)
	withdrawCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	withdrawCmd.Flags().AddFlagSet(common.ForceFlag)
}
