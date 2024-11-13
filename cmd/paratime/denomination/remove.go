package denomination

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	cliConfig "github.com/oasisprotocol/cli/config"
)

var removeDenomCmd = &cobra.Command{
	Use:   "remove <network> <paratime> <denomination>",
	Short: "Remove denomination",
	Args:  cobra.ExactArgs(3),
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		networkArg, ptArg, denomArg := args[0], args[1], args[2]

		if denomArg == config.NativeDenominationKey {
			cobra.CheckErr(fmt.Errorf("you cannot delete native denomination"))
			return
		}

		net := cfg.Networks.All[networkArg]
		if net == nil {
			cobra.CheckErr(fmt.Errorf("network '%s' does not exist", networkArg))
			return
		}

		pt := net.ParaTimes.All[ptArg]
		if pt == nil {
			cobra.CheckErr(fmt.Errorf("pratime '%s' does not exist", ptArg))
			return
		}

		denomArg = strings.ToLower(denomArg)
		_, ok := pt.Denominations[denomArg]
		if ok {
			delete(pt.Denominations, denomArg)
		} else {
			cobra.CheckErr(fmt.Errorf("denomination '%s' does not exist", denomArg))
			return
		}

		err := cfg.Save()
		cobra.CheckErr(err)
	},
}
