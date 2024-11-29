package network

import (
	"github.com/spf13/cobra"

	cliConfig "github.com/oasisprotocol/cli/config"
)

var setDefaultCmd = &cobra.Command{
	Use:   "set-default <name>",
	Short: "Sets the given network as the default network",
	Args:  cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		name := args[0]

		err := cfg.Networks.SetDefault(name)
		cobra.CheckErr(err)

		err = cfg.Save()
		cobra.CheckErr(err)
	},
}
