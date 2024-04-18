package common

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	configSdk "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/accounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rewards"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/testing"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/config"
	"github.com/oasisprotocol/cli/wallet"
	"github.com/oasisprotocol/cli/wallet/test"
)

const (
	addressExplicitSeparator = ":"
	addressExplicitParaTime  = "paratime"
	addressExplicitConsensus = "consensus"
	addressExplicitPool      = "pool"
	addressExplicitTest      = "test"

	// Shared address literals.
	poolCommon         = "common"
	poolFeeAccumulator = "fee-accumulator"

	// Consensus address literals.
	poolGovernanceDeposits = "governance-deposits"
	poolBurn               = "burn"

	// ParaTime address literals.
	poolRewards           = "rewards"
	poolPendingWithdrawal = "pending-withdrawal"
	poolPendingDelegation = "pending-delegation"
)

// LoadAccount loads the given named account.
func LoadAccount(cfg *config.Config, name string) wallet.Account {
	// Check if the specified account is a test account.
	if testName := ParseTestAccountAddress(name); testName != "" {
		acc, err := LoadTestAccount(testName)
		cobra.CheckErr(err)
		return acc
	}

	acfg, err := LoadAccountConfig(cfg, name)
	cobra.CheckErr(err)

	af, err := acfg.LoadFactory()
	cobra.CheckErr(err)

	var passphrase string
	if af.RequiresPassphrase() && !answerYes {
		// Ask for passphrase to decrypt the account.
		fmt.Printf("Unlock your account.\n")

		err = survey.AskOne(PromptPassphrase, &passphrase)
		cobra.CheckErr(err)
	}

	acc, err := cfg.Wallet.Load(name, passphrase)
	cobra.CheckErr(err)

	return acc
}

// ParseTestAccountAddress extracts test account name from "test:some_test_account" format or
// returns an empty string, if the format doesn't match.
func ParseTestAccountAddress(name string) string {
	if strings.Contains(name, addressExplicitSeparator) {
		subs := strings.SplitN(name, addressExplicitSeparator, 2)
		if subs[0] == addressExplicitTest {
			return subs[1]
		}
	}

	return ""
}

// LoadAccountConfig loads the config instance of the given named account.
func LoadAccountConfig(cfg *config.Config, name string) (*config.Account, error) {
	if testName := ParseTestAccountAddress(name); testName != "" {
		return LoadTestAccountConfig(testName)
	}

	// Early check for whether the account exists so that we don't ask for passphrase first.
	if acfg, exists := cfg.Wallet.All[name]; exists {
		return acfg, nil
	}

	return nil, fmt.Errorf("account '%s' does not exist in the wallet", name)
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

	alg := ""
	switch {
	case testAcc.SignatureAddressSpec().Ed25519 != nil:
		alg = wallet.AlgorithmEd25519Raw
	case testAcc.SignatureAddressSpec().Secp256k1Eth != nil:
		alg = wallet.AlgorithmSecp256k1Raw
	case testAcc.SignatureAddressSpec().Sr25519 != nil:
		alg = wallet.AlgorithmSr25519Raw
	default:
		return nil, fmt.Errorf("unrecognized algorithm for test account %s", name)
	}

	return &config.Account{
		Description: "",
		Kind:        test.Kind,
		Address:     testAcc.Address().String(),
		Config:      map[string]interface{}{"algorithm": alg},
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

	return ResolveAddress(net, address)
}

// ResolveAddress resolves a string address into the corresponding account address.
func ResolveAddress(net *configSdk.Network, address string) (*types.Address, *ethCommon.Address, error) {
	if addr, ethAddr, _ := helpers.ResolveEthOrOasisAddress(address); addr != nil {
		return addr, ethAddr, nil
	}

	if !strings.Contains(address, addressExplicitSeparator) {
		return nil, nil, fmt.Errorf("unsupported address format")
	}

	subs := strings.SplitN(address, addressExplicitSeparator, 3)
	switch kind, data := subs[0], subs[1]; kind {
	case addressExplicitParaTime:
		// paratime:sapphire, paratime:emerald, paratime:cipher
		pt := net.ParaTimes.All[data]
		if pt == nil {
			return nil, nil, fmt.Errorf("paratime '%s' does not exist", data)
		}

		addr := types.NewAddressFromConsensus(staking.NewRuntimeAddress(pt.Namespace()))
		return &addr, nil, nil
	case addressExplicitPool:
		// pool:paratime:pending-withdrawal, pool:paratime:fee-accumulator, pool:consensus:fee-accumulator
		poolKind, poolName := data, ""
		if len(subs) > 2 {
			poolName = subs[2]
		}
		if poolKind == addressExplicitParaTime {
			switch poolName {
			case poolCommon:
				return &accounts.CommonPoolAddress, nil, nil
			case poolFeeAccumulator:
				return &accounts.FeeAccumulatorAddress, nil, nil
			case poolPendingDelegation:
				return &consensusaccounts.PendingDelegationAddress, nil, nil
			case poolPendingWithdrawal:
				return &consensusaccounts.PendingWithdrawalAddress, nil, nil
			case poolRewards:
				return &rewards.RewardPoolAddress, nil, nil
			default:
				return nil, nil, fmt.Errorf("unsupported ParaTime pool: %s", poolName)
			}
		} else if poolKind == addressExplicitConsensus {
			var addr types.Address
			switch poolName {
			case poolBurn:
				addr = types.NewAddressFromConsensus(staking.BurnAddress)
			case poolCommon:
				addr = types.NewAddressFromConsensus(staking.CommonPoolAddress)
			case poolFeeAccumulator:
				addr = types.NewAddressFromConsensus(staking.FeeAccumulatorAddress)
			case poolGovernanceDeposits:
				addr = types.NewAddressFromConsensus(staking.GovernanceDepositsAddress)
			default:
				return nil, nil, fmt.Errorf("unsupported consensus pool: %s", poolName)
			}
			return &addr, nil, nil
		}
		return nil, nil, fmt.Errorf("unsupported pool kind: %s. Please use pool:<poolKind>:<poolName>, for example pool:paratime:pending-withdrawal", poolKind)
	case addressExplicitTest:
		// test:alice, test:dave
		if testKey, ok := testing.TestAccounts[data]; ok {
			return &testKey.Address, testKey.EthAddress, nil
		}
		return nil, nil, fmt.Errorf("unsupported test account: %s", data)
	default:
		// Unsupported kind.
		return nil, nil, fmt.Errorf("unsupported explicit address kind: %s", kind)
	}
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
		return fmt.Errorf("address '%s' is ParaTime rewards pool address", address)
	}

	if address == accounts.CommonPoolAddress.String() {
		return fmt.Errorf("address '%s' is ParaTime common pool address", address)
	}

	if address == accounts.FeeAccumulatorAddress.String() {
		return fmt.Errorf("address '%s' is ParaTime fee accumulator address", address)
	}

	if address == consensusaccounts.PendingDelegationAddress.String() {
		return fmt.Errorf("address '%s' is ParaTime pending delegations address", address)
	}

	if address == consensusaccounts.PendingWithdrawalAddress.String() {
		return fmt.Errorf("address '%s' is ParaTime pending withdrawal address", address)
	}

	if address == staking.BurnAddress.String() {
		return fmt.Errorf("address '%s' is consensus burn address", address)
	}

	if address == staking.CommonPoolAddress.String() {
		return fmt.Errorf("address '%s' is consensus common pool address", address)
	}

	if address == staking.FeeAccumulatorAddress.String() {
		return fmt.Errorf("address '%s' is consensus fee accumulator address", address)
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
