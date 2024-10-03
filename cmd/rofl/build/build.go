package build

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	"github.com/oasisprotocol/cli/cmd/common"
)

// Build modes.
const (
	buildModeProduction = "production"
	buildModeUnsafe     = "unsafe"
	buildModeAuto       = "auto"
)

var (
	outputFn  string
	buildMode string
	offline   bool

	Cmd = &cobra.Command{
		Use:   "build",
		Short: "Build a ROFL application",
	}
)

func detectBuildMode(npa *common.NPASelection) {
	// Configure build mode. In case auto is selected and not offline, query the network. If
	// autodetection fails, default to production mode.
	switch {
	case buildMode == buildModeAuto && !offline:
		ctx := context.Background()
		conn, err := connection.Connect(ctx, npa.Network)
		if err != nil {
			cobra.CheckErr(fmt.Errorf("unable to autodetect build mode, please provide --mode flag manually (failed to connect to GRPC endpoint: %w)", err))
		}

		params, err := conn.Consensus().Registry().ConsensusParameters(ctx, consensus.HeightLatest)
		if err != nil {
			cobra.CheckErr(fmt.Errorf("unable to autodetect build mode, please provide --mode flag manually (failed to get consensus parameters: %w)", err))
		}

		if params.DebugAllowTestRuntimes {
			buildMode = buildModeUnsafe
		}
	default:
	}
}

func init() {
	globalFlags := flag.NewFlagSet("", flag.ContinueOnError)
	globalFlags.StringVar(&buildMode, "mode", "auto", "build mode [production, unsafe, auto]")
	globalFlags.BoolVar(&offline, "offline", false, "do not perform any operations requiring network access")
	globalFlags.StringVar(&outputFn, "output", "", "output bundle filename")

	Cmd.PersistentFlags().AddFlagSet(globalFlags)
	Cmd.AddCommand(sgxCmd)
	Cmd.AddCommand(tdxCmd)
}
