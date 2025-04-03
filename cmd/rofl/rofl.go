package rofl

import (
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/cmd/rofl/build"
	"github.com/oasisprotocol/cli/cmd/rofl/machine"
	"github.com/oasisprotocol/cli/cmd/rofl/provider"
)

var Cmd = &cobra.Command{
	Use:     "rofl",
	Short:   "ROFL app management",
	Aliases: []string{"r"},
}

func init() {
	Cmd.AddCommand(initCmd)
	Cmd.AddCommand(createCmd)
	Cmd.AddCommand(updateCmd)
	Cmd.AddCommand(deployCmd)
	Cmd.AddCommand(removeCmd)
	Cmd.AddCommand(showCmd)
	Cmd.AddCommand(trustRootCmd)
	Cmd.AddCommand(build.Cmd)
	Cmd.AddCommand(identityCmd)
	Cmd.AddCommand(secretCmd)
	Cmd.AddCommand(upgradeCmd)
	Cmd.AddCommand(provider.Cmd)
	Cmd.AddCommand(machine.Cmd)
}
