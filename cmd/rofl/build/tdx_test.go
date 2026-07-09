package build //revive:disable

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	buildRofl "github.com/oasisprotocol/cli/build/rofl"
)

const (
	globalComposeFile  = "compose.yaml"
	testnetComposeFile = "compose.testnet.yaml"
)

func TestTdxWantedArtifactsUsesResolvedConfig(t *testing.T) {
	artifacts := tdxWantedArtifacts(buildRofl.ArtifactsConfig{
		Firmware: "firmware",
		Kernel:   "kernel",
		Stage2:   "stage2",
		Container: buildRofl.ContainerArtifactsConfig{
			Runtime: "runtime",
			Compose: "compose",
		},
	})

	got := make(map[string]string)
	for _, artifact := range artifacts {
		got[artifact.kind] = artifact.uri
	}

	require.Equal(t, map[string]string{
		artifactFirmware:         "firmware",
		artifactKernel:           "kernel",
		artifactStage2:           "stage2",
		artifactContainerRuntime: "runtime",
		artifactContainerCompose: "compose",
	}, got)
}

func TestValidateAppUsesDeploymentComposeOverride(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	writeComposeFile(t, filepath.Join(tmpDir, globalComposeFile), "8080", "global.example.com")
	writeComposeFile(t, filepath.Join(tmpDir, testnetComposeFile), "9090", "testnet.example.com")

	manifest := testContainerManifest()
	manifest.Artifacts = &buildRofl.ArtifactsConfig{
		Container: buildRofl.ContainerArtifactsConfig{
			Compose: globalComposeFile,
		},
	}
	manifest.Deployments = map[string]*buildRofl.Deployment{
		"testnet": {
			Network:  "testnet",
			ParaTime: "sapphire",
			Artifacts: &buildRofl.ArtifactsConfig{
				Container: buildRofl.ContainerArtifactsConfig{
					Compose: testnetComposeFile,
				},
			},
		},
	}

	cfg, err := ValidateApp(manifest, "testnet", ValidationOpts{Offline: true})
	require.NoError(t, err)
	require.Len(t, cfg.Ports, 1)
	require.Equal(t, "9090", cfg.Ports[0].Port)
	require.Equal(t, "testnet.example.com", cfg.Ports[0].CustomDomain)
}

func TestValidateAppFallsBackToGlobalCompose(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	writeComposeFile(t, filepath.Join(tmpDir, globalComposeFile), "8080", "global.example.com")

	manifest := testContainerManifest()
	manifest.Artifacts = &buildRofl.ArtifactsConfig{
		Container: buildRofl.ContainerArtifactsConfig{
			Compose: globalComposeFile,
		},
	}
	manifest.Deployments = map[string]*buildRofl.Deployment{
		"testnet": {
			Network:  "testnet",
			ParaTime: "sapphire",
		},
	}

	cfg, err := ValidateApp(manifest, "testnet", ValidationOpts{Offline: true})
	require.NoError(t, err)
	require.Len(t, cfg.Ports, 1)
	require.Equal(t, "8080", cfg.Ports[0].Port)
	require.Equal(t, "global.example.com", cfg.Ports[0].CustomDomain)
}

func testContainerManifest() *buildRofl.Manifest {
	return &buildRofl.Manifest{
		Name:    "test-app",
		Version: "0.1.0",
		TEE:     buildRofl.TEETypeTDX,
		Kind:    buildRofl.AppKindContainer,
		Resources: buildRofl.ResourcesConfig{
			Memory:   512,
			CPUCount: 1,
			Storage: &buildRofl.StorageConfig{
				Kind: buildRofl.StorageKindDiskPersistent,
				Size: 512,
			},
		},
	}
}

func writeComposeFile(t *testing.T, path, publishedPort, customDomain string) {
	t.Helper()

	compose := []byte(`services:
  web:
    image: ghcr.io/oasisprotocol/test:latest
    ports:
      - target: 80
        published: "` + publishedPort + `"
        protocol: tcp
    annotations:
      "net.oasis.proxy.ports.` + publishedPort + `.mode": terminate-tls
      "net.oasis.proxy.ports.` + publishedPort + `.custom_domain": ` + customDomain + `
`)
	require.NoError(t, os.WriteFile(path, compose, 0o600))
}
