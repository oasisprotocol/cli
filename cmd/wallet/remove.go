package wallet

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/config"
)

var rmCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm"},
	Short:   "Remove an existing account",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Global()
		name := args[0]

		// Early check for whether the wallet exists so that we don't ask for confirmation first.
		if _, exists := cfg.Wallet.All[name]; !exists {
			cobra.CheckErr(fmt.Errorf("account '%s' does not exist", name))
		}

		fmt.Printf("WARNING: Removing the account will ERASE secret key material!\n")
		fmt.Printf("WARNING: THIS ACTION IS IRREVERSIBLE!\n")

		var result string
		confirmText := fmt.Sprintf("I really want to remove account %s", name)
		prompt := &survey.Input{
			Message: fmt.Sprintf("Enter '%s' (without quotes) to confirm removal:", confirmText),
		}
		err := survey.AskOne(prompt, &result)
		cobra.CheckErr(err)

		if result != confirmText {
			cobra.CheckErr("Aborted.")
		}

		err = cfg.Wallet.Remove(name)
		cobra.CheckErr(err)

		err = cfg.Save()
		cobra.CheckErr(err)
	},
}
