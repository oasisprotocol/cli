package governance

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

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

func addShares(validatorVoteShares map[governance.Vote]quantity.Quantity, vote governance.Vote, amount quantity.Quantity) error {
	amt := amount.Clone()
	currShares := validatorVoteShares[vote]
	if err := amt.Add(&currShares); err != nil {
		return fmt.Errorf("failed to add votes: %w", err)
	}
	validatorVoteShares[vote] = *amt
	return nil
}

func subShares(validatorVoteShares map[governance.Vote]quantity.Quantity, vote governance.Vote, amount quantity.Quantity) error {
	amt := amount.Clone()
	currShares := validatorVoteShares[vote]
	if err := currShares.Sub(amt); err != nil {
		return fmt.Errorf("failed to sub votes: %w", err)
	}
	validatorVoteShares[vote] = currShares
	return nil
}

var (
	showVotes bool

	govShowCmd = &cobra.Command{
		Use:   "show <proposal-id>",
		Short: "Show proposal status by ID",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
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

			hasCorrectVotingPower := true
			switch proposal.State {
			case governance.StateActive:
			default:
				// If the proposal is closed, adjust the query height to the epoch at which the
				// proposal was closed.
				var pastHeight int64
				pastHeight, err = beaconConn.GetEpochBlock(
					ctx,
					proposal.ClosesAt,
				)
				if err != nil {
					// In case we are unable to query historic state, it is still better if we are
					// able to show the proposal, even if we are not able to calculate the total
					// eligible voting power at the time.
					hasCorrectVotingPower = false
					break
				}

				proposalQuery.Height = pastHeight
				height = pastHeight

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
			validatorVotes := make(map[staking.Address]*governance.Vote)
			validatorVoteShares := make(map[staking.Address]map[governance.Vote]quantity.Quantity)
			validatorEntitiesShares := make(map[staking.Address]*staking.SharePool)
			validatorVoters := make(map[staking.Address]quantity.Quantity)
			validatorNonVoters := make(map[staking.Address]quantity.Quantity)

			validators, err := schedulerConn.GetValidators(ctx, height)
			cobra.CheckErr(err)

			for _, validator := range validators {
				var node *node.Node
				node, err = nodeLookup.ByID(ctx, validator.ID)
				cobra.CheckErr(err)

				// If there are multiple nodes in the validator set belonging
				// to the same entity, only count the entity escrow once.
				entityAddr := staking.NewAddress(node.EntityID)
				if validatorEntitiesShares[entityAddr] != nil {
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

				validatorEntitiesShares[entityAddr] = &account.Escrow.Active
				err = totalVotingStake.Add(&account.Escrow.Active.Balance)
				cobra.CheckErr(err)
				validatorNonVoters[entityAddr] = account.Escrow.Active.Balance
				validatorVoteShares[entityAddr] = make(map[governance.Vote]quantity.Quantity)
			}

			// Tally the validator votes.

			var invalidVotes uint64
			for _, vote := range votes {
				escrow, ok := validatorEntitiesShares[vote.Voter]
				if !ok {
					// Non validator votes handled later.
					continue
				}

				validatorVotes[vote.Voter] = &vote.Vote
				if err = addShares(validatorVoteShares[vote.Voter], vote.Vote, escrow.TotalShares); err != nil {
					cobra.CheckErr(fmt.Errorf("failed to add shares: %w", err))
				}
				delete(validatorNonVoters, vote.Voter)
				validatorVoters[vote.Voter] = escrow.Balance
			}

			// Tally the delegator (non-validator) votes.
			type override struct {
				vote         governance.Vote
				shares       quantity.Quantity
				sharePercent *big.Float
			}
			validatorVoteOverrides := make(map[staking.Address]map[staking.Address]override)
			for _, vote := range votes {
				// Fetch outgoing delegations.
				var delegations map[staking.Address]*staking.Delegation
				delegations, err = stakingConn.DelegationsFor(ctx, &staking.OwnerQuery{Height: height, Owner: vote.Voter})
				cobra.CheckErr(err)

				var delegatesToValidator bool
				for to, delegation := range delegations {
					// Skip delegations to non-validators.
					if _, ok := validatorEntitiesShares[to]; !ok {
						continue
					}
					delegatesToValidator = true

					validatorVote := validatorVotes[to]
					// Nothing to do if vote matches the validator's vote.
					if validatorVote != nil && vote.Vote == *validatorVote {
						continue
					}

					// Vote doesn't match. Deduct shares from the validator's vote and
					// add shares to the delegator's vote.
					if validatorVote != nil {
						if err = subShares(validatorVoteShares[to], *validatorVote, delegation.Shares); err != nil {
							cobra.CheckErr(fmt.Errorf("failed to sub shares: %w", err))
						}
					}
					if err = addShares(validatorVoteShares[to], vote.Vote, delegation.Shares); err != nil {
						cobra.CheckErr(fmt.Errorf("failed to add shares: %w", err))
					}

					// Remember the validator vote overrides, so that we can display them later.
					if validatorVoteOverrides[to] == nil {
						validatorVoteOverrides[to] = make(map[staking.Address]override)
					}
					sharePercent := new(big.Float).SetInt(delegation.Shares.Clone().ToBigInt())
					sharePercent = sharePercent.Mul(sharePercent, new(big.Float).SetInt64(100))
					sharePercent = sharePercent.Quo(sharePercent, new(big.Float).SetInt(validatorEntitiesShares[to].TotalShares.ToBigInt()))
					validatorVoteOverrides[to][vote.Voter] = override{
						vote:         vote.Vote,
						shares:       delegation.Shares,
						sharePercent: sharePercent,
					}
				}

				if !delegatesToValidator {
					// Invalid vote if delegator doesn't delegate to a validator.
					invalidVotes++
				}
			}

			// Finalize the voting results - convert votes in shares into results in stake.

			derivedResults := make(map[governance.Vote]quantity.Quantity)
			for validator, votes := range validatorVoteShares {
				sharePool := validatorEntitiesShares[validator]
				for vote, shares := range votes {
					// Compute stake from shares.
					var escrow *quantity.Quantity
					escrow, err = sharePool.StakeForShares(shares.Clone())
					if err != nil {
						cobra.CheckErr(fmt.Errorf("failed to compute stake from shares: %w", err))
					}

					// Add stake to results.
					currentVotes := derivedResults[vote]
					if err = currentVotes.Add(escrow); err != nil {
						cobra.CheckErr(fmt.Errorf("failed to add votes: %w", err))
					}
					derivedResults[vote] = currentVotes
				}
			}

			// Display the high-level summary of the proposal status.

			fmt.Println("=== PROPOSAL STATUS ===")
			fmt.Printf("Network:         %s\n", npa.PrettyPrintNetwork())
			fmt.Printf("Proposal ID:     %d\n", proposalID)
			fmt.Printf("Status:          %s\n", proposal.State)
			fmt.Printf("Submitted By:    %s\n", proposal.Submitter)
			fmt.Printf("Created At:      epoch %d\n", proposal.CreatedAt)

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

				fmt.Printf("Closes At:       epoch %d (in %d epochs)\n", proposal.ClosesAt, proposal.ClosesAt-epoch)
				fmt.Printf("Current Outcome: %s\n", proposal.State)
			case governance.StatePassed, governance.StateFailed, governance.StateRejected:
				fmt.Println("Results:")
				for _, v := range []governance.Vote{governance.VoteYes, governance.VoteNo, governance.VoteAbstain} {
					fmt.Printf("  - %s: %s", v, proposal.Results[v])
					fmt.Println()
				}
			default:
				cobra.CheckErr(fmt.Errorf("unexpected proposal state: %v", proposal.State))
			}

			fmt.Println()

			// Display the proposal content.

			fmt.Println("=== PROPOSAL CONTENT ===")
			proposal.Content.PrettyPrint(ctx, "", os.Stdout)
			fmt.Println()

			// Calculate voting percentages.
			votedStake, err := proposal.VotedSum()
			cobra.CheckErr(err)

			voteStakePercentage := new(big.Float).SetInt(votedStake.Clone().ToBigInt())
			voteStakePercentage = voteStakePercentage.Mul(voteStakePercentage, new(big.Float).SetInt64(100))
			voteStakePercentage = voteStakePercentage.Quo(voteStakePercentage, new(big.Float).SetInt(totalVotingStake.ToBigInt()))
			fmt.Println("=== VOTED STAKE ===")
			switch hasCorrectVotingPower {
			case true:
				// Also show historic voted stake.
				fmt.Printf("Total voting stake: %s\n", totalVotingStake)
				fmt.Printf(
					"Voted stake:        %s (%.2f%%)\n",
					votedStake,
					voteStakePercentage,
				)
			case false:
				fmt.Printf("Voted stake:        %s\n", votedStake)
			}

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

			if hasCorrectVotingPower {
				fmt.Printf(
					"Threshold:          %d%%",
					governanceParams.StakeThreshold,
				)
				fmt.Println()
			}

			if showVotes {
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
				votersList := entitiesByDescendingStake(validatorVoters)
				for i, val := range votersList {
					name := getName(val.Address)
					stakePercentage := new(big.Float).SetInt(val.Stake.Clone().ToBigInt())
					stakePercentage = stakePercentage.Mul(stakePercentage, new(big.Float).SetInt64(100))
					stakePercentage = stakePercentage.Quo(stakePercentage, new(big.Float).SetInt(totalVotingStake.ToBigInt()))

					if hasCorrectVotingPower {
						fmt.Printf("  %d. %s,%s,%s (%.2f%%): %s\n", i+1, val.Address, name, val.Stake, stakePercentage, validatorVotes[val.Address])
					} else {
						fmt.Printf("  %d. %s,%s: %s\n", i+1, val.Address, name, validatorVotes[val.Address])
					}

					// Display delegators that voted differently.
					for voter, override := range validatorVoteOverrides[val.Address] {
						voterName := getName(voter)
						if hasCorrectVotingPower {
							fmt.Printf("    - %s,%s,%s (%.2f%%) -> %s\n", voter, voterName, override.shares, override.sharePercent, override.vote)
						} else {
							fmt.Printf("    - %s,%s -> %s\n", voter, voterName, override.vote)
						}
					}
				}

				if hasCorrectVotingPower {
					fmt.Println()
					fmt.Println("=== VALIDATORS NOT VOTED ===")
					nonVotersList := entitiesByDescendingStake(validatorNonVoters)
					for i, val := range nonVotersList {
						name := getName(val.Address)
						stakePercentage := new(big.Float).SetInt(val.Stake.Clone().ToBigInt())
						stakePercentage = stakePercentage.Mul(stakePercentage, new(big.Float).SetInt64(100))
						stakePercentage = stakePercentage.Quo(stakePercentage, new(big.Float).SetInt(totalVotingStake.ToBigInt()))
						fmt.Printf("  %d. %s,%s,%s (%.2f%%)", i+1, val.Address, name, val.Stake, stakePercentage)
						fmt.Println()
						// Display delegators that voted differently.
						for voter, override := range validatorVoteOverrides[val.Address] {
							voterName := getName(voter)
							fmt.Printf("    - %s,%s,%s (%.2f%%) -> %s", voter, voterName, override.shares, override.sharePercent, override.vote)
							fmt.Println()
						}
					}
				}
			}
		},
	}
)

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
	showVotesFlag := flag.NewFlagSet("", flag.ContinueOnError)
	showVotesFlag.BoolVar(&showVotes, "show-votes", false, "individual entity votes")
	govShowCmd.Flags().AddFlagSet(showVotesFlag)
	govShowCmd.Flags().AddFlagSet(common.SelectorNFlags)
	govShowCmd.Flags().AddFlagSet(common.HeightFlag)
}
