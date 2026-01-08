package network

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/consensus/api"
	"github.com/oasisprotocol/oasis-core/go/consensus/cometbft/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

const (
	// maxVerificationTime defines the upper bound within which the light client should
	// be able to fetch and verify light headers and finally detect and penalize a possible
	// byzantine behavior. This value intentionally overestimates the actual time required.
	maxVerificationTime = 24 * time.Hour

	// maxCpCreationTime defines the upper bound within which remote node should produce
	// checkpoint. This value intentionally overestimates the actual time required,
	// assuming nodes with very bulky NodeDB, where checkpoint creation is slow.
	maxCpCreationTime = 36 * time.Hour
)

var trustCmd = &cobra.Command{
	Use:   "trust",
	Short: "Show the recommended light client trust for consensus state sync",
	Long: `Show the recommended light client trust configuration for consensus state sync.

WARNING:
  The output is only reliable if the CLI is connected to the RPC endpoint you control.
  If using (default) public gRPC endpoint you are encouraged to verify trust parameters
  with external sources.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
		defer cancel()

		// Establish connection with the target network.
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)
		conn, err := connection.Connect(ctx, npa.Network)
		if err != nil {
			return fmt.Errorf("failed to establish connection with the target network: %w", err)
		}

		trust, err := calcTrust(ctx, conn)
		if err != nil {
			return fmt.Errorf("failed to calculate consensus state sync trust root: %w", err)
		}

		switch common.OutputFormat() {
		case common.FormatJSON:
			str, err := common.PrettyJSONMarshal(trust)
			if err != nil {
				return fmt.Errorf("failed to pretty json marshal: %w", err)
			}
			fmt.Println(string(str))
		default:
			fmt.Println("Trust period: ", trust.Period)
			fmt.Println("Trust height: ", trust.Height)
			fmt.Println("Trust hash: ", trust.Hash)
			fmt.Println()
			common.Warn("WARNING: Cannot be trusted unless the CLI is connected to the RPC endpoint you control.")
		}

		return nil
	},
}

// calcTrust calculates and verifies the recommended trust config.
func calcTrust(ctx context.Context, conn connection.Connection) (config.TrustConfig, error) {
	latest, err := conn.Consensus().Core().GetBlock(ctx, api.HeightLatest)
	if err != nil {
		return config.TrustConfig{}, fmt.Errorf("failed to get latest block: %w", err)
	}

	cpInterval, err := fetchCheckpointInterval(ctx, conn, latest.Height)
	if err != nil {
		return config.TrustConfig{}, fmt.Errorf("failed to fetch checkpoint interval: %w", err)
	}
	if cpInterval > latest.Height {
		return config.TrustConfig{}, fmt.Errorf("checkpoint interval exceeds latest height")
	}

	blkTime, err := calcAvgBlkTime(ctx, conn, latest)
	if err != nil {
		return config.TrustConfig{}, fmt.Errorf("failed to calculate average block time: %w", err)
	}

	debondingPeriod, err := calcDebondingPeriod(ctx, conn, latest.Height, blkTime)
	if err != nil {
		return config.TrustConfig{}, fmt.Errorf("failed to calculate debonding period (height: %d): %w", latest.Height, err)
	}
	trustPeriod := calcTrustPeriod(debondingPeriod)

	// Going back the whole checkpoint interval plus max checkpoint creation time guarantees there wil be at least
	// one target checkpoint height from the trust height onwards, for which remote nodes already created a checkpoint.
	candidateHeight := latest.Height - cpInterval - int64(maxCpCreationTime/blkTime)
	candidate, err := conn.Consensus().Core().GetBlock(ctx, candidateHeight)
	if err != nil {
		return config.TrustConfig{}, fmt.Errorf("failed to get candidate trust (height: %d): %w", candidateHeight, err)
	}

	// Sanity check: the trusted root must not be older than the sum of the trust
	// period and the maximum verification time. If it is, the light client will
	// not be able to safely use this block as a trusted root for the state sync.
	if candidate.Time.Add(trustPeriod).Add(maxVerificationTime).Before(time.Now()) {
		return config.TrustConfig{}, fmt.Errorf("impossible to calculate safe trust with current parameters")
	}

	return config.TrustConfig{
		Period: trustPeriod.Round(time.Hour),
		Height: uint64(candidate.Height), // #nosec G115
		Hash:   candidate.Hash.Hex(),
	}, nil
}

func fetchCheckpointInterval(ctx context.Context, conn connection.Connection, height int64) (int64, error) {
	params, err := conn.Consensus().Core().GetParameters(ctx, height)
	if err != nil {
		return 0, fmt.Errorf("failed to get consensus parameters (height: %d): %w", height, err)
	}
	return int64(params.Parameters.StateCheckpointInterval), nil // #nosec G115
}

func calcAvgBlkTime(ctx context.Context, conn connection.Connection, latest *api.Block) (time.Duration, error) {
	const deltaBlocks int64 = 1000
	blk, err := conn.Consensus().Core().GetBlock(ctx, latest.Height-deltaBlocks)
	if err != nil {
		return 0, fmt.Errorf("failed to get block: %w", err)
	}

	return latest.Time.Sub(blk.Time) / time.Duration(deltaBlocks), nil
}

func calcDebondingPeriod(ctx context.Context, conn connection.Connection, height int64, blkTime time.Duration) (time.Duration, error) {
	stakingParams, err := conn.Consensus().Staking().ConsensusParameters(ctx, height)
	if err != nil {
		return 0, fmt.Errorf("failed to get staking parameters: %w", err)
	}
	beaconParams, err := conn.Consensus().Beacon().ConsensusParameters(ctx, height)
	if err != nil {
		return 0, fmt.Errorf("failed to get beacon parameters: %w", err)
	}
	debondingBlks := int64(stakingParams.DebondingInterval) * beaconParams.Interval() // #nosec G115
	return time.Duration(debondingBlks) * blkTime, nil
}

// calcTrustPeriod returns suggested trust period.
//
// According to the CometBFT documentation, the sum of the trust period, the time
// required to verify headers, and the time needed to detect and penalize misbehavior
// should be significantly smaller than the total debonding period.
func calcTrustPeriod(debondingPeriod time.Duration) time.Duration {
	return debondingPeriod * 3 / 4
}

func init() {
	trustCmd.Flags().AddFlagSet(common.FormatFlag)
	common.AddSelectorNFlags(trustCmd)
}
