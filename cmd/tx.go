package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	consensusTx "github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var (
	txCmd = &cobra.Command{
		Use:   "tx",
		Short: "Raw transaction operations",
	}

	txSubmitCmd = &cobra.Command{
		Use:   "submit <filename.json>",
		Short: "Submit a transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			filename := args[0]

			// Establish connection with the target network.
			ctx := context.Background()
			conn, err := connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)

			rawTx, err := ioutil.ReadFile(filename)
			cobra.CheckErr(err)

			tx, err := tryDecodeTx(rawTx)
			cobra.CheckErr(err)

			var sigTx, meta interface{}
			switch dtx := tx.(type) {
			case *consensusTx.SignedTransaction, *types.UnverifiedTransaction:
				// Signed transaction, just broadcast.
				sigTx = tx
			case *consensusTx.Transaction:
				// Unsigned consensus transaction, sign first.
				acc := common.LoadAccount(cfg, npa.AccountName)
				sigTx, err = common.SignConsensusTransaction(ctx, npa, acc, conn, dtx)
				cobra.CheckErr(err)
			case *types.Transaction:
				// Unsigned runtime transaction, sign first.
				acc := common.LoadAccount(cfg, npa.AccountName)
				sigTx, meta, err = common.SignParaTimeTransaction(ctx, npa, acc, conn, dtx)
				cobra.CheckErr(err)
			}

			// Broadcast signed transaction.
			common.BroadcastTransaction(ctx, npa.ParaTime, conn, sigTx, meta, nil)
		},
	}

	txShowCmd = &cobra.Command{
		Use:   "show <filename.json>",
		Short: "Pretty print a transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			filename := args[0]

			rawTx, err := ioutil.ReadFile(filename)
			cobra.CheckErr(err)

			tx, err := tryDecodeTx(rawTx)
			cobra.CheckErr(err)

			common.PrintTransaction(npa, tx)
		},
	}
)

func tryDecodeTx(rawTx []byte) (any, error) {
	// Determine what kind of a transaction this is by attempting to decode it as either a
	// consensus layer transaction or a runtime transaction. Either could also be unsigned.
	txTypes := []struct {
		typ   any
		valFn func(v any) bool
	}{
		// Consensus transactions.
		{
			consensusTx.SignedTransaction{},
			func(v any) bool {
				tx := v.(*consensusTx.SignedTransaction)
				return len(tx.Blob) > 0 && tx.Signature.SanityCheck(tx.Signature.PublicKey) == nil
			},
		},
		{
			consensusTx.Transaction{},
			func(v any) bool {
				tx := v.(*consensusTx.Transaction)
				return tx.SanityCheck() == nil
			},
		},
		// Runtime transactions.
		{
			types.UnverifiedTransaction{},
			func(v any) bool {
				tx := v.(*types.UnverifiedTransaction)
				return len(tx.Body) > 0 && len(tx.AuthProofs) > 0
			},
		},
		{
			types.Transaction{},
			func(v any) bool {
				tx := v.(*types.Transaction)
				return tx.ValidateBasic() == nil
			},
		},
	}
	for _, txType := range txTypes {
		v := reflect.New(reflect.TypeOf(txType.typ)).Interface()
		if err := json.Unmarshal(rawTx, v); err != nil || !txType.valFn(v) {
			if err := cbor.Unmarshal(rawTx, v); err != nil || !txType.valFn(v) {
				continue
			}
		}
		return v, nil
	}
	return nil, fmt.Errorf("malformed transaction")
}

func init() {
	txSubmitCmd.Flags().AddFlagSet(common.SelectorFlags)
	txShowCmd.Flags().AddFlagSet(common.SelectorNPFlags)

	txCmd.AddCommand(txSubmitCmd)
	txCmd.AddCommand(txShowCmd)
}
