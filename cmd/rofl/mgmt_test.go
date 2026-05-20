package rofl

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	buildRofl "github.com/oasisprotocol/cli/build/rofl"
)

func TestUpgradeUpdatesDeploymentArtifacts(t *testing.T) {
	require := require.New(t)

	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	const manifestYAML = `
name: upgrade-test
version: 0.1.0
tee: tdx
kind: container
resources:
  memory: 512
  cpus: 1
  storage:
    kind: disk-persistent
    size: 512
artifacts:
  builder: old-builder
  firmware: old-firmware
  kernel: old-kernel
  stage2: old-stage2
  container:
    runtime: old-runtime
    compose: compose.yaml
deployments:
  testnet:
    network: testnet
    paratime: sapphire
    artifacts:
      kernel: old-testnet-kernel
      container:
        compose: compose.testnet.yaml
  staging:
    network: testnet
    paratime: sapphire
    artifacts:
      kernel: old-staging-kernel
      container:
        compose: compose.staging.yaml
`
	err := os.WriteFile(filepath.Join(tmpDir, "rofl.yaml"), []byte(manifestYAML), 0o600)
	require.NoError(err)

	output := captureStdout(t, func() {
		upgradeCmd.Run(upgradeCmd, nil)
	})

	updated, err := buildRofl.LoadManifest()
	require.NoError(err)

	latest := buildRofl.LatestContainerArtifacts
	latest.Builder = buildRofl.LatestContainerBuilderImage

	require.Equal(latest.Builder, updated.Artifacts.Builder)
	require.Equal(latest.Firmware, updated.Artifacts.Firmware)
	require.Equal(latest.Kernel, updated.Artifacts.Kernel)
	require.Equal(latest.Stage2, updated.Artifacts.Stage2)
	require.Equal(latest.Container.Runtime, updated.Artifacts.Container.Runtime)
	require.Equal("compose.yaml", updated.Artifacts.Container.Compose)

	for _, deploymentName := range []string{"staging", "testnet"} {
		artifacts := updated.Deployments[deploymentName].Artifacts
		require.NotNil(artifacts)
		require.Equal(latest.Kernel, artifacts.Kernel)
		require.Empty(artifacts.Firmware)
		require.Empty(artifacts.Stage2)
		require.Empty(artifacts.Container.Runtime)
		require.Equal("compose."+deploymentName+".yaml", artifacts.Container.Compose)
	}

	require.Contains(output, "Artifacts have been updated to the latest versions.")
	require.Contains(output, "Updated deployment artifact overrides: staging, testnet.")
	require.Contains(output, "Run `oasis rofl build` to build with the new artifacts.")
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	origStdout := os.Stdout
	defer func() {
		os.Stdout = origStdout
	}()

	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	fn()

	require.NoError(t, w.Close())
	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	return buf.String()
}
