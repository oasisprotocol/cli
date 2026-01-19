package common

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
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

// GenAccountNames generates a map of all known native addresses -> account name for pretty printing.
// It includes test accounts, configured networks (paratimes/ROFL defaults), addressbook and wallet.
//
// Priority order (later entries overwrite earlier):
// test accounts < network entries < addressbook < wallet.
func GenAccountNames() types.AccountNames {
	an := types.AccountNames{}

	// Test accounts have lowest priority.
	for name, acc := range testing.TestAccounts {
		an[acc.Address.String()] = fmt.Sprintf("test:%s", name)
	}

	// Network-derived entries (paratimes, ROFL providers) have second-lowest priority.
	cfg := config.Global()
	netNames := make([]string, 0, len(cfg.Networks.All))
	for name := range cfg.Networks.All {
		netNames = append(netNames, name)
	}
	sort.Strings(netNames)
	for _, netName := range netNames {
		net := cfg.Networks.All[netName]
		if net == nil {
			continue
		}

		// Include ParaTime runtime addresses as paratime:<name>.
		ptNames := make([]string, 0, len(net.ParaTimes.All))
		for ptName := range net.ParaTimes.All {
			ptNames = append(ptNames, ptName)
		}
		sort.Strings(ptNames)
		for _, ptName := range ptNames {
			pt := net.ParaTimes.All[ptName]
			if pt == nil {
				continue
			}

			rtAddr := types.NewAddressFromConsensus(staking.NewRuntimeAddress(pt.Namespace()))
			an[rtAddr.String()] = fmt.Sprintf("paratime:%s", ptName)

			// Include ROFL default provider addresses as rofl:provider:<paratime>.
			if svc, ok := buildRoflProvider.DefaultRoflServices[pt.ID]; ok {
				if svc.Provider != "" {
					if a, _, err := helpers.ResolveEthOrOasisAddress(svc.Provider); err == nil && a != nil {
						an[a.String()] = fmt.Sprintf("rofl:provider:%s", ptName)
					}
				}
			}
		}
	}

	// Addressbook entries have medium priority.
	for name, acc := range cfg.AddressBook.All {
		an[acc.GetAddress().String()] = name
	}

	// Wallet entries have highest priority.
	for name, acc := range cfg.Wallet.All {
		an[acc.GetAddress().String()] = name
	}

	return an
}

// FindAccountName finds account's name (if exists).
func FindAccountName(address string) string {
	an := GenAccountNames()
	return an[address]
}

// AddressFormatContext contains precomputed maps for address formatting.
type AddressFormatContext struct {
	// Names maps native address string to account name.
	Names types.AccountNames
	// Eth maps native address string to Ethereum hex address string, if known.
	Eth map[string]string
}

// GenAccountEthMap generates a map of native address string -> eth hex address (if known).
// Priority order matches GenAccountNames: test accounts < addressbook < wallet.
func GenAccountEthMap() map[string]string {
	eth := make(map[string]string)

	for _, acc := range testing.TestAccounts {
		if acc.EthAddress != nil {
			eth[acc.Address.String()] = acc.EthAddress.Hex()
		}
	}

	for _, acc := range config.Global().AddressBook.All {
		if ethAddr := acc.GetEthAddress(); ethAddr != nil {
			eth[acc.GetAddress().String()] = ethAddr.Hex()
		}
	}

	for _, acc := range config.Global().Wallet.All {
		if ethAddr := acc.GetEthAddress(); ethAddr != nil {
			eth[acc.GetAddress().String()] = ethAddr.Hex()
		}
	}

	return eth
}

// GenAddressFormatContext builds both name and eth address maps for formatting.
func GenAddressFormatContext() AddressFormatContext {
	return AddressFormatContext{
		Names: GenAccountNames(),
		Eth:   GenAccountEthMap(),
	}
}

// PrettyAddressWith formats the address in native or eth string for display using a precomputed context.
// Known addresses return "name (preferred_addr)", unknown addresses return the input unchanged.
func PrettyAddressWith(ctx AddressFormatContext, addr string) string {
	nativeAddr, ethFromInput, err := helpers.ResolveEthOrOasisAddress(addr)
	if err != nil || nativeAddr == nil {
		return addr
	}

	nativeStr := nativeAddr.String()

	name := ctx.Names[nativeStr]
	if name == "" {
		return addr
	}

	// Prefer eth address in parentheses when available.
	var parenAddr string
	if ethFromInput != nil {
		parenAddr = ethFromInput.Hex()
	} else if ethHex := ctx.Eth[nativeStr]; ethHex != "" {
		parenAddr = ethHex
	} else {
		parenAddr = nativeStr
	}

	if name == parenAddr {
		return parenAddr
	}

	return fmt.Sprintf("%s (%s)", name, parenAddr)
}

// PrettyAddress is like PrettyAddressWith but builds a fresh context on each call.
func PrettyAddress(addr string) string {
	return PrettyAddressWith(GenAddressFormatContext(), addr)
}
