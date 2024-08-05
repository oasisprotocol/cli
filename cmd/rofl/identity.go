package rofl

import (
	"fmt"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/oasisprotocol/oasis-core/go/common/sgx"
	"github.com/oasisprotocol/oasis-core/go/runtime/bundle"
	"github.com/oasisprotocol/oasis-core/go/runtime/bundle/component"
)

var (
	compID string

	identityCmd = &cobra.Command{
		Use:     "identity app.orc [--component ID]",
		Short:   "Show the cryptographic identity of the ROFL app(s) in the specified bundle",
		Aliases: []string{"id"},
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			bundleFn := args[0]

			bnd, err := bundle.Open(bundleFn)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to open bundle: %w", err))
			}

			var cid component.ID
			if compID != "" {
				if err = cid.UnmarshalText([]byte(compID)); err != nil {
					cobra.CheckErr(fmt.Errorf("malformed component ID: %w", err))
				}
			}

			var enclaveID *sgx.EnclaveIdentity
			for _, comp := range bnd.Manifest.GetAvailableComponents() {
				if comp.Kind != component.ROFL {
					continue // Skip non-ROFL components.
				}
				switch compID {
				case "":
					// When not specified we use the first ROFL app.
				default:
					if !comp.Matches(cid) {
						continue
					}
				}

				enclaveID, err = bnd.EnclaveIdentity(comp.ID())
				if err != nil {
					cobra.CheckErr(fmt.Errorf("failed to generate enclave identity of '%s': %w", comp.ID(), err))
				}

				data, _ := enclaveID.MarshalText()
				fmt.Println(string(data))
				return
			}

			switch compID {
			case "":
				cobra.CheckErr("no ROFL apps found in bundle")
			default:
				cobra.CheckErr(fmt.Errorf("ROFL app '%s' not found in bundle", compID))
			}
		},
	}
)

func init() {
	idFlags := flag.NewFlagSet("", flag.ContinueOnError)
	idFlags.StringVar(&compID, "component", "", "optional component ID")

	identityCmd.Flags().AddFlagSet(idFlags)
}
