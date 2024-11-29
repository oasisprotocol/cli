package denomination

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	cliConfig "github.com/oasisprotocol/cli/config"
)

var setNativeDenomCmd = &cobra.Command{
	Use:   "set-native <network> <paratime> <symbol> <number_of_decimals>",
	Short: "Set native denomination",
	Args:  cobra.ExactArgs(4),
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		networkArg, ptArg, symbolArg, decimalsArg := args[0], args[1], args[2], args[3]

		decimalsInt, err := strconv.Atoi(decimalsArg)
		if err != nil {
			cobra.CheckErr(fmt.Errorf("number of decimals '%s' cannot be converted to integer", decimalsArg))
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

		denomInfo := &config.DenominationInfo{
			Symbol:   symbolArg,
			Decimals: uint8(decimalsInt),
		}
		pt.Denominations[config.NativeDenominationKey] = denomInfo

		err = cfg.Save()
		cobra.CheckErr(err)
	},
}
