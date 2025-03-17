package governance

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	governance "github.com/oasisprotocol/oasis-core/go/governance/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var govCastVoteCmd = &cobra.Command{
	Use:   "cast-vote <proposal-id> { yes | no | abstain }",
	Short: "Cast a governance vote on a proposal",
	Args:  cobra.ExactArgs(2),
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)
		txCfg := common.GetTransactionConfig()
		rawProposalID, rawVote := args[0], args[1]

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
		var (
			proposalID uint64
			err        error
		)
		if proposalID, err = strconv.ParseUint(rawProposalID, 10, 64); err != nil {
			cobra.CheckErr(fmt.Errorf("bad proposal ID: %w", err))
		}

		// Parse vote.
		var vote governance.Vote
		if err = vote.UnmarshalText([]byte(rawVote)); err != nil {
			cobra.CheckErr(fmt.Errorf("bad vote: %w", err))
		}

		// Prepare transaction.
		tx := governance.NewCastVoteTx(0, nil, &governance.ProposalVote{
			ID:   proposalID,
			Vote: vote,
		})

		acc := common.LoadAccount(cfg, npa.AccountName)
		sigTx, err := common.SignConsensusTransaction(ctx, npa, acc, conn, tx)
		cobra.CheckErr(err)

		common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, nil, nil)
	},
}

func init() {
	govCastVoteCmd.Flags().AddFlagSet(common.SelectorNAFlags)
	govCastVoteCmd.Flags().AddFlagSet(common.TxFlags)
}
