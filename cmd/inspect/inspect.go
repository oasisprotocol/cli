package inspect

import (
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/cmd/common"
)

// Cmd is the network inspection sub-command set root.
var Cmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect the network",
}

func init() {

	runtimeStatsCmd.Flags().AddFlagSet(common.SelectorFlags)
	runtimeStatsCmd.Flags().AddFlagSet(csvFlags)

	blockCmd.Flags().AddFlagSet(common.SelectorFlags)

	Cmd.AddCommand(runtimeStatsCmd)
	Cmd.AddCommand(blockCmd)
}
