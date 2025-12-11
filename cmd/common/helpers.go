package common

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	configSdk "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	sdkhelpers "github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/testing"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	buildRoflProvider "github.com/oasisprotocol/cli/build/rofl/provider"
	"github.com/oasisprotocol/cli/config"
)

// Warnf prints a message to stderr with formatting.
func Warnf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// Warn prints a message to stderr.
func Warn(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}

// CheckForceErr treats error as warning, if --force is provided.
func CheckForceErr(err interface{}) {
	// No error.
	if err == nil {
		return
	}

	// --force is provided.
	if IsForce() {
		Warnf("Warning: %s\nProceeding by force as requested", err)
		return
	}

	// Print error with --force hint and quit.
	errMsg := fmt.Sprintf("%s", err)
	errMsg += "\nUse --force to ignore this check"
	cobra.CheckErr(errMsg)
}

// GenAccountNames generates a map of all addresses -> account name for pretty printing.
func GenAccountNames() types.AccountNames {
	an := types.AccountNames{}
	for name, acc := range config.Global().Wallet.All {
		an[acc.GetAddress().String()] = name
	}

	for name, acc := range config.Global().AddressBook.All {
		an[acc.GetAddress().String()] = name
	}

	for name, acc := range testing.TestAccounts {
		an[acc.Address.String()] = fmt.Sprintf("test:%s", name)
	}

	return an
}

func GenAccountNamesForNetwork(net *configSdk.Network) types.AccountNames {
	an := GenAccountNames()

	// Include ParaTime native addresses as paratime:<name> for the selected network.
	if net != nil {
		for ptName, pt := range net.ParaTimes.All {
			rtAddr := types.NewAddressFromConsensus(staking.NewRuntimeAddress(pt.Namespace()))
			if _, exists := an[rtAddr.String()]; !exists {
				an[rtAddr.String()] = fmt.Sprintf("paratime:%s", ptName)
			}
		}

		// Include ROFL default provider addresses as rofl:provider:<paratime>.
		for ptName, pt := range net.ParaTimes.All {
			if svc, ok := buildRoflProvider.DefaultRoflServices[pt.ID]; ok {
				if svc.Provider != "" {
					if a, _, err := sdkhelpers.ResolveEthOrOasisAddress(svc.Provider); err == nil && a != nil {
						if _, exists := an[a.String()]; !exists {
							an[a.String()] = fmt.Sprintf("rofl:provider:%s", ptName)
						}
					}
				}
			}
		}
	}

	return an
}

func GenAccountEthMap(_ *configSdk.Network) map[string]string {
	em := make(map[string]string)

	// Address book entries which may carry an Ethereum address.
	for _, entry := range config.Global().AddressBook.All {
		if ea := entry.GetEthAddress(); ea != nil {
			em[entry.GetAddress().String()] = ea.Hex()
		}
	}

	// Built-in test accounts (many have both forms).
	for _, acc := range testing.TestAccounts {
		if acc.EthAddress != nil {
			em[acc.Address.String()] = acc.EthAddress.Hex()
		}
	}

	return em
}

func PrettyAddress(net *configSdk.Network, addr types.Address) string {
	name := GenAccountNamesForNetwork(net)[addr.String()]
	if name == "" {
		// Unknown address; return the native form as-is.
		return addr.String()
	}

	if eth := GenAccountEthMap(net)[addr.String()]; eth != "" {
		return fmt.Sprintf("%s (%s)", name, eth)
	}
	return fmt.Sprintf("%s (%s)", name, addr.String())
}

// FindAccountName finds account's name (if exists).
func FindAccountName(address string) string {
	an := GenAccountNames()
	return an[address]
}

// FindAccountNameForNetwork finds account's name in the context of a specific network.
func FindAccountNameForNetwork(net *configSdk.Network, address string) string {
	an := GenAccountNamesForNetwork(net)
	return an[address]
}
