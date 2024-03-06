package paratime

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	cliConfig "github.com/oasisprotocol/cli/config"
)

var addCmd = &cobra.Command{
	Use:   "add <network> <name> <id>",
	Short: "Add a new ParaTime",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
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

		// Ask user for some additional parameters.
		questions := []*survey.Question{
			{
				Name:   "description",
				Prompt: &survey.Input{Message: "Description:"},
			},
			{
				Name: "symbol",
				Prompt: &survey.Input{
					Message: "Denomination symbol:",
					Default: net.Denomination.Symbol,
				},
			},
			{
				Name: "decimals",
				Prompt: &survey.Input{
					Message: "Denomination decimal places:",
					Default: fmt.Sprintf("%d", net.Denomination.Decimals),
				},
				Validate: survey.Required,
			},
		}
		answers := struct {
			Description string
			Symbol      string
			Decimals    uint8
		}{}
		err := survey.Ask(questions, &answers)
		cobra.CheckErr(err)

		pt.Description = answers.Description
		pt.Denominations = map[string]*config.DenominationInfo{
			config.NativeDenominationKey: {
				Symbol:   answers.Symbol,
				Decimals: answers.Decimals,
			},
		}
		pt.ConsensusDenomination = config.NativeDenominationKey // TODO: Make this configurable.

		err = net.ParaTimes.Add(name, &pt)
		cobra.CheckErr(err)

		err = cfg.Save()
		cobra.CheckErr(err)
	},
}
