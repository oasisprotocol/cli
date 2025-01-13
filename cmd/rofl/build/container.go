package build

import (
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/runtime/bundle"

	buildRofl "github.com/oasisprotocol/cli/build/rofl"
	"github.com/oasisprotocol/cli/cmd/common"
)

const (
	artifactContainerRuntime = "rofl-container runtime"
	artifactContainerCompose = "compose.yaml"

	defaultContainerStage2TemplateURI = "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.3.0/stage2-podman.tar.bz2#d84e0ca961fb0913b73a50ed90eb743af24b3d0acecdbe2594650e2801b41171"

	defaultContainerRuntimeURI = "https://github.com/oasisprotocol/oasis-sdk/releases/download/rofl-containers%2Fv0.1.0/rofl-containers#89d533f8c2c0a8015fdc269ae350ab5b6a271c3aa6b17dd2c1ea49b3ffff2e06"

	defaultContainerComposeURI = "compose.yaml"
)

// tdxBuildContainer builds a TDX-based container ROFL app.
func tdxBuildContainer(tmpDir string, npa *common.NPASelection, manifest *buildRofl.Manifest, bnd *bundle.Bundle) error {
	fmt.Println("Building a container-based TDX ROFL application...")

	tdxStage2TemplateURI = defaultContainerStage2TemplateURI

	wantedArtifacts := tdxGetDefaultArtifacts()
	wantedArtifacts = append(wantedArtifacts,
		&artifact{
			kind: artifactContainerRuntime,
			uri:  defaultContainerRuntimeURI,
		},
		&artifact{
			kind: artifactContainerCompose,
			uri:  defaultContainerComposeURI,
		},
	)
	tdxOverrideArtifacts(manifest, wantedArtifacts)
	artifacts := tdxFetchArtifacts(wantedArtifacts)
	detectBuildMode(npa)

	// Use the pre-built container runtime.
	initPath := artifacts[artifactContainerRuntime]

	stage2, err := tdxPrepareStage2(tmpDir, artifacts, initPath, map[string]string{
		artifacts[artifactContainerCompose]: "etc/oasis/containers/compose.yaml",
	})
	if err != nil {
		return err
	}

	// Configure app ID.
	var extraKernelOpts []string
	extraKernelOpts = append(extraKernelOpts,
		fmt.Sprintf("ROFL_APP_ID=%s", manifest.AppID),
	)

	// Obtain and configure trust root.
	trustRoot, err := fetchTrustRoot(npa, manifest.TrustRoot)
	if err != nil {
		return err
	}
	extraKernelOpts = append(extraKernelOpts,
		fmt.Sprintf("ROFL_CONSENSUS_TRUST_ROOT=%s", trustRoot),
	)

	fmt.Println("Creating ORC bundle...")

	return tdxBundleComponent(manifest, artifacts, bnd, stage2, extraKernelOpts)
}
