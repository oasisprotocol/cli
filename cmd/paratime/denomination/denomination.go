package denomination

import (
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:     "denomination",
	Short:   "Denomination operations",
	Aliases: []string{"denom"},
}

func init() {
	Cmd.AddCommand(setDenomCmd)
	Cmd.AddCommand(setNativeDenomCmd)
	Cmd.AddCommand(removeDenomCmd)
}
