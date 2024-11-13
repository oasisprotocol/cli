package denomination

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	cliConfig "github.com/oasisprotocol/cli/config"
)

var (
	symbol string

	setDenomCmd = &cobra.Command{
		Use:   "set <network> <paratime> <denomination> <number_of_decimals> [--symbol <symbol>]",
		Short: "Set denomination",
		Args:  cobra.ExactArgs(4),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			networkArg, ptArg, denomArg, decimalsArg := args[0], args[1], args[2], args[3]

			if symbol == "" {
				symbol = denomArg
			}

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
				Symbol:   symbol,
				Decimals: uint8(decimalsInt),
			}
			pt.Denominations[denomArg] = denomInfo

			err = cfg.Save()
			cobra.CheckErr(err)
		},
	}
)

func init() {
	symbolFlag := flag.NewFlagSet("", flag.ContinueOnError)
	symbolFlag.StringVar(&symbol, "symbol", "", "Denomination symbol")
	setDenomCmd.Flags().AddFlagSet(symbolFlag)
}
