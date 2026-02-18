package wallet

import (
	"sort"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/cmd/common"
	"github.com/oasisprotocol/cli/config"
	"github.com/oasisprotocol/cli/table"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List configured accounts",
	Args:    cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		cfg := config.Global()
		table := table.New()
		table.Header("Account", "Kind", "Address")

		var output [][]string //nolint: prealloc
		for name, acc := range cfg.Wallet.All {
			if cfg.Wallet.Default == name {
				name += common.DefaultMarker
			}

			addrStr := acc.Address
			if ethAddr := acc.GetEthAddress(); ethAddr != nil {
				addrStr = ethAddr.Hex()
			}
			output = append(output, []string{
				name,
				acc.PrettyKind(),
				addrStr,
			})
		}

		// Sort output by name.
		sort.Slice(output, func(i, j int) bool {
			return output[i][0] < output[j][0]
		})

		cobra.CheckErr(table.Bulk(output))
		cobra.CheckErr(table.Render())
	},
}
