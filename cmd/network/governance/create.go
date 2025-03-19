package governance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	governance "github.com/oasisprotocol/oasis-core/go/governance/api"
	keymanager "github.com/oasisprotocol/oasis-core/go/keymanager/api"
	keymanagerSecrets "github.com/oasisprotocol/oasis-core/go/keymanager/secrets"
	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	roothash "github.com/oasisprotocol/oasis-core/go/roothash/api"
	scheduler "github.com/oasisprotocol/oasis-core/go/scheduler/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	upgrade "github.com/oasisprotocol/oasis-core/go/upgrade/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

func parseChange[A any](raw []byte, dst *A, module string) (cbor.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	// Fail on unknown fields, to ensure that the parameters change is valid for the module.
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return nil, fmt.Errorf("%s: %w", module, err)
	}
	return cbor.Marshal(dst), nil
}

func parseConsensusParameterChange(module string, raw []byte) (cbor.RawMessage, error) {
	switch module {
	case governance.ModuleName:
		return parseChange(raw, &governance.ConsensusParameterChanges{}, module)
	case keymanager.ModuleName:
		return parseChange(raw, &keymanagerSecrets.ConsensusParameterChanges{}, module)
	case registry.ModuleName:
		return parseChange(raw, &registry.ConsensusParameterChanges{}, module)
	case roothash.ModuleName:
		return parseChange(raw, &roothash.ConsensusParameterChanges{}, module)
	case scheduler.ModuleName:
		return parseChange(raw, &scheduler.ConsensusParameterChanges{}, module)
	case staking.ModuleName:
		return parseChange(raw, &staking.ConsensusParameterChanges{}, module)
	default:
		return nil, fmt.Errorf("unknown module: %s", module)
	}
}

var (
	govCreateProposalCmd = &cobra.Command{
		Use:   "create-proposal",
		Short: "Create a governance proposal",
	}

	govCreateProposalUpgradeCmd = &cobra.Command{
		Use:   "upgrade <descriptor.json>",
		Short: "Create an upgrade descriptor governance proposal",
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

			common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, nil, nil)
		},
	}

	govCreateProposalParameterChangeCmd = &cobra.Command{
		Use:   "parameter-change <module> <changes.json>",
		Short: "Create a parameter change governance proposal",
		Args:  cobra.ExactArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()

			module := args[0]
			changesFile := args[1]

			npa.MustHaveAccount()

			// When not in offline mode, connect to the given network endpoint.
			ctx := context.Background()
			var conn connection.Connection
			if !txCfg.Offline {
				var err error
				conn, err = connection.Connect(ctx, npa.Network)
				cobra.CheckErr(err)
			}

			// Load and parse changes json.
			rawChanges, err := os.ReadFile(changesFile)
			cobra.CheckErr(err)

			changes, err := parseConsensusParameterChange(module, rawChanges)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("malformed parameter upgrade proposal: %w", err))
			}

			content := &governance.ChangeParametersProposal{
				Module:  module,
				Changes: changes,
			}
			if err = content.ValidateBasic(); err != nil {
				cobra.CheckErr(fmt.Errorf("invalid parameter upgrade proposal: %w", err))
			}

			// Prepare transaction.
			tx := governance.NewSubmitProposalTx(0, nil, &governance.ProposalContent{
				ChangeParameters: content,
			})

			acc := common.LoadAccount(cfg, npa.AccountName)
			sigTx, err := common.SignConsensusTransaction(ctx, npa, acc, conn, tx)
			cobra.CheckErr(err)

			common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, nil, nil)
		},
	}

	govCreateProposalCancelUpgradeCmd = &cobra.Command{
		Use:   "cancel-upgrade <proposal-id>",
		Short: "Create a cancel upgrade governance proposal",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()
			rawProposalID := args[0]

			npa.MustHaveAccount()

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

			common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, nil, nil)
		},
	}
)

func init() {
	govCreateProposalUpgradeCmd.Flags().AddFlagSet(common.SelectorNAFlags)
	govCreateProposalUpgradeCmd.Flags().AddFlagSet(common.TxFlags)

	govCreateProposalParameterChangeCmd.Flags().AddFlagSet(common.SelectorNAFlags)
	govCreateProposalUpgradeCmd.Flags().AddFlagSet(common.TxFlags)

	govCreateProposalCancelUpgradeCmd.Flags().AddFlagSet(common.SelectorNAFlags)
	govCreateProposalCancelUpgradeCmd.Flags().AddFlagSet(common.TxFlags)

	govCreateProposalCmd.AddCommand(govCreateProposalUpgradeCmd)
	govCreateProposalCmd.AddCommand(govCreateProposalParameterChangeCmd)
	govCreateProposalCmd.AddCommand(govCreateProposalCancelUpgradeCmd)
}
