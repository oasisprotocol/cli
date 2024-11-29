package wallet

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/oasisprotocol/cli/cmd/common"
	"github.com/oasisprotocol/cli/config"
	"github.com/oasisprotocol/cli/wallet"
)

var accKind string

var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new account",
	Args:  cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		cfg := config.Global()
		name := args[0]

		checkAccountExists(cfg, name)

		af, err := wallet.Load(accKind)
		cobra.CheckErr(err)

		// Ask for passphrase to encrypt the wallet with.
		var passphrase string
		if af.RequiresPassphrase() {
			passphrase = common.AskNewPassphrase()
		}

		accCfg := &config.Account{
			Kind: accKind,
		}
		err = accCfg.SetConfigFromFlags()
		cobra.CheckErr(err)

		err = cfg.Wallet.Create(name, passphrase, accCfg)
		cobra.CheckErr(err)

		err = cfg.Save()
		cobra.CheckErr(err)
	},
}

func checkAccountExists(cfg *config.Config, name string) {
	if _, exists := cfg.Wallet.All[name]; exists {
		cobra.CheckErr(fmt.Errorf("account '%s' already exists in the wallet", name))
	}
	if _, exists := cfg.AddressBook.All[name]; exists {
		cobra.CheckErr(fmt.Errorf("address named '%s' already exists in the address book", name))
	}
}

func init() {
	flags := flag.NewFlagSet("", flag.ContinueOnError)
	kinds := make([]string, 0, len(wallet.AvailableKinds()))
	for _, w := range wallet.AvailableKinds() {
		kinds = append(kinds, w.Kind())
	}
	flags.StringVar(&accKind, "kind", "file", fmt.Sprintf("Account kind [%s]", strings.Join(kinds, ", ")))

	// TODO: Group flags in usage by tweaking the usage template/function.
	for _, af := range wallet.AvailableKinds() {
		flags.AddFlagSet(af.Flags())
	}

	createCmd.Flags().AddFlagSet(flags)
}
