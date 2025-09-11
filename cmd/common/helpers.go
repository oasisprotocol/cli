package common

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/testing"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/config"

	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
)

// PrettyPrintContext builds a context carrying data the SDK pretty-printers can use:
//   - sdkConfig.ContextKeyParaTimeCfg:   ParaTime configuration (*config.ParaTime) for denomination formatting,
//   - types.ContextKeyAccountNames:      mapping for friendly names.
//
// NOTE: We intentionally do NOT set an ETH address map here to stay compatible with
// older oasis-sdk versions. The CLI shows both addresses explicitly via helpers.
func PrettyPrintContext(npa *NPASelection) context.Context {
	ctx := context.Background()

	if npa != nil && npa.ParaTime != nil {
		ctx = context.WithValue(ctx, sdkConfig.ContextKeyParaTimeCfg, npa.ParaTime)
	}

	// Provide friendly names for pretty printers.
	ctx = context.WithValue(ctx, types.ContextKeyAccountNames, GenKnownNames(npa.Network))

	return ctx
}

// CheckForceErr treats error as warning, if --force is provided.
func CheckForceErr(err interface{}) {
	// No error.
	if err == nil {
		return
	}

	// --force is provided.
	if IsForce() {
		fmt.Printf("Warning: %s\nProceeding by force as requested\n", err)
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

func genParaTimeNames(net *sdkConfig.Network) map[string]string {
	res := make(map[string]string)
	if net == nil {
		return res
	}
	for ptName, pt := range net.ParaTimes.All {
		addr := types.NewAddressFromConsensus(staking.NewRuntimeAddress(pt.Namespace()))
		res[addr.String()] = fmt.Sprintf("paratime:%s", ptName)
	}
	return res
}

// GenAccountEthAddresses returns a map of native (bech32) -> Ethereum (0x...) addresses used for
// ETH-first pretty-printing in SDK transactions.
//
// Sources included:
//   - Address Book entries that also store an ETH address.
//   - Built-in test accounts (secp256k1) with known ETH addresses.
//
// Wallet accounts are intentionally NOT included here to avoid triggering unlock/passphrase
// prompts during read-only operations (e.g. `tx show`, `paratime show`). Deriving the ETH address
// for a wallet entry may require accessing secret material or a hardware device.
//
// How to get ETH-first formatting for wallet accounts without unlock prompts:
//   - Add the account to the Address Book and include its Ethereum address.
//   - Or pass the Ethereum address explicitly when invoking commands.
func GenAccountEthAddresses() map[string]string {
	m := make(map[string]string)

	// Address book entries with eth address.
	for _, entry := range config.Global().AddressBook.All {
		native := entry.GetAddress().String()
		if ea := entry.GetEthAddress(); ea != nil {
			m[native] = ea.Hex()
		}
	}

	// Test accounts that are secp256k1-based.
	for _, tk := range testing.TestAccounts {
		if tk.EthAddress != nil {
			m[tk.Address.String()] = tk.EthAddress.Hex()
		}
	}

	// (Wallet accounts' ETH addresses are not included; see rationale above.)
	return m
}

func GenKnownNames(net *sdkConfig.Network) map[string]string {
	names := GenAccountNames()
	for k, v := range genParaTimeNames(net) {
		// Prefer explicit wallet/addressbook/test names if conflict; only add if absent.
		if _, exists := names[k]; !exists {
			names[k] = v
		}
	}
	return names
}

// FindAccountName finds account's name (if exists).
func FindAccountName(address string) string {
	an := GenAccountNames()
	return an[address]
}

func prettyAddressWith(net *sdkConfig.Network, addrStr string) string {
	names := GenKnownNames(net)
	eth := GenAccountEthAddresses()

	display := addrStr
	if ifEth, ok := eth[addrStr]; ok && ifEth != "" {
		display = ifEth
	}
	if name, ok := names[addrStr]; ok && name != "" {
		return fmt.Sprintf("%s (%s)", name, display)
	}
	return display
}

// PrettyAddressFromConsensus formats a consensus-layer address nicely.
func PrettyAddressFromConsensus(net *sdkConfig.Network, addr staking.Address) string {
	return prettyAddressWith(net, addr.String())
}

// PrettyAddressFromNative formats a runtime/native (bech32) address nicely.
func PrettyAddressFromNative(net *sdkConfig.Network, addr types.Address) string {
	return prettyAddressWith(net, addr.String())
}

// PrettyAddressBothFromConsensus formats a consensus-layer (staking) address as:
//   "<name> (oasis1... | 0x...)" when an ETH form is known,
//   "<name> (oasis1...)"          otherwise,
// falling back to the pair without a name if none is known.
func PrettyAddressBothFromConsensus(net *sdkConfig.Network, addr staking.Address) string {
	return prettyAddressBothWith(net, addr.String())
}

// PrettyAddressBothFromNative formats a runtime/native address in the same "both addresses" form.
func PrettyAddressBothFromNative(net *sdkConfig.Network, addr types.Address) string {
	return prettyAddressBothWith(net, addr.String())
}

// prettyAddressBothWith renders "<name> (bech32 | 0x...)" when ETH is known, otherwise "<name> (bech32)".
// If no name is known, it renders "bech32 | 0x..." or just "bech32".
func prettyAddressBothWith(net *sdkConfig.Network, addrStr string) string {
	names := GenKnownNames(net)
	eth := GenAccountEthAddresses()

	// Compose "bech32 | 0x..." (or just bech32 if no ETH mapping known).
	pair := addrStr
	if ethHex, ok := eth[addrStr]; ok && ethHex != "" {
		pair = fmt.Sprintf("%s | %s", addrStr, ethHex)
	}

	// If a friendly name is available, include it.
	if name, ok := names[addrStr]; ok && name != "" {
		return fmt.Sprintf("%s (%s)", name, pair)
	}
	return pair
}
