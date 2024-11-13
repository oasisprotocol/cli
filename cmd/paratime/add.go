package paratime

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var (
	symbol      string
	numDecimals uint
	description string

	addCmd = &cobra.Command{
		Use:   "add <network> <name> <id>",
		Short: "Add a new ParaTime",
		Args:  cobra.ExactArgs(3),
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

			paratimeInfo := struct {
				Description string
				Symbol      string
				Decimals    uint8
			}{
				Symbol:   net.Denomination.Symbol,
				Decimals: net.Denomination.Decimals,
			}

			if symbol != "" {
				paratimeInfo.Symbol = symbol
			}

			if numDecimals != 0 {
				paratimeInfo.Decimals = uint8(numDecimals)
			}

			if description != "" {
				paratimeInfo.Description = description
			}

			if !common.GetAnswerYes() {
				// Ask user for some additional parameters.
				questions := []*survey.Question{
					{
						Name: "description",
						Prompt: &survey.Input{
							Message: "Description:",
							Default: paratimeInfo.Description,
						},
					},
					{
						Name: "symbol",
						Prompt: &survey.Input{
							Message: "Denomination symbol:",
							Default: paratimeInfo.Symbol,
						},
					},
					{
						Name: "decimals",
						Prompt: &survey.Input{
							Message: "Denomination decimal places:",
							Default: fmt.Sprintf("%d", paratimeInfo.Decimals),
						},
						Validate: survey.Required,
					},
				}

				err := survey.Ask(questions, &paratimeInfo)
				cobra.CheckErr(err)
			}

			pt.Description = paratimeInfo.Description
			pt.Denominations = map[string]*config.DenominationInfo{
				config.NativeDenominationKey: {
					Symbol:   paratimeInfo.Symbol,
					Decimals: paratimeInfo.Decimals,
				},
			}
			pt.ConsensusDenomination = config.NativeDenominationKey // TODO: Make this configurable.

			err := net.ParaTimes.Add(name, &pt)
			cobra.CheckErr(err)

			err = cfg.Save()
			cobra.CheckErr(err)
		},
	}
)

func init() {
	addCmd.Flags().AddFlagSet(common.AnswerYesFlag)

	symbolFlag := flag.NewFlagSet("", flag.ContinueOnError)
	symbolFlag.StringVar(&symbol, "symbol", "", "paratime's symbol")
	addCmd.Flags().AddFlagSet(symbolFlag)

	numDecimalsFlag := flag.NewFlagSet("", flag.ContinueOnError)
	numDecimalsFlag.UintVar(&numDecimals, "num-decimals", 0, "paratime's number of decimals")
	addCmd.Flags().AddFlagSet(numDecimalsFlag)

	descriptionFlag := flag.NewFlagSet("", flag.ContinueOnError)
	descriptionFlag.StringVar(&description, "description", "", "paratime's description")
	addCmd.Flags().AddFlagSet(descriptionFlag)
}
