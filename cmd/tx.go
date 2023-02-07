package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"

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
		Short: "Submit a previously signed transaction",
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

			// Determine what kind of a transaction this is by attempting to decode it as either a
			// consensus layer transaction or a runtime transaction.
			var (
				sigTx  interface{}
				consTx consensusTx.SignedTransaction
				rtTx   types.UnverifiedTransaction
			)
			if err = json.Unmarshal(rawTx, &consTx); err != nil {
				// Failed to unmarshal as consensus transaction, try as runtime transaction.
				if err = json.Unmarshal(rawTx, &rtTx); err != nil {
					cobra.CheckErr(fmt.Errorf("malformed transaction"))
				}
				sigTx = &rtTx
			} else {
				sigTx = &consTx
			}

			// Broadcast signed transaction.
			common.BroadcastTransaction(ctx, npa.ParaTime, conn, sigTx, nil, nil)
		},
	}
)

func init() {
	txSubmitCmd.Flags().AddFlagSet(common.SelectorFlags)

	txCmd.AddCommand(txSubmitCmd)
}
