package common

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/testing"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/config"
)

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
