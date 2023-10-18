package paratime

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/node"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	roothash "github.com/oasisprotocol/oasis-core/go/roothash/api"
	"github.com/oasisprotocol/oasis-core/go/roothash/api/block"
	scheduler "github.com/oasisprotocol/oasis-core/go/scheduler/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
	"github.com/oasisprotocol/cli/metadata"
)

var fileCSV string

type runtimeStats struct {
	// Rounds.
	rounds uint64
	// Successful rounds.
	successfulRounds uint64
	// Failed rounds.
	failedRounds uint64
	// Rounds failed due to proposer timeouts.
	proposerTimeoutedRounds uint64
	// Epoch transition rounds.
	epochTransitionRounds uint64
	// Suspended rounds.
	suspendedRounds uint64

	// Discrepancies.
	discrepancyDetected        uint64
	discrepancyDetectedTimeout uint64

	// Per-entity stats.
	entities map[signature.PublicKey]*entityStats

	entitiesOutput [][]string
	entitiesHeader []string
}

type entityStats struct {
	// Rounds entity node was elected.
	roundsElected uint64
	// Rounds entity node was elected as primary executor worker.
	roundsPrimary uint64
	// Rounds entity node was elected as primary executor worker and workers were invoked.
	roundsPrimaryRequired uint64
	// Rounds entity node was elected as a backup executor worker.
	roundsBackup uint64
	// Rounds entity node was elected as a backup executor worker
	// and backup workers were invoked.
	roundsBackupRequired uint64
	// Rounds entity node was a proposer.
	roundsProposer uint64

	// How many good blocks committed while being primary worker.
	committedGoodBlocksPrimary uint64
	// How many bad blocs committed while being primary worker.
	committedBadBlocksPrimary uint64
	// How many good blocks committed while being backup worker.
	committedGoodBlocksBackup uint64
	// How many bad blocks committed while being backup worker.
	committedBadBlocksBackup uint64

	// How many rounds missed committing a block while being a primary worker.
	missedPrimary uint64
	// How many rounds missed committing a block while being a backup worker (and discrepancy detection was invoked).
	missedBackup uint64
}

var statsCmd = &cobra.Command{
	Use:   "statistics [<start-height> [<end-height>]]",
	Short: "Show ParaTime statistics",
	Long: "Show ParaTime statistics between start-height and end-height round of blocks." +
		"\nIf negative start-height is passed, it will show statistics for the last given blocks." +
		"\nIf 0 start-height passed, it will generate statistics from the oldest block available to the endpoint.",
	Aliases: []string{"stats"},
	Args:    cobra.MaximumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)
		if npa.ParaTime == nil {
			cobra.CheckErr("no ParaTimes configured")
		}
		runtimeID := npa.ParaTime.Namespace()

		// Parse command line arguments
		var (
			startHeightArg int64 = -1
			endHeight      uint64
		)
		if argLen := len(args); argLen > 0 {
			var err error

			// Start height is present for 1 and 2 args.
			startHeightArg, err = strconv.ParseInt(args[0], 10, 64)
			cobra.CheckErr(err)

			if argLen == 2 {
				endHeight, err = strconv.ParseUint(args[1], 10, 64)
				cobra.CheckErr(err)
			}
		}

		// Establish connection with the target network.
		ctx := context.Background()
		conn, err := connection.Connect(ctx, npa.Network)
		cobra.CheckErr(err)

		consensusConn := conn.Consensus()

		// Fixup the start/end heights if they were not specified (or are 0)
		if endHeight == 0 {
			var blk *consensus.Block
			blk, err = consensusConn.GetBlock(ctx, consensus.HeightLatest)
			cobra.CheckErr(err)
			endHeight = uint64(blk.Height)
		}
		var startHeight uint64
		switch {
		case startHeightArg < 0:
			delta := uint64(-startHeightArg)
			if endHeight <= delta {
				cobra.CheckErr(fmt.Errorf("start-height %d will underflow end-height %d", startHeightArg, endHeight))
			}
			startHeight = endHeight - delta
		case startHeightArg == 0:
			var status *consensus.Status
			status, err = consensusConn.GetStatus(ctx)
			cobra.CheckErr(err)
			startHeight = uint64(status.LastRetainedHeight)
		default:
			startHeight = uint64(startHeightArg)
		}

		chainCtx, err := consensusConn.GetChainContext(ctx)
		cobra.CheckErr(err)
		signature.SetChainContext(chainCtx)

		fmt.Println("=== PARATIME STATISTICS ===")
		fmt.Printf("%-26s %s", "Network:", npa.PrettyPrintNetwork())
		fmt.Println()
		fmt.Printf("%-26s %s", "ParaTime ID:", runtimeID)
		fmt.Println()
		fmt.Printf("%-26s %8d", "Start height:", startHeight)
		fmt.Println()
		fmt.Printf("%-26s %8d", "End height:", endHeight)
		fmt.Println()

		// Do the actual work
		stats := &runtimeStats{
			entities: make(map[signature.PublicKey]*entityStats),
		}

		var (
			currentRound     uint64
			currentCommittee *scheduler.Committee
			currentScheduler *scheduler.CommitteeNode
			roundDiscrepancy bool
		)

		roothashConn := consensusConn.RootHash()
		registryConn := consensusConn.Registry()

		nl, err := common.NewNodeLookup(ctx, consensusConn, registryConn, int64(startHeight))
		cobra.CheckErr(err)

		nodeToEntityMap := make(map[signature.PublicKey]signature.PublicKey)
		nodeToEntity := func(id signature.PublicKey) signature.PublicKey {
			// Note: node.EntityID is immutable once registered, so the
			// cache does not need to ever be updated.
			entityID, ok := nodeToEntityMap[id]
			if !ok {
				var node *node.Node
				node, err = nl.ByID(ctx, id)
				cobra.CheckErr(err)

				entityID = node.EntityID
				nodeToEntityMap[id] = entityID
			}

			return entityID
		}

		for height := int64(startHeight); height < int64(endHeight); height++ {
			if height%1000 == 0 {
				fmt.Printf("progressed: height: %d\n", height)
			}
			err = nl.SetHeight(ctx, height)
			cobra.CheckErr(err)

			rtRequest := &roothash.RuntimeRequest{
				RuntimeID: runtimeID,
				Height:    height,
			}

			// Query latest roothash block and events.
			var blk *block.Block
			blk, err = roothashConn.GetLatestBlock(ctx, rtRequest)
			switch err {
			case nil:
			case roothash.ErrInvalidRuntime:
				continue
			default:
				cobra.CheckErr(err)
			}
			var evs []*roothash.Event
			evs, err = roothashConn.GetEvents(ctx, height)
			cobra.CheckErr(err)

			// Go over events before updating potential new round committee info.
			// Even if round transition happened at this height, all events emitted
			// at this height belong to the previous round.
			for _, ev := range evs {
				// Skip events for initial height where we don't have round info yet.
				if height == int64(startHeight) {
					break
				}
				// Skip events for other runtimes.
				if ev.RuntimeID != runtimeID {
					continue
				}
				switch {
				case ev.ExecutorCommitted != nil:
					// Nothing to do here. We use Finalized event Good/Bad Compute node
					// fields to process commitments.
				case ev.ExecutionDiscrepancyDetected != nil:
					if ev.ExecutionDiscrepancyDetected.Timeout {
						stats.discrepancyDetectedTimeout++
					} else {
						stats.discrepancyDetected++
					}
					roundDiscrepancy = true
				case ev.Finalized != nil:
					var rtResults *roothash.RoundResults
					rtResults, err = roothashConn.GetLastRoundResults(ctx, rtRequest)
					cobra.CheckErr(err)

					// Skip the empty finalized event that is triggered on initial round.
					if len(rtResults.GoodComputeEntities) == 0 && len(rtResults.BadComputeEntities) == 0 && currentCommittee == nil {
						continue
					}
					// Skip if epoch transition or suspended blocks.
					if blk.Header.HeaderType == block.EpochTransition || blk.Header.HeaderType == block.Suspended {
						continue
					}

					// Update stats.
				OUTER:
					for _, member := range currentCommittee.Members {
						// entity := nodeToEntity[member.PublicKey]
						entity := nodeToEntity(member.PublicKey)
						// Primary workers are always required.
						if member.Role == scheduler.RoleWorker {
							stats.entities[entity].roundsPrimaryRequired++
						}
						// In case of discrepancies backup workers were invoked as well.
						if roundDiscrepancy && member.Role == scheduler.RoleBackupWorker {
							stats.entities[entity].roundsBackupRequired++
						}

						// Go over good commitments.
						for _, v := range rtResults.GoodComputeEntities {
							if entity != v {
								continue
							}
							switch member.Role {
							case scheduler.RoleWorker:
								stats.entities[entity].committedGoodBlocksPrimary++
								continue OUTER
							case scheduler.RoleBackupWorker:
								if roundDiscrepancy {
									stats.entities[entity].committedGoodBlocksBackup++
									continue OUTER
								}
							case scheduler.RoleInvalid:
							}
						}

						// Go over bad commitments.
						for _, v := range rtResults.BadComputeEntities {
							if entity != v {
								continue
							}
							switch member.Role {
							case scheduler.RoleWorker:
								stats.entities[entity].committedBadBlocksPrimary++
								continue OUTER
							case scheduler.RoleBackupWorker:
								if roundDiscrepancy {
									stats.entities[entity].committedBadBlocksBackup++
									continue OUTER
								}
							case scheduler.RoleInvalid:
							}
						}

						// Neither good nor bad - missed commitment.
						if member.Role == scheduler.RoleWorker {
							stats.entities[entity].missedPrimary++
						}
						if roundDiscrepancy && member.Role == scheduler.RoleBackupWorker {
							stats.entities[entity].missedBackup++
						}
					}
				}
			}

			// New round.
			if currentRound != blk.Header.Round {
				currentRound = blk.Header.Round
				stats.rounds++

				switch blk.Header.HeaderType {
				case block.Normal:
					stats.successfulRounds++
				case block.EpochTransition:
					stats.epochTransitionRounds++
				case block.RoundFailed:
					stats.failedRounds++
				case block.Suspended:
					stats.suspendedRounds++
					currentCommittee = nil
					currentScheduler = nil
					continue
				default:
					cobra.CheckErr(fmt.Errorf(
						"unexpected block header type: header_type: %v, height: %v",
						blk.Header.HeaderType,
						height,
					))
				}

				// Query runtime state and setup committee info for the round.
				var state *roothash.RuntimeState
				state, err = roothashConn.GetRuntimeState(ctx, rtRequest)
				cobra.CheckErr(err)
				if state.Committee == nil || state.CommitmentPool == nil {
					// No committee - election failed(?)
					fmt.Printf("\nWarning: unexpected or missing committee for runtime: height: %d\n", height)
					currentCommittee = nil
					currentScheduler = nil
					continue
				}
				// Set committee info.
				var ok bool
				currentCommittee = state.Committee
				currentScheduler, ok = currentCommittee.Scheduler(currentRound, 0)
				if !ok {
					cobra.CheckErr("failed to query primary scheduler, no workers in committee")
				}
				roundDiscrepancy = false

				// Update election stats.
				seen := make(map[signature.PublicKey]bool)
				for _, member := range currentCommittee.Members {
					entity := nodeToEntity(member.PublicKey)
					if _, ok := stats.entities[entity]; !ok {
						stats.entities[entity] = &entityStats{}
					}

					// Multiple records for same node in case the node has
					// multiple roles. Only count it as elected once.
					if !seen[member.PublicKey] {
						stats.entities[entity].roundsElected++
					}
					seen[member.PublicKey] = true

					if member.Role == scheduler.RoleWorker {
						stats.entities[entity].roundsPrimary++
					}
					if member.Role == scheduler.RoleBackupWorker {
						stats.entities[entity].roundsBackup++
					}
					if member.PublicKey == currentScheduler.PublicKey {
						stats.entities[entity].roundsProposer++
					}
				}
			}
		}

		// Prepare and printout stats.
		entityMetadataLookup, err := metadata.EntitiesFromRegistry(ctx)
		if err != nil {
			// Non-fatal, this is informative and gathering stats is time
			// consuming.
			fmt.Printf("\nWarning: failed to query metadata registry: %v\n", err)
		}

		stats.prepareEntitiesOutput(entityMetadataLookup)
		stats.printStats()

		if fileCSV == "" {
			stats.printEntityStats()
			return
		}

		// Also save entity stats in a csv.
		fout, err := os.Create(fileCSV)
		cobra.CheckErr(err)
		defer fout.Close()

		w := csv.NewWriter(fout)
		err = w.Write(stats.entitiesHeader)
		cobra.CheckErr(err)
		err = w.WriteAll(stats.entitiesOutput)
		cobra.CheckErr(err)
	},
}

func (s *runtimeStats) prepareEntitiesOutput(
	metadataLookup map[types.Address]*metadata.Entity,
) {
	s.entitiesOutput = make([][]string, 0)

	s.entitiesHeader = []string{
		"Entity Addr",
		"Entity Name",
		"Elected",
		"Primary",
		"Backup",
		"Proposer",
		"Primary invoked",
		"Primary Good commit",
		"Prim Bad commmit",
		"Bckp invoked",
		"Bckp Good commit",
		"Bckp Bad commit",
		"Primary missed",
		"Bckp missed",
	}

	addrToName := func(addr types.Address) string {
		if metadataLookup != nil {
			if entry, ok := metadataLookup[addr]; ok {
				return entry.Name
			}
		}
		return ""
	}

	for entity, stats := range s.entities {
		var line []string

		entityAddr := types.NewAddressFromConsensusPublicKey(entity)

		line = append(line,
			entityAddr.String(),
			addrToName(entityAddr),
			strconv.FormatUint(stats.roundsElected, 10),
			strconv.FormatUint(stats.roundsPrimary, 10),
			strconv.FormatUint(stats.roundsBackup, 10),
			strconv.FormatUint(stats.roundsProposer, 10),
			strconv.FormatUint(stats.roundsPrimaryRequired, 10),
			strconv.FormatUint(stats.committedGoodBlocksPrimary, 10),
			strconv.FormatUint(stats.committedBadBlocksPrimary, 10),
			strconv.FormatUint(stats.roundsBackupRequired, 10),
			strconv.FormatUint(stats.committedGoodBlocksBackup, 10),
			strconv.FormatUint(stats.committedBadBlocksBackup, 10),
			strconv.FormatUint(stats.missedPrimary, 10),
			strconv.FormatUint(stats.missedBackup, 10),
		)
		s.entitiesOutput = append(s.entitiesOutput, line)
	}
}

func (s *runtimeStats) printStats() {
	fmt.Printf("%-26s %d", "ParaTime rounds:", s.rounds)
	fmt.Println()
	fmt.Printf("%-26s %d", "Successful rounds:", s.successfulRounds)
	fmt.Println()
	fmt.Printf("%-26s %d", "Epoch transition rounds:", s.epochTransitionRounds)
	fmt.Println()
	fmt.Printf("%-26s %d", "Proposer timed out rounds:", s.proposerTimeoutedRounds)
	fmt.Println()
	fmt.Printf("%-26s %d", "Failed rounds:", s.failedRounds)
	fmt.Println()
	fmt.Printf("%-26s %d", "Discrepancies:", s.discrepancyDetected)
	fmt.Println()
	fmt.Printf("%-26s %d", "Discrepancies (timeout):", s.discrepancyDetectedTimeout)
	fmt.Println()
	fmt.Printf("%-26s %d", "Suspended:", s.suspendedRounds)
	fmt.Println()
}

func (s *runtimeStats) printEntityStats() {
	fmt.Println("\n=== ENTITY STATISTICS ===")
	table := tablewriter.NewWriter(os.Stdout)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.SetHeader(s.entitiesHeader)
	table.AppendBulk(s.entitiesOutput)
	table.Render()
}

func init() {
	statsCmd.Flags().AddFlagSet(common.SelectorNPFlags)
	statsCmd.Flags().StringVarP(&fileCSV, "output-file", "o", "", "output statistics into specified CSV file")
}
