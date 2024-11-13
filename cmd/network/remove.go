package network

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var rmCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm"},
	Short:   "Remove an existing network",
	Args:    cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		name := args[0]

		net, exists := cfg.Networks.All[name]
		if !exists {
			cobra.CheckErr(fmt.Errorf("network '%s' does not exist", name))
		}

		if !common.GetAnswerYes() && len(net.ParaTimes.All) > 0 {
			fmt.Printf("WARNING: Network '%s' contains %d ParaTimes.\n", name, len(net.ParaTimes.All))
			common.Confirm("Are you sure you want to remove the network?", "not removing network")
		}

		err := cfg.Networks.Remove(name)
		cobra.CheckErr(err)

		err = cfg.Save()
		cobra.CheckErr(err)
	},
}

func init() {
	rmCmd.Flags().AddFlagSet(common.AnswerYesFlag)
}
