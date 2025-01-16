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

	defaultContainerStage2TemplateURI = "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.3.1/stage2-podman.tar.bz2#4239991c742c189f9f6949e597276e9ae5972768c4bfb5108190565e2addf47e"

	defaultContainerRuntimeURI = "https://github.com/oasisprotocol/oasis-sdk/releases/download/rofl-containers%2Fv0.2.0/rofl-containers#d6f60ce41a33e8240396596037094816cdcd40abd1813d6dc26988d557bceccd"

	defaultContainerComposeURI = "compose.yaml"
)

// tdxBuildContainer builds a TDX-based container ROFL app.
func tdxBuildContainer(
	tmpDir string,
	npa *common.NPASelection,
	manifest *buildRofl.Manifest,
	deployment *buildRofl.Deployment,
	bnd *bundle.Bundle,
) error {
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
		fmt.Sprintf("ROFL_APP_ID=%s", deployment.AppID),
	)

	// Obtain and configure trust root.
	trustRoot, err := fetchTrustRoot(npa, deployment.TrustRoot)
	if err != nil {
		return err
	}
	extraKernelOpts = append(extraKernelOpts,
		fmt.Sprintf("ROFL_CONSENSUS_TRUST_ROOT=%s", trustRoot),
	)

	fmt.Println("Creating ORC bundle...")

	return tdxBundleComponent(manifest, artifacts, bnd, stage2, extraKernelOpts)
}
