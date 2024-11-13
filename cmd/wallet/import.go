package wallet

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/oasisprotocol/cli/cmd/common"
	"github.com/oasisprotocol/cli/config"
	"github.com/oasisprotocol/cli/wallet"
	walletFile "github.com/oasisprotocol/cli/wallet/file"
)

var (
	algorithm string
	number    uint32
	secret    string

	passphrase string
	accCfg     *config.Account
	kind       wallet.ImportKind

	importCmd = &cobra.Command{
		Use:   "import <name>",
		Short: "Import an existing account",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg := config.Global()
			name := args[0]

			checkAccountExists(cfg, name)

			// NOTE: We only support importing into the file-based wallet for now.
			af, err := wallet.Load(walletFile.Kind)
			cobra.CheckErr(err)

			afCfg := make(map[string]interface{})

			if !common.GetAnswerYes() {
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

				err = kind.UnmarshalText([]byte(kindRaw))
				cobra.CheckErr(err)

				// Ask for wallet configuration.
				afCfg, err = af.GetConfigFromSurvey(&kind)
				cobra.CheckErr(err)
			} else {
				afCfg["algorithm"] = algorithm
				afCfg["number"] = number
				switch algorithm {
				case wallet.AlgorithmEd25519Raw, wallet.AlgorithmSecp256k1Raw, wallet.AlgorithmSr25519Raw:
					kind = wallet.ImportKindPrivateKey
				default:
					kind = wallet.ImportKindMnemonic
				}
			}

			accCfg = &config.Account{
				Kind:   af.Kind(),
				Config: afCfg,
			}

			var src *wallet.ImportSource
			if !common.GetAnswerYes() {
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
				passphrase = common.AskNewPassphrase()

				src = &wallet.ImportSource{
					Kind: kind,
					Data: answers.Data,
				}
			} else {
				src = &wallet.ImportSource{
					Kind: kind,
					Data: secret,
				}
			}

			err = cfg.Wallet.Import(name, passphrase, accCfg, src)
			cobra.CheckErr(err)

			err = cfg.Save()
			cobra.CheckErr(err)
		},
	}
)

func init() {
	importCmd.Flags().AddFlagSet(common.AnswerYesFlag)

	algorithmFlag := flag.NewFlagSet("", flag.ContinueOnError)
	algorithmFlag.StringVar(&algorithm, "algorithm", wallet.AlgorithmEd25519Raw, fmt.Sprintf("Cryptographic algorithm to use for this account [%s]", strings.Join(walletFile.SupportedAlgorithmsForImport(nil), ", ")))
	importCmd.Flags().AddFlagSet(algorithmFlag)

	numberFlag := flag.NewFlagSet("", flag.ContinueOnError)
	numberFlag.Uint32Var(&number, "number", 0, "Key number to use in the key derivation scheme")
	importCmd.Flags().AddFlagSet(numberFlag)

	secretFlag := flag.NewFlagSet("", flag.ContinueOnError)
	secretFlag.StringVar(&secret, "secret", "", "A secret key or mnemonic to use for this account")
	importCmd.Flags().AddFlagSet(secretFlag)
}
