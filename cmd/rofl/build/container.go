package build

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/oasisprotocol/oasis-core/go/common/version"
	"github.com/oasisprotocol/oasis-core/go/runtime/bundle"

	"github.com/oasisprotocol/cli/cmd/common"
	roflCommon "github.com/oasisprotocol/cli/cmd/rofl/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

const (
	artifactContainerRuntime = "rofl-container runtime"
	artifactContainerCompose = "compose.yaml"

	defaultContainerStage2TemplateURI = "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.3.0/stage2-podman.tar.bz2"

	defaultContainerRuntimeURI = "https://github.com/oasisprotocol/oasis-sdk/releases/download/rofl-containers/v0.1.0/runtime"
)

var (
	tdxContainerRuntimeURI string
	tdxContainerComposeURI string

	tdxContainerCmd = &cobra.Command{
		Use:   "container",
		Short: "Build a container-based TDX ROFL application",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			manifest := roflCommon.LoadManifestAndSetNPA(cfg, npa)

			wantedArtifacts := tdxGetDefaultArtifacts()
			wantedArtifacts = append(wantedArtifacts,
				&artifact{
					kind: artifactContainerRuntime,
					uri:  tdxContainerRuntimeURI,
				},
				&artifact{
					kind: artifactContainerCompose,
					uri:  tdxContainerComposeURI,
				},
			)
			tdxOverrideArtifacts(manifest, wantedArtifacts)
			artifacts := tdxFetchArtifacts(wantedArtifacts)

			fmt.Println("Building a container-based TDX ROFL application...")

			detectBuildMode(npa)

			// Start creating the bundle early so we can fail before building anything.
			bnd := &bundle.Bundle{
				Manifest: &bundle.Manifest{
					Name: manifest.Name,
					ID:   npa.ParaTime.Namespace(),
				},
			}
			var err error
			bnd.Manifest.Version, err = version.FromString(manifest.Version)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("unsupported package version format: %w", err))
			}

			fmt.Printf("App ID:  %s\n", manifest.AppID)
			fmt.Printf("Name:    %s\n", bnd.Manifest.Name)
			fmt.Printf("Version: %s\n", bnd.Manifest.Version)

			// Use the pre-built container runtime.
			initPath := artifacts[artifactContainerRuntime]

			stage2, err := tdxPrepareStage2(artifacts, initPath, map[string]string{
				artifacts[artifactContainerCompose]: "etc/oasis/containers/compose.yaml",
			})
			cobra.CheckErr(err)
			defer os.RemoveAll(stage2.tmpDir)

			// Configure app ID.
			var extraKernelOpts []string
			extraKernelOpts = append(extraKernelOpts,
				fmt.Sprintf("ROFL_APP_ID=%s", manifest.AppID),
			)

			// Obtain and configure trust root.
			trustRoot, err := fetchTrustRoot(npa, manifest.TrustRoot)
			if err != nil {
				_ = os.RemoveAll(stage2.tmpDir)
				cobra.CheckErr(err)
			}
			extraKernelOpts = append(extraKernelOpts,
				fmt.Sprintf("ROFL_CONSENSUS_TRUST_ROOT=%s", trustRoot),
			)

			fmt.Println("Creating ORC bundle...")

			outFn, err := tdxBundleComponent(manifest, artifacts, bnd, stage2, extraKernelOpts)
			if err != nil {
				_ = os.RemoveAll(stage2.tmpDir)
				cobra.CheckErr(err)
			}

			fmt.Println("Computing enclave identity...")

			eids, err := roflCommon.ComputeEnclaveIdentity(bnd, "")
			cobra.CheckErr(err)

			fmt.Println("Update the manifest with the following identities to use the new app:")
			fmt.Println()
			for _, enclaveID := range eids {
				data, _ := enclaveID.MarshalText()
				fmt.Printf("- \"%s\"\n", string(data))
			}
			fmt.Println()

			fmt.Printf("ROFL app built and bundle written to '%s'.\n", outFn)
		},
	}
)

func init() {
	tdxContainerFlags := flag.NewFlagSet("", flag.ContinueOnError)
	tdxContainerFlags.StringVar(&tdxContainerRuntimeURI, "runtime", defaultContainerRuntimeURI, "URL or path to runtime binary")
	tdxContainerFlags.StringVar(&tdxContainerComposeURI, "compose", "compose.yaml", "URL or path to compose.yaml")

	tdxContainerCmd.Flags().AddFlagSet(tdxContainerFlags)
}
