package rofl

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	buildRofl "github.com/oasisprotocol/cli/build/rofl"
	"github.com/oasisprotocol/cli/cmd/common"
	roflCommon "github.com/oasisprotocol/cli/cmd/rofl/common"
)

var setTrustRootCmd = &cobra.Command{
	Use:   "set-trust-root [<height>] [<hash>]",
	Short: "Set the trust root in the ROFL app manifest",
	Long: "Set the consensus block height and hash of the trust root in the ROFL app manifest.\n\n" +
		"If no arguments are passed, the latest finalized block is used. If only the height is\n" +
		"passed, the corresponding block hash is fetched from the network.",
	Args: cobra.RangeArgs(0, 2),
	Run: func(_ *cobra.Command, args []string) {
		// Load manifest and deployment, and configure network/paratime/account selection.
		manifest, deployment, npa := roflCommon.LoadManifestAndSetNPA(nil)

		var (
			height uint64
			hash   string
		)
		switch len(args) {
		case 2:
			// Both height and hash passed; set them directly without querying the network.
			h, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("malformed height: %w", err))
			}
			height = h
			hash = args[1]
		default:
			// Determine the block height to query (latest by default).
			queryHeight := consensus.HeightLatest
			if len(args) == 1 {
				h, err := strconv.ParseInt(args[0], 10, 64)
				if err != nil {
					cobra.CheckErr(fmt.Errorf("malformed height: %w", err))
				}
				queryHeight = h
			}

			// Establish connection with the target network and fetch the block.
			ctx := context.Background()
			conn, err := connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)

			blk, err := conn.Consensus().Core().GetBlock(ctx, queryHeight)
			cobra.CheckErr(err)

			height = uint64(blk.Height)
			hash = blk.Hash.Hex()
		}

		if deployment.TrustRoot == nil {
			deployment.TrustRoot = &buildRofl.TrustRootConfig{}
		}
		deployment.TrustRoot.Height = height
		deployment.TrustRoot.Hash = hash

		if err := manifest.Save(); err != nil {
			cobra.CheckErr(fmt.Errorf("failed to update manifest: %w", err))
		}

		out, err := common.PrettyJSONMarshal(deployment.TrustRoot)
		cobra.CheckErr(err)
		fmt.Printf("Updated '%s' with trust root:\n%s\n", manifest.SourceFileName(), string(out))
	},
}

func init() {
	setTrustRootCmd.Flags().AddFlagSet(common.SelectorNPFlags)
	setTrustRootCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
}
