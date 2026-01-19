package common

import (
	"fmt"
	"os"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	configSdk "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
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

// GenAccountNames generates a map of all addresses -> account name for pretty printing.
// Priority order (later entries overwrite earlier): test accounts < addressbook < wallet.
// This ensures wallet identity takes precedence over addressbook labels for the same address.
func GenAccountNames() types.AccountNames {
	return GenAccountNamesForNetwork(nil)
}

// GenAccountNamesForNetwork generates a map of all addresses -> account name for pretty printing,
// including network-specific entries (paratimes, ROFL providers) when a network is provided.
// Priority order (later entries overwrite earlier): test accounts < network entries < addressbook < wallet.
func GenAccountNamesForNetwork(net *configSdk.Network) types.AccountNames {
	an := types.AccountNames{}

	// Test accounts have lowest priority.
	for name, acc := range testing.TestAccounts {
		an[acc.Address.String()] = fmt.Sprintf("test:%s", name)
	}

	// Network-derived entries (paratimes, ROFL providers) have second-lowest priority.
	if net != nil {
		// Include ParaTime runtime addresses as paratime:<name>.
		for ptName, pt := range net.ParaTimes.All {
			rtAddr := types.NewAddressFromConsensus(staking.NewRuntimeAddress(pt.Namespace()))
			an[rtAddr.String()] = fmt.Sprintf("paratime:%s", ptName)
		}

		// Include ROFL default provider addresses as rofl:provider:<paratime>.
		for ptName, pt := range net.ParaTimes.All {
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
	for name, acc := range config.Global().AddressBook.All {
		an[acc.GetAddress().String()] = name
	}

	// Wallet entries have highest priority.
	for name, acc := range config.Global().Wallet.All {
		an[acc.GetAddress().String()] = name
	}

	return an
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

// AddressFormatContext contains precomputed maps for address formatting.
type AddressFormatContext struct {
	// Names maps native address string to account name.
	Names types.AccountNames
	// Eth maps native address string to Ethereum hex address string (if known).
	Eth map[string]string
}

// GenAccountEthMap generates a map of native address string -> eth hex address (if known).
func GenAccountEthMap() map[string]string {
	return GenAccountEthMapForNetwork(nil)
}

// GenAccountEthMapForNetwork generates a map of native address string -> eth hex address (if known).
func GenAccountEthMapForNetwork(_ *configSdk.Network) map[string]string {
	eth := make(map[string]string)

	// From test accounts.
	for _, acc := range testing.TestAccounts {
		if acc.EthAddress != nil {
			eth[acc.Address.String()] = acc.EthAddress.Hex()
		}
	}

	// From address book entries (higher priority than test accounts).
	for _, acc := range config.Global().AddressBook.All {
		if ethAddr := acc.GetEthAddress(); ethAddr != nil {
			eth[acc.GetAddress().String()] = ethAddr.Hex()
		}
	}

	// From wallet entries (highest priority), when an Ethereum address is available in config.
	for _, acc := range config.Global().Wallet.All {
		if ethAddr := acc.GetEthAddress(); ethAddr != nil {
			eth[acc.GetAddress().String()] = ethAddr.Hex()
		}
	}

	return eth
}

// GenAddressFormatContext builds both name and eth address maps for formatting.
func GenAddressFormatContext() AddressFormatContext {
	return GenAddressFormatContextForNetwork(nil)
}

// GenAddressFormatContextForNetwork builds both name and eth address maps for formatting,
// including network-specific entries when a network is provided.
func GenAddressFormatContextForNetwork(net *configSdk.Network) AddressFormatContext {
	return AddressFormatContext{
		Names: GenAccountNamesForNetwork(net),
		Eth:   GenAccountEthMapForNetwork(net),
	}
}

// PrettyAddressWith formats an address using a precomputed context.
// If the address is known (in wallet, addressbook, or test accounts), it returns "name (address)".
// For secp256k1 accounts with a known Ethereum address, the Ethereum hex format is preferred in parentheses.
// If the address is unknown, it returns the original address string unchanged.
func PrettyAddressWith(ctx AddressFormatContext, addr string) string {
	// Try to parse the address to get canonical native form.
	nativeAddr, ethFromInput, err := helpers.ResolveEthOrOasisAddress(addr)
	if err != nil || nativeAddr == nil {
		// Cannot parse, return unchanged.
		return addr
	}

	nativeStr := nativeAddr.String()

	// Look up the name.
	name := ctx.Names[nativeStr]
	if name == "" {
		// Unknown address, return the original input.
		return addr
	}

	// Determine which address to show in parentheses.
	// Prefer Ethereum address if available (from input or from known eth addresses).
	var parenAddr string
	if ethFromInput != nil {
		parenAddr = ethFromInput.Hex()
	} else if ethHex := ctx.Eth[nativeStr]; ethHex != "" {
		parenAddr = ethHex
	} else {
		parenAddr = nativeStr
	}

	// Guard against redundant "name (name)" output.
	if name == parenAddr {
		return parenAddr
	}

	return fmt.Sprintf("%s (%s)", name, parenAddr)
}

func preferredAddressString(nativeAddr *types.Address, ethAddr *ethCommon.Address) string {
	if ethAddr != nil {
		return ethAddr.Hex()
	}
	if nativeAddr == nil {
		return ""
	}
	return nativeAddr.String()
}

// PrettyResolvedAddressWith formats a resolved address tuple (native, eth) using a precomputed context.
//
// If ethAddr is non-nil, it is preferred to preserve the "user provided eth" behavior even when
// the native-to-eth mapping is not available in the context.
func PrettyResolvedAddressWith(ctx AddressFormatContext, nativeAddr *types.Address, ethAddr *ethCommon.Address) string {
	addrStr := preferredAddressString(nativeAddr, ethAddr)
	if addrStr == "" {
		return ""
	}
	return PrettyAddressWith(ctx, addrStr)
}

// PrettyAddress formats an address for display without network context.
// If the address is known (in wallet, addressbook, or test accounts), it returns "name (address)".
// For secp256k1 accounts with a known Ethereum address, the Ethereum hex format is preferred in parentheses.
// If the address is unknown, it returns the original address string unchanged.
func PrettyAddress(addr string) string {
	return PrettyAddressWith(GenAddressFormatContext(), addr)
}

// PrettyAddressForNetwork formats an address for display with network context.
// This includes network-derived names like paratime addresses and ROFL providers.
func PrettyAddressForNetwork(net *configSdk.Network, addr types.Address) string {
	ctx := GenAddressFormatContextForNetwork(net)
	return PrettyAddressWith(ctx, addr.String())
}
