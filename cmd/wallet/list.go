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
		table.SetHeader([]string{"Account", "Kind", "Address"})

		var output [][]string
		for name, acc := range cfg.Wallet.All {
			if cfg.Wallet.Default == name {
				name += common.DefaultMarker
			}
			output = append(output, []string{
				name,
				acc.PrettyKind(),
				acc.Address,
			})
		}

		// Sort output by name.
		sort.Slice(output, func(i, j int) bool {
			return output[i][0] < output[j][0]
		})

		table.AppendBulk(output)
		table.Render()
	},
}
