package rofl

import (
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:     "rofl",
	Short:   "ROFL app management",
	Aliases: []string{"r"},
}

func init() {
	Cmd.AddCommand(createCmd)
	Cmd.AddCommand(updateCmd)
	Cmd.AddCommand(removeCmd)
	Cmd.AddCommand(showCmd)
	Cmd.AddCommand(trustRootCmd)
	Cmd.AddCommand(buildCmd)
	Cmd.AddCommand(identityCmd)
}
