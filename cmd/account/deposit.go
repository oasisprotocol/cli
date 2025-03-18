package account

import (
	"context"
	"fmt"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/errors"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	sdkSignature "github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var depositCmd = &cobra.Command{
	Use:   "deposit <amount> [to]",
	Short: "Deposit tokens into ParaTime",
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

		// Resolve destination address when specified.
		var toAddr *types.Address
		var toEthAddr *ethCommon.Address
		if to != "" {
			var err error
			toAddr, toEthAddr, err = common.ResolveLocalAccountOrAddress(npa.Network, to)
			cobra.CheckErr(err)
		}

		// Check, if to address is known to be unspendable.
		if toAddr != nil {
			common.CheckForceErr(common.CheckAddressNotReserved(cfg, toAddr.String()))
		}

		// Parse amount.
		amountBaseUnits, err := helpers.ParseParaTimeDenomination(npa.ParaTime, amount, npa.ConsensusDenomination())
		cobra.CheckErr(err)

		// Prepare transaction.
		tx := consensusaccounts.NewDepositTx(nil, &consensusaccounts.Deposit{
			To:     toAddr,
			Amount: *amountBaseUnits,
		})

		acc := common.LoadAccount(cfg, npa.AccountName)
		txDetails := sdkSignature.TxDetails{OrigTo: toEthAddr}
		sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, &txDetails)
		cobra.CheckErr(err)

		if txCfg.Export {
			common.ExportTransaction(sigTx)
			return
		}

		decoder := conn.Runtime(npa.ParaTime).ConsensusAccounts
		waitCh := common.WaitForEvent(ctx, npa.ParaTime, conn, decoder, func(ev client.DecodedEvent) interface{} {
			ce, ok := ev.(*consensusaccounts.Event)
			if !ok || ce.Deposit == nil {
				return nil
			}
			if !ce.Deposit.From.Equal(acc.Address()) || ce.Deposit.Nonce != tx.AuthInfo.SignerInfo[0].Nonce {
				return nil
			}
			return ce.Deposit
		})

		common.BroadcastTransaction(ctx, npa, conn, sigTx, meta, nil)

		fmt.Printf("Waiting for deposit result...\n")

		ev := <-waitCh
		if ev == nil {
			cobra.CheckErr("Failed to wait for event.")
		}

		// Check for result.
		switch we := ev.(*consensusaccounts.DepositEvent); we.IsSuccess() {
		case true:
			fmt.Printf("Deposit succeeded.\n")
		case false:
			cobra.CheckErr(fmt.Sprintf("Deposit failed with error: %s",
				common.PrettyErrorHints(ctx, npa, conn, tx, meta, &types.FailedCallResult{
					Module:  we.Error.Module,
					Code:    we.Error.Code,
					Message: errors.FromCode(we.Error.Module, we.Error.Code, "").Error(),
				})))
		}
	},
}

func init() {
	depositCmd.Flags().AddFlagSet(common.SelectorFlags)
	depositCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	depositCmd.Flags().AddFlagSet(common.ForceFlag)
}
