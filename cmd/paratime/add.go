package paratime

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var addCmd = &cobra.Command{
	Use:               "add <network> <name> <id>",
	Short:             "Add a new ParaTime",
	Args:              cobra.ExactArgs(3),
	ValidArgsFunction: common.NetworksAt(0), // <network> at position 1.
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		network, name, id := args[0], args[1], args[2]

		net, exists := cfg.Networks.All[network]
		if !exists {
			cobra.CheckErr(fmt.Errorf("network '%s' does not exist", network))
		}

		pt := config.ParaTime{
			ID: id,
		}
		// Validate initial paratime configuration early.
		cobra.CheckErr(config.ValidateIdentifier(name))
		cobra.CheckErr(pt.Validate())

		// Inherit the network's denomination as the default, then ask for the rest.
		details := common.DenominationDetails{
			Symbol:   net.Denomination.Symbol,
			Decimals: net.Denomination.Decimals,
		}
		common.AskDenominationDetails(&details)

		pt.Description = details.Description
		pt.Denominations = map[string]*config.DenominationInfo{
			config.NativeDenominationKey: {
				Symbol:   details.Symbol,
				Decimals: details.Decimals,
			},
		}
		pt.ConsensusDenomination = config.NativeDenominationKey // TODO: Make this configurable.

		err := net.ParaTimes.Add(name, &pt)
		cobra.CheckErr(err)

		err = cfg.Save()
		cobra.CheckErr(err)
	},
}

func init() {
	addCmd.Flags().AddFlagSet(common.AnswerYesFlag)
	addCmd.Flags().AddFlagSet(common.DenominationFlags)
}
