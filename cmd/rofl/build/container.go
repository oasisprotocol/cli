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

	defaultContainerStage2TemplateURI = "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.3.3/stage2-podman.tar.bz2#827531546f3db6b0945ece7ddab4e10d648eaa3ba1c146b7889d7cb9cbf0b507"

	defaultContainerRuntimeURI = "https://github.com/oasisprotocol/oasis-sdk/releases/download/rofl-containers%2Fv0.3.3/rofl-containers#b7f025e3bb844a4ce044fa3a2503f6854e5e2d2d5ec22be919c582e57cf5d6ab"

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
