package paratime

import (
	"fmt"

	"github.com/spf13/cobra"

	cliConfig "github.com/oasisprotocol/cli/config"
)

var removeCmd = &cobra.Command{
	Use:     "remove <network> <name>",
	Aliases: []string{"rm"},
	Short:   "Remove an existing ParaTime",
	Args:    cobra.ExactArgs(2),
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		network, name := args[0], args[1]

		net, exists := cfg.Networks.All[network]
		if !exists {
			cobra.CheckErr(fmt.Errorf("network '%s' does not exist", network))
		}

		err := net.ParaTimes.Remove(name)
		cobra.CheckErr(err)

		err = cfg.Save()
		cobra.CheckErr(err)
	},
}
