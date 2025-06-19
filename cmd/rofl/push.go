package rofl

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/cmd/common"
	roflCommon "github.com/oasisprotocol/cli/cmd/rofl/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push ROFL app to OCI repository",
	Args:  cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)

		manifest, deployment := roflCommon.LoadManifestAndSetNPA(cfg, npa, deploymentName, &roflCommon.ManifestOptions{
			NeedAppID: true,
		})

		orcFilename := roflCommon.GetOrcFilename(manifest, deploymentName)
		ociRepository := ociRepository(deployment)

		if common.OutputFormat() == common.FormatText {
			fmt.Printf("Pushing ROFL app to OCI repository '%s'...\n", ociRepository)
		}
		ociDigest, manifestHash := pushBundleToOciRepository(orcFilename, ociRepository)

		ociReference := fmt.Sprintf("%s@%s", ociRepository, ociDigest)
		switch common.OutputFormat() {
		case common.FormatJSON:
			str, err := common.PrettyJSONMarshal(map[string]string{
				"oci_reference": ociReference,
				"manifest_hash": manifestHash.Hex(),
			})
			cobra.CheckErr(err)
			fmt.Println(string(str))
		default:
			fmt.Println("OCI reference:", ociReference)
			fmt.Println("Manifest hash:", manifestHash)
		}
	},
}

func init() {
	pushCmd.Flags().AddFlagSet(common.FormatFlag)
}
