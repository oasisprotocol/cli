package wallet

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/config"
)

var renameCmd = &cobra.Command{
	Use:     "rename <old> <new>",
	Short:   "Rename an existing account",
	Aliases: []string{"mv"},
	Args:    cobra.ExactArgs(2),
	Run: func(_ *cobra.Command, args []string) {
		cfg := config.Global()
		oldName, newName := args[0], args[1]

		if _, exists := cfg.AddressBook.All[newName]; exists {
			cobra.CheckErr(fmt.Errorf("address named '%s' already exists in the address book", newName))
		}
		err := cfg.Wallet.Rename(oldName, newName)
		cobra.CheckErr(err)

		err = cfg.Save()
		cobra.CheckErr(err)
	},
}
