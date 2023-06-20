package governance

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	governance "github.com/oasisprotocol/oasis-core/go/governance/api"
	upgrade "github.com/oasisprotocol/oasis-core/go/upgrade/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var (
	govCreateProposalCmd = &cobra.Command{
		Use:   "create-proposal",
		Short: "Create a governance proposal",
	}

	govCreateProposalUpgradeCmd = &cobra.Command{
		Use:   "upgrade <descriptor.json>",
		Short: "Create an upgrade descriptor governance proposal",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()
			filename := args[0]

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

			// Load upgrade descriptor.
			rawDescriptor, err := os.ReadFile(filename)
			cobra.CheckErr(err)

			// Parse upgrade descriptor.
			var descriptor upgrade.Descriptor
			if err = json.Unmarshal(rawDescriptor, &descriptor); err != nil {
				cobra.CheckErr(fmt.Errorf("malformed upgrade descriptor: %w", err))
			}
			if err = descriptor.ValidateBasic(); err != nil {
				cobra.CheckErr(fmt.Errorf("invalid upgrade descriptor: %w", err))
			}

			// Prepare transaction.
			tx := governance.NewSubmitProposalTx(0, nil, &governance.ProposalContent{
				Upgrade: &governance.UpgradeProposal{
					Descriptor: descriptor,
				},
			})

			acc := common.LoadAccount(cfg, npa.AccountName)
			sigTx, err := common.SignConsensusTransaction(ctx, npa, acc, conn, tx)
			cobra.CheckErr(err)

			common.BroadcastOrExportTransaction(ctx, npa.ParaTime, conn, sigTx, nil, nil)
		},
	}

	govCreateProposalCancelUpgradeCmd = &cobra.Command{
		Use:   "cancel-upgrade <proposal-id>",
		Short: "Create a cancel upgrade governance proposal",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()
			rawProposalID := args[0]

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

			// Parse proposal ID.
			proposalID, err := strconv.ParseUint(rawProposalID, 10, 64)
			cobra.CheckErr(err)

			// Prepare transaction.
			tx := governance.NewSubmitProposalTx(0, nil, &governance.ProposalContent{
				CancelUpgrade: &governance.CancelUpgradeProposal{
					ProposalID: proposalID,
				},
			})

			acc := common.LoadAccount(cfg, npa.AccountName)
			sigTx, err := common.SignConsensusTransaction(ctx, npa, acc, conn, tx)
			cobra.CheckErr(err)

			common.BroadcastOrExportTransaction(ctx, npa.ParaTime, conn, sigTx, nil, nil)
		},
	}
)

func init() {
	govCreateProposalUpgradeCmd.Flags().AddFlagSet(common.SelectorNAFlags)
	govCreateProposalUpgradeCmd.Flags().AddFlagSet(common.TxFlags)

	govCreateProposalCancelUpgradeCmd.Flags().AddFlagSet(common.SelectorNAFlags)
	govCreateProposalCancelUpgradeCmd.Flags().AddFlagSet(common.TxFlags)

	govCreateProposalCmd.AddCommand(govCreateProposalUpgradeCmd)
	govCreateProposalCmd.AddCommand(govCreateProposalCancelUpgradeCmd)
}
