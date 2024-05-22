package paratime

import (
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/cmd/paratime/denomination"
)

var Cmd = &cobra.Command{
	Use:     "paratime",
	Short:   "ParaTime layer operations",
	Aliases: []string{"p", "pt"},
}

func init() {
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(addCmd)
	Cmd.AddCommand(registerCmd)
	Cmd.AddCommand(removeCmd)
	Cmd.AddCommand(setDefaultCmd)
	Cmd.AddCommand(showCmd)
	Cmd.AddCommand(statsCmd)
	Cmd.AddCommand(denomination.Cmd)
}
