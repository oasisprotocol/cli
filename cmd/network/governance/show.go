package governance

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strconv"

	"github.com/spf13/cobra"

	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	"github.com/oasisprotocol/oasis-core/go/common/node"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	governance "github.com/oasisprotocol/oasis-core/go/governance/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
	"github.com/oasisprotocol/cli/metadata"
)

var govShowCmd = &cobra.Command{
	Use:   "show <proposal-id>",
	Short: "Show proposal status by ID",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)

		// Determine the proposal ID to query.
		proposalID, err := strconv.ParseUint(args[0], 10, 64)
		cobra.CheckErr(err)

		// Establish connection with the target network.
		ctx := context.Background()
		conn, err := connection.Connect(ctx, npa.Network)
		cobra.CheckErr(err)

		consensusConn := conn.Consensus()
		governanceConn := consensusConn.Governance()
		beaconConn := consensusConn.Beacon()
		schedulerConn := consensusConn.Scheduler()
		registryConn := consensusConn.Registry()
		stakingConn := consensusConn.Staking()

		// Figure out the height to use if "latest".
		height, err := common.GetActualHeight(
			ctx,
			consensusConn,
		)
		cobra.CheckErr(err)

		// Retrieve the proposal.
		proposalQuery := &governance.ProposalQuery{
			Height:     height,
			ProposalID: proposalID,
		}
		proposal, err := governanceConn.Proposal(ctx, proposalQuery)
		cobra.CheckErr(err)

		if proposal.State != governance.StateActive {
			// If the proposal is closed, adjust the query height to the
			// epoch at which the proposal was closed.
			height, err = beaconConn.GetEpochBlock(
				ctx,
				proposal.ClosesAt,
			)
			cobra.CheckErr(err)

			proposalQuery.Height = height

			proposal, err = governanceConn.Proposal(ctx, proposalQuery)
			cobra.CheckErr(err)
		}

		// Retrieve the parameters and votes.
		governanceParams, err := governanceConn.ConsensusParameters(ctx, height)
		cobra.CheckErr(err)
		votes, err := governanceConn.Votes(ctx, proposalQuery)
		cobra.CheckErr(err)

		// Retrieve all the node descriptors.
		nodeLookup, err := common.NewNodeLookup(
			ctx,
			consensusConn,
			registryConn,
			height,
		)
		cobra.CheckErr(err)

		// Figure out the per-validator and total voting power.
		//
		// Note: This also initializes the non-voter list to the entire
		// validator set, and each validator that voted will be removed
		// as the actual votes are examined.

		totalVotingStake := quantity.NewQuantity()
		validatorEntitiesEscrow := make(map[staking.Address]*quantity.Quantity)
		voters := make(map[staking.Address]quantity.Quantity)
		nonVoters := make(map[staking.Address]quantity.Quantity)

		validators, err := schedulerConn.GetValidators(ctx, height)
		cobra.CheckErr(err)

		for _, validator := range validators {
			var node *node.Node
			node, err = nodeLookup.ByID(ctx, validator.ID)
			cobra.CheckErr(err)

			// If there are multiple nodes in the validator set belonging
			// to the same entity, only count the entity escrow once.
			entityAddr := staking.NewAddress(node.EntityID)
			if validatorEntitiesEscrow[entityAddr] != nil {
				continue
			}

			var account *staking.Account
			account, err = stakingConn.Account(
				ctx,
				&staking.OwnerQuery{
					Height: height,
					Owner:  entityAddr,
				},
			)
			cobra.CheckErr(err)

			validatorEntitiesEscrow[entityAddr] = &account.Escrow.Active.Balance
			err = totalVotingStake.Add(&account.Escrow.Active.Balance)
			cobra.CheckErr(err)
			nonVoters[entityAddr] = account.Escrow.Active.Balance
		}

		// Tally the votes.

		derivedResults := make(map[governance.Vote]quantity.Quantity)
		var invalidVotes uint64
		for _, vote := range votes {
			escrow, ok := validatorEntitiesEscrow[vote.Voter]
			if !ok {
				// Voter not in current validator set - invalid vote.
				invalidVotes++
				continue
			}

			currentVotes := derivedResults[vote.Vote]
			newVotes := escrow.Clone()
			err = newVotes.Add(&currentVotes)
			cobra.CheckErr(err)
			derivedResults[vote.Vote] = *newVotes

			delete(nonVoters, vote.Voter)
			voters[vote.Voter] = *escrow.Clone()
		}

		// Display the high-level summary of the proposal status.

		fmt.Printf("Network:     %s", npa.PrettyPrintNetwork())
		fmt.Println()

		fmt.Printf("Proposal ID: %d", proposalID)
		fmt.Println()

		fmt.Printf("Status:      %s", proposal.State)
		fmt.Println()

		switch proposal.State {
		case governance.StateActive:
			// Close the proposal to get simulated results.
			proposal.Results = derivedResults
			err = proposal.CloseProposal(
				*totalVotingStake.Clone(),
				governanceParams.StakeThreshold,
			)
			cobra.CheckErr(err)

			var epoch beacon.EpochTime
			epoch, err = beaconConn.GetEpoch(
				ctx,
				height,
			)
			cobra.CheckErr(err)

			fmt.Printf("Vote outcome if ended now: %s", proposal.State)
			fmt.Println()
			fmt.Printf("Voting ends in %d epochs.", proposal.ClosesAt-epoch)
			fmt.Println()
		case governance.StatePassed, governance.StateFailed, governance.StateRejected:
			fmt.Println("Results:")
			for _, v := range []governance.Vote{governance.VoteYes, governance.VoteNo, governance.VoteAbstain} {
				fmt.Printf("  - %s: %s", v, proposal.Results[v])
				fmt.Println()
			}
		default:
			cobra.CheckErr(fmt.Errorf("unexpected proposal state: %v", proposal.State))
		}

		// Calculate voting percentages.
		votedStake, err := proposal.VotedSum()
		cobra.CheckErr(err)

		voteStakePercentage := new(big.Float).SetInt(votedStake.Clone().ToBigInt())
		voteStakePercentage = voteStakePercentage.Mul(voteStakePercentage, new(big.Float).SetInt64(100))
		voteStakePercentage = voteStakePercentage.Quo(voteStakePercentage, new(big.Float).SetInt(totalVotingStake.ToBigInt()))
		fmt.Println()
		fmt.Println("=== VOTED STAKE ===")
		fmt.Printf("Total voting stake: %s", totalVotingStake)
		fmt.Println()
		fmt.Printf(
			"Voted stake:        %s (%.2f%%)",
			votedStake,
			voteStakePercentage,
		)
		fmt.Println()

		votedYes := proposal.Results[governance.VoteYes]
		votedYesPercentage := new(big.Float).SetInt(votedYes.Clone().ToBigInt())
		votedYesPercentage = votedYesPercentage.Mul(votedYesPercentage, new(big.Float).SetInt64(100))
		if votedStake.Cmp(quantity.NewFromUint64(0)) > 0 {
			votedYesPercentage = votedYesPercentage.Quo(votedYesPercentage, new(big.Float).SetInt(votedStake.ToBigInt()))
		}
		fmt.Printf(
			"Voted yes stake:    %s (%.2f%%)",
			votedYes,
			votedYesPercentage,
		)
		fmt.Println()
		fmt.Printf(
			"Threshold:          %d%%",
			governanceParams.StakeThreshold,
		)
		fmt.Println()

		// Try to figure out the human readable names for all the entities.
		fromRegistry, err := metadata.EntitiesFromRegistry(ctx)
		if err != nil {
			fmt.Println()
			fmt.Printf("Warning: failed to query metadata registry: %v", err)
			fmt.Println()
		}
		fromOasisscan, err := metadata.EntitiesFromOasisscan(ctx)
		if err != nil {
			fmt.Println()
			fmt.Printf("Warning: failed to query oasisscan: %v", err)
			fmt.Println()
		}

		getName := func(addr staking.Address) string {
			for _, src := range []struct {
				m      map[types.Address]*metadata.Entity
				suffix string
			}{
				{fromRegistry, ""},
				{fromOasisscan, " (from oasisscan)"},
			} {
				if src.m == nil {
					continue
				}
				if entry := src.m[types.NewAddressFromConsensus(addr)]; entry != nil {
					return entry.Name + src.suffix
				}
			}
			return "<none>"
		}

		fmt.Println()
		fmt.Println("=== VALIDATORS VOTED ===")
		votersList := entitiesByDescendingStake(voters)
		for i, val := range votersList {
			name := getName(val.Address)
			stakePercentage := new(big.Float).SetInt(val.Stake.Clone().ToBigInt())
			stakePercentage = stakePercentage.Mul(stakePercentage, new(big.Float).SetInt64(100))
			stakePercentage = stakePercentage.Quo(stakePercentage, new(big.Float).SetInt(totalVotingStake.ToBigInt()))
			fmt.Printf("  %d. %s,%s,%s (%.2f%%)", i+1, val.Address, name, val.Stake, stakePercentage)
			fmt.Println()
		}

		fmt.Println()
		fmt.Println("=== VALIDATORS NOT VOTED ===")
		nonVotersList := entitiesByDescendingStake(nonVoters)
		for i, val := range nonVotersList {
			name := getName(val.Address)
			stakePercentage := new(big.Float).SetInt(val.Stake.Clone().ToBigInt())
			stakePercentage = stakePercentage.Mul(stakePercentage, new(big.Float).SetInt64(100))
			stakePercentage = stakePercentage.Quo(stakePercentage, new(big.Float).SetInt(totalVotingStake.ToBigInt()))
			fmt.Printf("  %d. %s,%s,%s (%.2f%%)", i+1, val.Address, name, val.Stake, stakePercentage)
			fmt.Println()
		}
	},
}

func entitiesByDescendingStake(m map[staking.Address]quantity.Quantity) entityStakes {
	pl := make(entityStakes, 0, len(m))
	for k, v := range m {
		pl = append(pl, &entityStake{
			Address: k,
			Stake:   v,
		})
	}
	sort.Sort(sort.Reverse(pl))
	return pl
}

type entityStake struct {
	Address staking.Address
	Stake   quantity.Quantity
}

type entityStakes []*entityStake

func (p entityStakes) Len() int           { return len(p) }
func (p entityStakes) Less(i, j int) bool { return p[i].Stake.Cmp(&p[j].Stake) < 0 }
func (p entityStakes) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func init() {
	govShowCmd.Flags().AddFlagSet(common.SelectorNFlags)
	govShowCmd.Flags().AddFlagSet(common.HeightFlag)
}
