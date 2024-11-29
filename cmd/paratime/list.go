package paratime

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
	"github.com/oasisprotocol/cli/table"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List configured ParaTimes",
	Args:    cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		cfg := cliConfig.Global()
		table := table.New()
		table.SetHeader([]string{"Network", "Paratime", "ID", "Denomination(s)"})

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
					formatDenominations(pt.Denominations),
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

func formatDenominations(denominations map[string]*config.DenominationInfo) string {
	var fmtDenom string
	fmtDenomArray := make([]string, 0, len(denominations))
	for nativeKey, denom := range denominations {
		if nativeKey == config.NativeDenominationKey {
			fmtDenom = fmt.Sprintf("%s[%d] (*)", denom.Symbol, denom.Decimals)
		} else {
			fmtDenom = fmt.Sprintf("%s[%d]", denom.Symbol, denom.Decimals)
		}

		fmtDenomArray = append(fmtDenomArray, fmtDenom)
	}
	slices.Sort(fmtDenomArray)
	return strings.Join(fmtDenomArray, "\n")
}
