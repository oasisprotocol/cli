package account

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	sdkSignature "github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var undelegateCmd = &cobra.Command{
	Use:   "undelegate <shares> <from>",
	Short: "Undelegate given amount of shares from an entity",
	Args:  cobra.ExactArgs(2),
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)
		txCfg := common.GetTransactionConfig()
		amount, from := args[0], args[1]

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
		fromAddr, toEthAddr, err := common.ResolveLocalAccountOrAddress(npa.Network, from)
		cobra.CheckErr(err)

		acc := common.LoadAccount(cfg, npa.AccountName)

		var shares quantity.Quantity
		err = shares.UnmarshalText([]byte(amount))
		cobra.CheckErr(err)

		var (
			sigTx, meta interface{}
			waitCh      <-chan interface{}
		)
		switch npa.ParaTime {
		case nil:
			// Consensus layer delegation.
			tx := staking.NewReclaimEscrowTx(0, nil, &staking.ReclaimEscrow{
				Account: fromAddr.ConsensusAddress(),
				Shares:  shares,
			})

			sigTx, err = common.SignConsensusTransaction(ctx, npa, acc, conn, tx)
			cobra.CheckErr(err)
		default:
			// ParaTime delegation.
			tx := consensusaccounts.NewUndelegateTx(nil, &consensusaccounts.Undelegate{
				From:   *fromAddr,
				Shares: shares,
			})

			txDetails := sdkSignature.TxDetails{OrigTo: toEthAddr}
			sigTx, meta, err = common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, &txDetails)
			cobra.CheckErr(err)

			if !txCfg.Offline {
				decoder := conn.Runtime(npa.ParaTime).ConsensusAccounts
				waitCh = common.WaitForEvent(ctx, npa.ParaTime, conn, decoder, func(ev client.DecodedEvent) interface{} {
					ce, ok := ev.(*consensusaccounts.Event)
					if !ok || ce.UndelegateStart == nil {
						return nil
					}
					if !ce.UndelegateStart.To.Equal(acc.Address()) || ce.UndelegateStart.Nonce != tx.AuthInfo.SignerInfo[0].Nonce {
						return nil
					}
					return ce.UndelegateStart
				})
			}
		}

		if !common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, nil) {
			return
		}

		// Wait for undelegation start in case this was an undelegation from a paratime.
		if waitCh == nil {
			return
		}

		fmt.Printf("Waiting for undelegation result...\n")

		ev := <-waitCh
		if ev == nil {
			cobra.CheckErr("Failed to wait for event.")
		}

		// Check for result.
		switch we := ev.(*consensusaccounts.UndelegateStartEvent); we.IsSuccess() {
		case true:
			fmt.Printf("Undelegation started.\n")
		case false:
			cobra.CheckErr(fmt.Errorf("undelegation failed with error code %d from module %s",
				we.Error.Code,
				we.Error.Module,
			))
		}
	},
}

func init() {
	undelegateCmd.Flags().AddFlagSet(common.SelectorFlags)
	undelegateCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
}
