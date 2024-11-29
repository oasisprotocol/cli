package governance

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
	"github.com/oasisprotocol/cli/table"
)

var govListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List governance proposals",
	Args:    cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)

		// When not in offline mode, connect to the given network endpoint.
		ctx := context.Background()
		conn, err := connection.Connect(ctx, npa.Network)
		cobra.CheckErr(err)

		table := table.New()
		table.SetHeader([]string{"ID", "Kind", "Submitter", "Created At", "Closes At", "State"})

		proposals, err := conn.Consensus().Governance().Proposals(ctx, common.GetHeight())
		if err != nil {
			cobra.CheckErr(fmt.Errorf("failed to fetch proposals: %w", err))
		}

		var output [][]string
		for _, proposal := range proposals {
			var kind string
			switch {
			case proposal.Content.Upgrade != nil:
				kind = "upgrade"
			case proposal.Content.CancelUpgrade != nil:
				kind = fmt.Sprintf("cancel upgrade %d", proposal.Content.CancelUpgrade.ProposalID)
			case proposal.Content.ChangeParameters != nil:
				kind = fmt.Sprintf("change parameters (%s)", proposal.Content.ChangeParameters.Module)
			default:
				kind = "unknown"
			}

			output = append(output, []string{
				fmt.Sprintf("%d", proposal.ID),
				kind,
				proposal.Submitter.String(),
				fmt.Sprintf("%d", proposal.CreatedAt),
				fmt.Sprintf("%d", proposal.ClosesAt),
				proposal.State.String(),
			})
		}

		table.AppendBulk(output)
		table.Render()
	},
}

func init() {
	govListCmd.Flags().AddFlagSet(common.SelectorNFlags)
	govListCmd.Flags().AddFlagSet(common.HeightFlag)
}
