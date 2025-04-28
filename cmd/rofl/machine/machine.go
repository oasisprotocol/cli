package machine

import (
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:     "machine",
	Short:   "ROFL machine management commands",
	Aliases: []string{"i"},
}

func init() {
	Cmd.AddCommand(showCmd)
	Cmd.AddCommand(restartCmd)
	Cmd.AddCommand(stopCmd)
	Cmd.AddCommand(removeCmd)
	Cmd.AddCommand(topUpCmd)
}
