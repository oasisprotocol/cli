package build

import (
	"context"
	"fmt"

	"github.com/compose-spec/compose-go/v2/cli"

	"github.com/oasisprotocol/oasis-core/go/runtime/bundle"

	"github.com/oasisprotocol/cli/build/env"
	buildRofl "github.com/oasisprotocol/cli/build/rofl"
	"github.com/oasisprotocol/cli/cmd/common"
)

// tdxBuildContainer builds a TDX-based container ROFL app.
func tdxBuildContainer(
	buildEnv env.ExecEnv,
	tmpDir string,
	npa *common.NPASelection,
	manifest *buildRofl.Manifest,
	deployment *buildRofl.Deployment,
	bnd *bundle.Bundle,
) error {
	fmt.Println("Building a container-based TDX ROFL application...")

	wantedArtifacts := tdxWantedArtifacts(manifest, buildRofl.LatestContainerArtifacts)
	artifacts := tdxFetchArtifacts(wantedArtifacts)

	// Validate compose file.
	fmt.Println("Validating compose file...")
	options, err := cli.NewProjectOptions([]string{artifacts[artifactContainerCompose]})
	if err != nil {
		return fmt.Errorf("failed to set up compose options: %w", err)
	}
	_, err = options.LoadProject(context.Background())
	if err != nil {
		fmt.Println(err)
		return fmt.Errorf("pre-build compose validation failed")
	}

	// Use the pre-built container runtime.
	initPath := artifacts[artifactContainerRuntime]

	stage2, err := tdxPrepareStage2(buildEnv, tmpDir, artifacts, initPath, map[string]string{
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

	return tdxBundleComponent(buildEnv, manifest, artifacts, bnd, stage2, extraKernelOpts)
}
