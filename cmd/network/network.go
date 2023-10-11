package network

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	"github.com/oasisprotocol/cli/cmd/network/governance"
)

// Cmd is the network sub-command set root.
var Cmd = &cobra.Command{
	Use:     "network",
	Short:   "Consensus layer operations",
	Aliases: []string{"n", "net"},
}

func networkDetailsFromSurvey(net *config.Network) {
	// Ask user for some additional parameters.
	questions := []*survey.Question{
		{
			Name:   "description",
			Prompt: &survey.Input{Message: "Description:"},
		},
		{
			Name:   "symbol",
			Prompt: &survey.Input{Message: "Denomination symbol:"},
		},
		{
			Name: "decimals",
			Prompt: &survey.Input{
				Message: "Denomination decimal places:",
				Default: "9",
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

	net.Description = answers.Description
	net.Denomination.Symbol = answers.Symbol
	net.Denomination.Decimals = answers.Decimals
}

func init() {
	Cmd.AddCommand(addCmd)
	Cmd.AddCommand(addLocalCmd)
	Cmd.AddCommand(governance.Cmd)
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(rmCmd)
	Cmd.AddCommand(setChainContextCmd)
	Cmd.AddCommand(setDefaultCmd)
	Cmd.AddCommand(setRPCCmd)
	Cmd.AddCommand(showCmd)
	Cmd.AddCommand(statusCmd)
}
