package wallet

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/cmd/common"
	"github.com/oasisprotocol/cli/config"
	"github.com/oasisprotocol/cli/wallet"
	walletFile "github.com/oasisprotocol/cli/wallet/file"
)

var importCmd = &cobra.Command{
	Use:   "import <name>",
	Short: "Import an existing account",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Global()
		name := args[0]

		checkAccountExists(cfg, name)

		// NOTE: We only support importing into the file-based wallet for now.
		af, err := wallet.Load(walletFile.Kind)
		cobra.CheckErr(err)

		// Ask for import kind.
		var supportedKinds []string
		for _, kind := range af.SupportedImportKinds() {
			supportedKinds = append(supportedKinds, string(kind))
		}

		var kindRaw string
		err = survey.AskOne(&survey.Select{
			Message: "Kind:",
			Options: supportedKinds,
		}, &kindRaw)
		cobra.CheckErr(err)

		var kind wallet.ImportKind
		err = kind.UnmarshalText([]byte(kindRaw))
		cobra.CheckErr(err)

		// Ask for wallet configuration.
		afCfg, err := af.GetConfigFromSurvey(&kind)
		cobra.CheckErr(err)

		// Ask for import data.
		var answers struct {
			Data string
		}
		questions := []*survey.Question{
			{
				Name:     "data",
				Prompt:   af.DataPrompt(kind, afCfg),
				Validate: af.DataValidator(kind, afCfg),
			},
		}
		err = survey.Ask(questions, &answers)
		cobra.CheckErr(err)

		// Ask for passphrase.
		passphrase := common.AskNewPassphrase()

		accCfg := &config.Account{
			Kind:   af.Kind(),
			Config: afCfg,
		}
		src := &wallet.ImportSource{
			Kind: kind,
			Data: answers.Data,
		}

		err = cfg.Wallet.Import(name, passphrase, accCfg, src)
		cobra.CheckErr(err)

		err = cfg.Save()
		cobra.CheckErr(err)
	},
}
