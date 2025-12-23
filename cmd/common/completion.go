package common

import (
	"sort"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/config"
)

// CobraCompletionFunc is the function signature for cobra completions.
type CobraCompletionFunc = func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)

// Helpers.

func mapKeys[T any](m map[string]T) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	return names
}

func noFileComp(names []string) ([]string, cobra.ShellCompDirective) {
	sort.Strings(names)
	return names, cobra.ShellCompDirectiveNoFileComp
}

func simpleComplete(fn func() []string) CobraCompletionFunc {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return noFileComp(fn())
	}
}

func completeAt(fn func() []string, positions ...int) CobraCompletionFunc {
	return func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		for _, pos := range positions {
			if len(args) == pos {
				return noFileComp(fn())
			}
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

// Data sources.

func accountNames() []string {
	return mapKeys(config.Global().Wallet.All)
}

func addressBookNames() []string {
	return mapKeys(config.Global().AddressBook.All)
}

func addressNames() []string {
	cfg := config.Global()
	return append(mapKeys(cfg.Wallet.All), mapKeys(cfg.AddressBook.All)...)
}

func networkNames() []string {
	return mapKeys(config.Global().Networks.All)
}

// Simple completers - complete at any position.

// CompleteAccountNames provides completion for wallet account names.
var CompleteAccountNames = simpleComplete(accountNames)

// CompleteAddressBookNames provides completion for address book entry names.
var CompleteAddressBookNames = simpleComplete(addressBookNames)

// CompleteAccountAndAddressBookNames provides completion for both wallet accounts
// and address book entries. Useful for commands that accept either as an address.
var CompleteAccountAndAddressBookNames = simpleComplete(addressNames)

// CompleteNetworkNames provides completion for network names.
var CompleteNetworkNames = simpleComplete(networkNames)

// CompleteParaTimeNames provides completion for ParaTime names.
// It uses the default network if --network flag is not set.
func CompleteParaTimeNames(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	cfg := config.Global()

	// Get network from flag or use default.
	networkName, _ := cmd.Flags().GetString("network")
	if networkName == "" {
		networkName = cfg.Networks.Default
	}
	if networkName == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	network := cfg.Networks.All[networkName]
	if network == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return noFileComp(mapKeys(network.ParaTimes.All))
}

// CompleteNetworkThenParaTime provides completion for commands that take
// <network> <paratime> as positional arguments.
func CompleteNetworkThenParaTime(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	// First argument: complete network names.
	if len(args) == 0 {
		return noFileComp(networkNames())
	}

	// Second argument: complete paratime names for the given network.
	if len(args) == 1 {
		network := config.Global().Networks.All[args[0]]
		if network == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return noFileComp(mapKeys(network.ParaTimes.All))
	}

	return nil, cobra.ShellCompDirectiveNoFileComp
}

// Position-aware completers - complete only at specified positions.

// AddressesAt returns a completion function for wallet + address book names at specified positions.
func AddressesAt(positions ...int) CobraCompletionFunc {
	return completeAt(addressNames, positions...)
}

// NetworksAt returns a completion function for network names at specified positions.
func NetworksAt(positions ...int) CobraCompletionFunc {
	return completeAt(networkNames, positions...)
}

// StaticAt returns a completion function for static values at specified positions.
func StaticAt(values []string, positions ...int) CobraCompletionFunc {
	return completeAt(func() []string { return values }, positions...)
}

// RegisterSelectorCompletions registers completion functions for the common
// selector flags (--network, --paratime, --account) on the given command.
func RegisterSelectorCompletions(cmd *cobra.Command) {
	_ = cmd.RegisterFlagCompletionFunc("network", CompleteNetworkNames)
	_ = cmd.RegisterFlagCompletionFunc("paratime", CompleteParaTimeNames)
	_ = cmd.RegisterFlagCompletionFunc("account", CompleteAccountNames)
}
