package rofl

import (
	"fmt"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/oasisprotocol/oasis-core/go/runtime/bundle"

	roflCommon "github.com/oasisprotocol/cli/cmd/rofl/common"
)

var (
	compID string

	identityCmd = &cobra.Command{
		Use:     "identity app.orc [--component ID]",
		Short:   "Show the cryptographic identity of the ROFL app(s) in the specified bundle",
		Aliases: []string{"id"},
		Args:    cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			bundleFn := args[0]

			bnd, err := bundle.Open(bundleFn)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to open bundle: %w", err))
			}

			ids, err := roflCommon.ComputeEnclaveIdentity(bnd, compID)
			cobra.CheckErr(err)

			for _, id := range ids {
				data, _ := id.Enclave.MarshalText()
				fmt.Println(string(data))
			}
		},
	}
)

func init() {
	idFlags := flag.NewFlagSet("", flag.ContinueOnError)
	idFlags.StringVar(&compID, "component", "", "optional component ID")

	identityCmd.Flags().AddFlagSet(idFlags)
}
