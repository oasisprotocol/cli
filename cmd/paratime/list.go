package paratime

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
	Short:   "List configured ParaTimes",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		table := table.New()
		table.SetHeader([]string{"Network", "Paratime", "ID"})

		var output [][]string
		for netName, net := range cfg.Networks.All {
			for ptName, pt := range net.ParaTimes.All {
				displayPtName := ptName
				if net.ParaTimes.Default == ptName {
					displayPtName += common.DefaultMarker
				}

				output = append(output, []string{
					netName,
					displayPtName,
					pt.ID,
				})
			}
		}

		// Sort output by network name and paratime name.
		sort.Slice(output, func(i, j int) bool {
			if output[i][0] != output[j][0] {
				return output[i][0] < output[j][0]
			}
			return output[i][1] < output[j][1]
		})

		table.AppendBulk(output)
		table.Render()
	},
}
