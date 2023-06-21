package governance

import (
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:     "governance",
	Short:   "Governance operations",
	Aliases: []string{"gov"},
}

func init() {
	Cmd.AddCommand(govCreateProposalCmd)
	Cmd.AddCommand(govCastVoteCmd)
	Cmd.AddCommand(govShowCmd)
	Cmd.AddCommand(govListCmd)
}
