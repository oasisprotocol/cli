package common

import (
	"fmt"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	cliConfig "github.com/oasisprotocol/cli/config"
)

const DefaultMarker = " (*)"

var (
	selectedNetwork  string
	selectedParaTime string
	selectedAccount  string

	noParaTime bool
)

var (
	// AccountFlag corresponds to the --account selector flag.
	AccountFlag *flag.FlagSet
	// SelectorFlags contains the common selector flags for network/ParaTime/account.
	SelectorFlags *flag.FlagSet
	// SelectorNPFlags contains the common selector flags for network/ParaTime.
	SelectorNPFlags *flag.FlagSet
	// SelectorNFlags contains the common selector flags for network.
	SelectorNFlags *flag.FlagSet
	// SelectorNAFlags contains the common selector flags for network/account.
	SelectorNAFlags *flag.FlagSet
)

// NPASelection contains the network/ParaTime/account selection.
type NPASelection struct {
	NetworkName string
	Network     *config.Network

	ParaTimeName string
	ParaTime     *config.ParaTime

	AccountName string
	Account     *cliConfig.Account
}

// GetNPASelection returns the user-selected network/ParaTime/account combination.
func GetNPASelection(cfg *cliConfig.Config) *NPASelection {
	var s NPASelection
	s.NetworkName = cfg.Networks.Default
	if selectedNetwork != "" {
		s.NetworkName = selectedNetwork
	}
	if s.NetworkName == "" {
		cobra.CheckErr(fmt.Errorf("no networks configured"))
	}
	s.Network = cfg.Networks.All[s.NetworkName]
	if s.Network == nil {
		cobra.CheckErr(fmt.Errorf("network '%s' does not exist", s.NetworkName))
	}

	if !noParaTime {
		s.ParaTimeName = s.Network.ParaTimes.Default
		if selectedParaTime != "" {
			s.ParaTimeName = selectedParaTime
		}
		if s.ParaTimeName != "" {
			s.ParaTime = s.Network.ParaTimes.All[s.ParaTimeName]
			if s.ParaTime == nil {
				cobra.CheckErr(fmt.Errorf("ParaTime '%s' does not exist", s.ParaTimeName))
			}
		}
	}

	s.AccountName = cfg.Wallet.Default
	if selectedAccount != "" {
		s.AccountName = selectedAccount
	}
	if s.AccountName != "" {
		accCfg, err := LoadAccountConfig(cfg, s.AccountName)
		cobra.CheckErr(err)
		s.Account = accCfg
	}

	return &s
}

// MustHaveAccount checks whether Account is populated and fails if it is not.
func (npa *NPASelection) MustHaveAccount() {
	if npa.Account == nil {
		cobra.CheckErr("no accounts configured in your wallet. Run `oasis wallet` to create or import an account. Then use --account <name> to specify the account")
	}
}

// MustHaveParaTime checks whether ParaTime is populated and fails if it is not.
func (npa *NPASelection) MustHaveParaTime() {
	if npa.ParaTime == nil {
		cobra.CheckErr("no ParaTimes selected. Run `oasis paratime` to configure a ParaTime first. Then use --paratime <name> to specify the ParaTime")
	}
}

// PrettyPrintNetwork formats the network name and description, if one exists.
func (npa *NPASelection) PrettyPrintNetwork() (out string) {
	out = npa.NetworkName
	if len(npa.Network.Description) > 0 {
		out += fmt.Sprintf(" (%s)", npa.Network.Description)
	}
	return
}

// ConsensusDenomination returns the denomination used to represent the consensus layer token.
func (npa *NPASelection) ConsensusDenomination() (denom types.Denomination) {
	if npa.ParaTime == nil {
		return types.NativeDenomination
	}

	switch cfgDenom := npa.ParaTime.ConsensusDenomination; cfgDenom {
	case config.NativeDenominationKey:
		denom = types.NativeDenomination
	default:
		denom = types.Denomination(cfgDenom)
	}
	return
}

func init() {
	AccountFlag = flag.NewFlagSet("", flag.ContinueOnError)
	AccountFlag.StringVar(&selectedAccount, "account", "", "explicitly set account to use")

	SelectorFlags = flag.NewFlagSet("", flag.ContinueOnError)
	SelectorFlags.StringVar(&selectedNetwork, "network", "", "explicitly set network to use")
	SelectorFlags.StringVar(&selectedParaTime, "paratime", "", "explicitly set ParaTime to use")
	SelectorFlags.BoolVar(&noParaTime, "no-paratime", false, "explicitly set that no ParaTime should be used")
	SelectorFlags.AddFlagSet(AccountFlag)

	SelectorNPFlags = flag.NewFlagSet("", flag.ContinueOnError)
	SelectorNPFlags.StringVar(&selectedNetwork, "network", "", "explicitly set network to use")
	SelectorNPFlags.StringVar(&selectedParaTime, "paratime", "", "explicitly set ParaTime to use")
	SelectorNPFlags.BoolVar(&noParaTime, "no-paratime", false, "explicitly set that no ParaTime should be used")

	SelectorNAFlags = flag.NewFlagSet("", flag.ContinueOnError)
	SelectorNAFlags.StringVar(&selectedNetwork, "network", "", "explicitly set network to use")
	SelectorNAFlags.AddFlagSet(AccountFlag)

	SelectorNFlags = flag.NewFlagSet("", flag.ContinueOnError)
	SelectorNFlags.StringVar(&selectedNetwork, "network", "", "explicitly set network to use")

	// Backward compatibility.
	SelectorFlags.StringVar(&selectedAccount, "wallet", "", "explicitly set account to use. OBSOLETE, USE --account INSTEAD!")
	err := SelectorFlags.MarkHidden("wallet")
	cobra.CheckErr(err)
}
