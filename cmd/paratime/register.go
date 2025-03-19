package paratime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var registerCmd = &cobra.Command{
	Use:   "register <descriptor.json>",
	Short: "Register a new ParaTime or update an existing one",
	Args:  cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)
		txCfg := common.GetTransactionConfig()
		filename := args[0]

		npa.MustHaveAccount()

		// When not in offline mode, connect to the given network endpoint.
		ctx := context.Background()
		var conn connection.Connection
		if !txCfg.Offline {
			var err error
			conn, err = connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)
		}

		// Load runtime descriptor.
		rawDescriptor, err := os.ReadFile(filename)
		cobra.CheckErr(err)

		// Parse runtime descriptor.
		var descriptor registry.Runtime
		if err = json.Unmarshal(rawDescriptor, &descriptor); err != nil {
			cobra.CheckErr(fmt.Errorf("malformed ParaTime descriptor: %w", err))
		}

		// Prepare transaction.
		tx := registry.NewRegisterRuntimeTx(0, nil, &descriptor)

		acc := common.LoadAccount(cfg, npa.AccountName)
		sigTx, err := common.SignConsensusTransaction(ctx, npa, acc, conn, tx)
		cobra.CheckErr(err)

		common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, nil, nil)
	},
}

func init() {
	registerCmd.Flags().AddFlagSet(common.SelectorNAFlags)
	registerCmd.Flags().AddFlagSet(common.TxFlags)
}
