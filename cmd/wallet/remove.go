package wallet

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/cmd/common"
	"github.com/oasisprotocol/cli/config"
)

var rmCmd = &cobra.Command{
	Use:     "remove <name> [name ...]",
	Aliases: []string{"rm"},
	Short:   "Remove existing account(s)",
	Args:    cobra.MinimumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		cfg := config.Global()

		// Remove duplicated arguments
		uniqueArgs := unique(args)

		for _, name := range uniqueArgs {
			// Early check for whether the accounts exist so that we don't ask for
			// confirmation first or delete only part of the list.
			if _, exists := cfg.Wallet.All[name]; !exists {
				cobra.CheckErr(fmt.Errorf("account '%s' does not exist", name))
			}
		}

		if !common.GetAnswerYes() {
			fmt.Printf("WARNING: Removing the account will ERASE secret key material!\n")
			fmt.Printf("WARNING: THIS ACTION IS IRREVERSIBLE!\n")

			var result string
			confirmText := fmt.Sprintf("I really want to remove accounts: %s", strings.Join(uniqueArgs, ", "))
			prompt := &survey.Input{
				Message: fmt.Sprintf("Enter '%s' (without quotes) to confirm removal:", confirmText),
			}
			err := survey.AskOne(prompt, &result)
			cobra.CheckErr(err)

			if result != confirmText {
				cobra.CheckErr("Aborted.")
			}
		}

		for _, name := range uniqueArgs {
			err := cfg.Wallet.Remove(name)
			cobra.CheckErr(err)

			err = cfg.Save()
			cobra.CheckErr(err)
		}
	},
}

func unique(stringSlice []string) []string {
	keys := make(map[string]struct{})
	list := []string{}
	for _, entry := range stringSlice {
		if _, ok := keys[entry]; !ok {
			keys[entry] = struct{}{}
			list = append(list, entry)
		}
	}
	return list
}

func init() {
	rmCmd.Flags().AddFlagSet(common.AnswerYesFlag)
}
