package network

import (
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	"github.com/oasisprotocol/cli/cmd/common"
	"github.com/oasisprotocol/cli/cmd/network/governance"
)

// Cmd is the network sub-command set root.
var Cmd = &cobra.Command{
	Use:     "network",
	Short:   "Consensus layer operations",
	Aliases: []string{"n", "net"},
}

func networkDetailsFromSurvey(net *config.Network) {
	// 9 is the default used for new networks when nothing better (an existing value cloned from a
	// hardcoded default network, or a --num-decimals flag) is available.
	decimals := net.Denomination.Decimals
	if decimals == 0 {
		decimals = 9
	}
	details := common.DenominationDetails{
		Description: net.Description,
		Symbol:      net.Denomination.Symbol,
		Decimals:    decimals,
	}
	common.AskDenominationDetails(&details)

	net.Description = details.Description
	net.Denomination.Symbol = details.Symbol
	net.Denomination.Decimals = details.Decimals
}

func init() {
	Cmd.AddCommand(addCmd)
	Cmd.AddCommand(addLocalCmd)
	Cmd.AddCommand(governance.Cmd)
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(rmCmd)
	Cmd.AddCommand(setChainContextCmd)
	Cmd.AddCommand(setDefaultCmd)
	Cmd.AddCommand(setRPCCmd)
	Cmd.AddCommand(showCmd)
	Cmd.AddCommand(statusCmd)
	Cmd.AddCommand(trustCmd)
}
