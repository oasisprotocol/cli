package provider

import (
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:     "provider",
	Short:   "ROFL provider commands",
	Aliases: []string{"p"},
}

func init() {
	Cmd.AddCommand(initCmd)
	Cmd.AddCommand(createCmd)
	Cmd.AddCommand(updateCmd)
	Cmd.AddCommand(updateOffersCmd)
	Cmd.AddCommand(removeCmd)
}
