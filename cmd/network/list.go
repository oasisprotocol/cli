package network

import (
	"sort"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
	"github.com/oasisprotocol/cli/table"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List configured networks",
	Args:    cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		cfg := cliConfig.Global()
		table := table.New()
		table.SetHeader([]string{"Name", "Chain Context", "RPC"})

		var output [][]string
		for name, net := range cfg.Networks.All {
			displayName := name
			if cfg.Networks.Default == name {
				displayName += common.DefaultMarker
			}

			output = append(output, []string{
				displayName,
				net.ChainContext,
				net.RPC,
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
