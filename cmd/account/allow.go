package account

import (
	"context"

	"github.com/spf13/cobra"

	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var allowCmd = &cobra.Command{
	Use:   "allow <beneficiary> <amount>",
	Short: "Configure beneficiary allowance",
	Args:  cobra.ExactArgs(2),
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)
		txCfg := common.GetTransactionConfig()
		beneficiary, amount := args[0], args[1]

		npa.MustHaveAccount()

		// When not in offline mode, connect to the given network endpoint.
		ctx := context.Background()
		var conn connection.Connection
		if !txCfg.Offline {
			var err error
			conn, err = connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)
		}

		// Resolve beneficiary address.
		benAddr, _, err := common.ResolveLocalAccountOrAddress(npa.Network, beneficiary)
		cobra.CheckErr(err)

		// Parse amount.
		var negative bool
		if amount[0] == '-' {
			negative = true
			amount = amount[1:]
		}
		amountChange, err := helpers.ParseConsensusDenomination(npa.Network, amount)
		cobra.CheckErr(err)

		// Prepare transaction.
		tx := staking.NewAllowTx(0, nil, &staking.Allow{
			Beneficiary:  benAddr.ConsensusAddress(),
			Negative:     negative,
			AmountChange: *amountChange,
		})

		acc := common.LoadAccount(cfg, npa.AccountName)
		sigTx, err := common.SignConsensusTransaction(ctx, npa, acc, conn, tx)
		cobra.CheckErr(err)

		common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, nil, nil)
	},
}

func init() {
	allowCmd.Flags().AddFlagSet(common.SelectorNAFlags)
	allowCmd.Flags().AddFlagSet(common.TxFlags)
}
