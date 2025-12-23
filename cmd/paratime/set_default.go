package paratime

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var setDefaultCmd = &cobra.Command{
	Use:               "set-default <network> <name>",
	Short:             "Sets the given ParaTime as the default ParaTime for the given network",
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: common.CompleteNetworkThenParaTime,
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		network, name := args[0], args[1]

		net, exists := cfg.Networks.All[network]
		if !exists {
			cobra.CheckErr(fmt.Errorf("network '%s' does not exist", network))
		}

		err := net.ParaTimes.SetDefault(name)
		cobra.CheckErr(err)

		err = cfg.Save()
		cobra.CheckErr(err)
	},
}
