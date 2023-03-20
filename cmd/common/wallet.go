package common

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	configSdk "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rewards"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/testing"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/config"
	"github.com/oasisprotocol/cli/wallet"
	"github.com/oasisprotocol/cli/wallet/test"
)

// LoadAccount loads the given named account.
func LoadAccount(cfg *config.Config, name string) wallet.Account {
	// Check if the specified account is a test account.
	if testName := helpers.ParseTestAccountAddress(name); testName != "" {
		acc, err := LoadTestAccount(testName)
		cobra.CheckErr(err)
		return acc
	}

	// Early check for whether the account exists so that we don't ask for passphrase first.
	var (
		acfg   *config.Account
		exists bool
	)
	if acfg, exists = cfg.Wallet.All[name]; !exists {
		cobra.CheckErr(fmt.Errorf("account '%s' does not exist in the wallet", name))
	}

	af, err := acfg.LoadFactory()
	cobra.CheckErr(err)

	var passphrase string
	if af.RequiresPassphrase() && !txYes {
		// Ask for passphrase to decrypt the account.
		fmt.Printf("Unlock your account.\n")

		err = survey.AskOne(PromptPassphrase, &passphrase)
		cobra.CheckErr(err)
	}

	acc, err := cfg.Wallet.Load(name, passphrase)
	cobra.CheckErr(err)

	return acc
}

// LoadTestAccount loads the given named test account.
func LoadTestAccount(name string) (wallet.Account, error) {
	if testKey, ok := testing.TestAccounts[name]; ok {
		return test.NewTestAccount(testKey)
	}
	return nil, fmt.Errorf("test account %s does not exist", name)
}

// LoadTestAccountConfig loads config for the given named test account.
func LoadTestAccountConfig(name string) (*config.Account, error) {
	testAcc, err := LoadTestAccount(name)
	if err != nil {
		return nil, err
	}

	return &config.Account{
		Description: "",
		Kind:        test.Kind,
		Address:     testAcc.Address().String(),
		Config:      nil,
	}, nil
}

// ResolveLocalAccountOrAddress resolves a string address into the corresponding account address.
func ResolveLocalAccountOrAddress(net *configSdk.Network, address string) (*types.Address, *ethCommon.Address, error) {
	// Check if address is the account name in the wallet.
	if acc, ok := config.Global().Wallet.All[address]; ok {
		addr := acc.GetAddress()
		// TODO: Implement acc.GetEthAddress()
		return &addr, nil, nil
	}

	// Check if address is the name of an address book entry.
	if entry, ok := config.Global().AddressBook.All[address]; ok {
		addr := entry.GetAddress()
		return &addr, entry.GetEthAddress(), nil
	}

	return helpers.ResolveAddress(net, address)
}

// CheckAddressIsConsensusCapable checks whether the given address is derived from any known
// Ethereum address and is thus unspendable on consensus layer.
func CheckAddressIsConsensusCapable(cfg *config.Config, address string) error {
	if ethCommon.IsHexAddress(address) {
		return fmt.Errorf("signer for address '%s' will not be able to sign transactions on consensus layer", address)
	}

	for name, acc := range cfg.Wallet.All {
		if acc.Address == address && !acc.HasConsensusSigner() {
			return fmt.Errorf("destination account '%s' (%s) will not be able to sign transactions on consensus layer", name, acc.Address)
		}
	}

	for name := range testing.TestAccounts {
		testAcc, _ := LoadTestAccount(name)
		if testAcc.Address().String() == address && testAcc.ConsensusSigner() == nil {
			return fmt.Errorf("test account '%s' (%s) will not be able to sign transactions on consensus layer", name, testAcc.Address().String())
		}
	}

	for name, acc := range cfg.AddressBook.All {
		if acc.Address == address && acc.GetEthAddress() != nil {
			return fmt.Errorf("signer for address named '%s' (%s) will not be able to sign transactions on consensus layer", name, acc.GetEthAddress().Hex())
		}
	}

	return nil
}

// CheckAddressNotReserved checks whether the given native address is potentially
// unspendable like the reserved addresses for the staking reward and common pool,
// fee accumulator or the native ParaTime addresses.
func CheckAddressNotReserved(cfg *config.Config, address string) error {
	if address == rewards.RewardPoolAddress.String() {
		return fmt.Errorf("address '%s' is rewards pool address", address)
	}

	if address == staking.CommonPoolAddress.String() {
		return fmt.Errorf("address '%s' is common pool address", address)
	}

	if address == staking.FeeAccumulatorAddress.String() {
		return fmt.Errorf("address '%s' is fee accumulator address", address)
	}

	if address == staking.GovernanceDepositsAddress.String() {
		return fmt.Errorf("address '%s' is governance deposit address", address)
	}

	for netName, net := range cfg.Networks.All {
		for ptName, pt := range net.ParaTimes.All {
			if types.NewAddressFromConsensus(staking.NewRuntimeAddress(pt.Namespace())).String() == address {
				return fmt.Errorf("did you mean --paratime %s? Address '%s' is native address of paratime:%s on %s and should not be transferred to directly", ptName, address, ptName, netName)
			}
		}
	}

	return nil
}
