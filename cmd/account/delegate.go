package account

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	sdkSignature "github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var delegateCmd = &cobra.Command{
	Use:   "delegate <amount> <to>",
	Short: "Delegate given amount of tokens to an entity",
	Args:  cobra.ExactArgs(2),
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)
		txCfg := common.GetTransactionConfig()
		amount, to := args[0], args[1]

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

		acc := common.LoadAccount(cfg, npa.AccountName)

		var (
			sigTx, meta interface{}
			waitCh      <-chan interface{}
		)
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
			amountBaseUnits, err := helpers.ParseParaTimeDenomination(npa.ParaTime, amount, npa.ConsensusDenomination())
			cobra.CheckErr(err)

			// Prepare transaction.
			tx := consensusaccounts.NewDelegateTx(nil, &consensusaccounts.Delegate{
				To:     *toAddr,
				Amount: *amountBaseUnits,
			})

			txDetails := sdkSignature.TxDetails{OrigTo: toEthAddr}
			sigTx, meta, err = common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, &txDetails)
			cobra.CheckErr(err)

			if !txCfg.Offline {
				decoder := conn.Runtime(npa.ParaTime).ConsensusAccounts
				waitCh = common.WaitForEvent(ctx, npa.ParaTime, conn, decoder, func(ev client.DecodedEvent) interface{} {
					ce, ok := ev.(*consensusaccounts.Event)
					if !ok || ce.Delegate == nil {
						return nil
					}
					if !ce.Delegate.From.Equal(acc.Address()) || ce.Delegate.Nonce != tx.AuthInfo.SignerInfo[0].Nonce {
						return nil
					}
					return ce.Delegate
				})
			}
		}

		if !common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, nil) {
			return
		}

		// Wait for delegation result in case this was a delegation from a paratime.
		if waitCh == nil {
			return
		}

		fmt.Printf("Waiting for delegation result...\n")

		ev := <-waitCh
		if ev == nil {
			cobra.CheckErr("Failed to wait for event.")
		}

		// Check for result.
		switch we := ev.(*consensusaccounts.DelegateEvent); we.IsSuccess() {
		case true:
			fmt.Printf("Delegation succeeded.\n")
		case false:
			cobra.CheckErr(fmt.Errorf("delegation failed with error code %d from module %s",
				we.Error.Code,
				we.Error.Module,
			))
		}
	},
}

func init() {
	delegateCmd.Flags().AddFlagSet(common.SelectorFlags)
	delegateCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
}
