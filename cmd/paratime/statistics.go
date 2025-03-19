package paratime

import (
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"sort"
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
	// Rounds entity node was elected as a backup executor worker.
	roundsBackup uint64
	// Rounds entity node was elected as a backup executor worker
	// and backup workers were invoked.
	roundsBackupRequired uint64
	// Rounds entity node was a primary proposer.
	roundsPrimaryProposer uint64
	// Rounds entity node proposed as primary proposer.
	roundsPrimaryProposed uint64
	// Rounds entity node proposed as backup proposer.
	roundsBackupProposed uint64

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
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)
		npa.MustHaveParaTime()
		runtimeID := npa.ParaTime.Namespace()

		// Parse command line arguments
		var (
			startHeightArg int64 = -1
			startHeight    int64
			endHeight      int64
		)
		if argLen := len(args); argLen > 0 {
			var err error

			// Start height is present for 1 and 2 args.
			startHeightArg, err = strconv.ParseInt(args[0], 10, 64)
			cobra.CheckErr(err)

			if argLen == 2 {
				endHeight, err = strconv.ParseInt(args[1], 10, 64)
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
			endHeight = blk.Height
		}
		switch {
		case startHeightArg < 0:
			delta := -startHeightArg
			if endHeight <= delta {
				cobra.CheckErr(fmt.Errorf("start-height %d will underflow end-height %d", startHeightArg, endHeight))
			}
			startHeight = endHeight - delta
		case startHeightArg == 0:
			var status *consensus.Status
			status, err = consensusConn.GetStatus(ctx)
			cobra.CheckErr(err)
			startHeight = status.LastRetainedHeight
		default:
			startHeight = startHeightArg
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

		roothashConn := consensusConn.RootHash()
		registryConn := consensusConn.Registry()

		nl, err := common.NewNodeLookup(ctx, consensusConn, registryConn, startHeight)
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

		var (
			state            *roothash.RuntimeState
			roundScheduler   signature.PublicKey
			roundDiscrepancy bool
		)

		// Executor committed and execution discrepancy detected events may span across multiple
		// consensus blocks. To avoid handling all of them, extract necessary data from the state
		// preceding the starting block.
		if height := startHeight - 1; height > 0 {
			state, err = roothashConn.GetRuntimeState(ctx, &roothash.RuntimeRequest{
				RuntimeID: runtimeID,
				Height:    height,
			})
			switch err {
			case nil:
				if state.CommitmentPool == nil || state.CommitmentPool.HighestRank == math.MaxUint64 {
					break
				}
				sc, ok := state.CommitmentPool.SchedulerCommitments[state.CommitmentPool.HighestRank]
				if !ok {
					break
				}
				roundScheduler = sc.Commitment.Header.SchedulerID
				roundDiscrepancy = state.CommitmentPool.Discrepancy
			case roothash.ErrInvalidRuntime:
				// State not available.
			case consensus.ErrVersionNotFound:
				// Height too far in the past.
			default:
				cobra.CheckErr(err)
			}
		}

		for height := startHeight; height < endHeight; height++ {
			if height%1000 == 0 {
				fmt.Printf("progressed: height: %d\n", height)
			}

			err = nl.SetHeight(ctx, height)
			cobra.CheckErr(err)

			// Query the latest runtime state.
			rtRequest := &roothash.RuntimeRequest{
				RuntimeID: runtimeID,
				Height:    height,
			}

			state, err = roothashConn.GetRuntimeState(ctx, rtRequest)
			switch err {
			case nil:
			case roothash.ErrInvalidRuntime:
				// State not available.
				continue
			default:
				cobra.CheckErr(err)
			}

			// Skip if the runtime was suspended.
			if state.Committee == nil || state.CommitmentPool == nil {
				continue
			}

			// Query and process events.
			var evs []*roothash.Event
			evs, err = roothashConn.GetEvents(ctx, height)
			cobra.CheckErr(err)

			for _, ev := range evs {
				// Skip events for other runtimes.
				if ev.RuntimeID != runtimeID {
					continue
				}

				switch {
				case ev.ExecutorCommitted != nil:
					if ev.ExecutorCommitted.Commit.NodeID == ev.ExecutorCommitted.Commit.Header.SchedulerID {
						roundScheduler = ev.ExecutorCommitted.Commit.Header.SchedulerID
					}
				case ev.ExecutionDiscrepancyDetected != nil:
					// Note that we are counting discrepancy events that occurred between
					// the specified start and end blocks, which differs from counting those
					// associated with finalized blocks.
					if ev.ExecutionDiscrepancyDetected.Timeout {
						stats.discrepancyDetectedTimeout++
					} else {
						stats.discrepancyDetected++
					}

					roundDiscrepancy = true
				case ev.Finalized != nil:
					func() {
						stats.rounds++
						switch ht := state.LastBlock.Header.HeaderType; ht {
						case block.Normal:
							stats.successfulRounds++
						case block.RoundFailed:
							stats.failedRounds++
						case block.EpochTransition:
							stats.epochTransitionRounds++
							return
						case block.Suspended:
							stats.suspendedRounds++
							return
						default:
							cobra.CheckErr(fmt.Errorf("unexpected block header type: header_type: %v, height: %v", ht, height))
						}

						primaryScheduler, ok := state.Committee.Scheduler(state.LastBlock.Header.Round, 0)
						if !ok {
							cobra.CheckErr(fmt.Errorf("failed to query primary scheduler, no workers in committee"))
						}

						var rtResults *roothash.RoundResults
						rtResults, err = roothashConn.GetLastRoundResults(ctx, rtRequest)
						cobra.CheckErr(err)

						seen := make(map[signature.PublicKey]struct{})
						good := make(map[signature.PublicKey]struct{})
						bad := make(map[signature.PublicKey]struct{})

						for _, ent := range rtResults.GoodComputeEntities {
							good[ent] = struct{}{}
						}
						for _, ent := range rtResults.BadComputeEntities {
							bad[ent] = struct{}{}
						}

						for _, member := range state.Committee.Members {
							entity := nodeToEntity(member.PublicKey)
							if _, ok := stats.entities[entity]; !ok {
								stats.entities[entity] = &entityStats{}
							}

							// Count as elected once if the node has multiple roles.
							if _, ok := seen[member.PublicKey]; !ok {
								stats.entities[entity].roundsElected++
							}
							seen[member.PublicKey] = struct{}{}

							switch member.Role {
							case scheduler.RoleWorker:
								stats.entities[entity].roundsPrimary++

								if _, ok := good[entity]; ok {
									stats.entities[entity].committedGoodBlocksPrimary++
								} else if _, ok := bad[entity]; ok {
									stats.entities[entity].committedBadBlocksPrimary++
								} else {
									stats.entities[entity].missedPrimary++
								}

								if member.PublicKey == primaryScheduler.PublicKey {
									stats.entities[entity].roundsPrimaryProposer++
								}

								if member.PublicKey == roundScheduler {
									switch member.PublicKey == primaryScheduler.PublicKey {
									case true:
										stats.entities[entity].roundsPrimaryProposed++
									case false:
										stats.entities[entity].roundsBackupProposed++
									}
								}
							case scheduler.RoleBackupWorker:
								stats.entities[entity].roundsBackup++

								if !roundDiscrepancy {
									break
								}

								stats.entities[entity].roundsBackupRequired++

								if _, ok := good[entity]; ok {
									stats.entities[entity].committedGoodBlocksBackup++
								} else if _, ok := bad[entity]; ok {
									stats.entities[entity].committedBadBlocksBackup++
								} else {
									stats.entities[entity].missedBackup++
								}
							case scheduler.RoleInvalid:
							}
						}
					}()

					roundDiscrepancy = false
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
		"Prim Good Commit",
		"Prim Bad Commit",
		"Prim Missed",
		"Bckp Invoked",
		"Bckp Good Commit",
		"Bckp Bad Commit",
		"Bckp Missed",
		"Primary Proposer",
		"Prim Proposed",
		"Bckp Proposed",
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
		entityAddr := types.NewAddressFromConsensusPublicKey(entity)

		line := []string{
			entityAddr.String(),
			addrToName(entityAddr),
			strconv.FormatUint(stats.roundsElected, 10),
			strconv.FormatUint(stats.roundsPrimary, 10),
			strconv.FormatUint(stats.roundsBackup, 10),
			strconv.FormatUint(stats.committedGoodBlocksPrimary, 10),
			strconv.FormatUint(stats.committedBadBlocksPrimary, 10),
			strconv.FormatUint(stats.missedPrimary, 10),
			strconv.FormatUint(stats.roundsBackupRequired, 10),
			strconv.FormatUint(stats.committedGoodBlocksBackup, 10),
			strconv.FormatUint(stats.committedBadBlocksBackup, 10),
			strconv.FormatUint(stats.missedBackup, 10),
			strconv.FormatUint(stats.roundsPrimaryProposer, 10),
			strconv.FormatUint(stats.roundsPrimaryProposed, 10),
			strconv.FormatUint(stats.roundsBackupProposed, 10),
		}
		s.entitiesOutput = append(s.entitiesOutput, line)
	}

	sort.Slice(s.entitiesOutput, func(i, j int) bool {
		return s.entitiesOutput[i][0] < s.entitiesOutput[j][0]
	})
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
