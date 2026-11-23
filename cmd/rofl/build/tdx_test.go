//nolint:goconst // Constants would reduce readability in this file.
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
		Platform: "platform",
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
		artifactPlatform:         "platform",
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

func TestValidateAppProxyProtocol(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

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

	for _, tc := range []struct {
		annotation string
		expected   string
		valid      bool
	}{
		{`"net.oasis.proxy.ports.8080.proxy_protocol": v2`, "v2", true},
		{`"net.oasis.proxy.ports.8080.mode": passthrough`, "", true},
		{`"net.oasis.proxy.ports.8080.proxy_protocol": v1`, "", false},
	} {
		compose := []byte(`services:
  web:
    image: ghcr.io/oasisprotocol/test:latest
    ports:
      - target: 80
        published: "8080"
        protocol: tcp
    annotations:
      ` + tc.annotation + `
`)
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, globalComposeFile), compose, 0o600))

		cfg, err := ValidateApp(manifest, "testnet", ValidationOpts{Offline: true})
		if !tc.valid {
			require.ErrorContains(t, err, "invalid PROXY protocol version")
			continue
		}
		require.NoError(t, err)
		require.Len(t, cfg.Ports, 1)
		require.Equal(t, tc.expected, cfg.Ports[0].ProxyProtocol)
	}
}

func TestValidateAppProxyTimeouts(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	manifest := testContainerManifest()
	manifest.Artifacts = &buildRofl.ArtifactsConfig{
		Container: buildRofl.ContainerArtifactsConfig{
			Compose: globalComposeFile,
		},
	}

	for _, tc := range []struct {
		annotations string
		expected    PortMapping
		err         string
	}{
		{
			annotations: `"net.oasis.proxy.ports.8080.timeout_rw": "300"
      "net.oasis.proxy.ports.8080.timeout_connection": 3600
      "net.oasis.proxy.ports.8080.timeout_connect": 5`,
			expected: PortMapping{TimeoutConnect: 5, TimeoutConnection: 3600, TimeoutRW: 300},
		},
		{
			annotations: `"net.oasis.proxy.ports.8080.mode": passthrough`,
		},
		{
			annotations: `"net.oasis.proxy.ports.8080.timeout_rw": 0`,
			err:         "has an invalid timeout_rw",
		},
		{
			annotations: `"net.oasis.proxy.ports.8080.timeout_connection": 86401`,
			err:         "has an invalid timeout_connection",
		},
		{
			annotations: `"net.oasis.proxy.ports.8080.timeout_connect": 5s`,
			err:         "has an invalid timeout_connect",
		},
	} {
		compose := []byte(`services:
  web:
    image: ghcr.io/oasisprotocol/test:latest
    ports:
      - target: 80
        published: "8080"
        protocol: tcp
    annotations:
      ` + tc.annotations + `
`)
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, globalComposeFile), compose, 0o600))

		cfg, err := ValidateApp(manifest, "", ValidationOpts{Offline: true})
		if tc.err != "" {
			require.ErrorContains(t, err, tc.err)
			continue
		}
		require.NoError(t, err)
		require.Len(t, cfg.Ports, 1)
		require.Equal(t, tc.expected.TimeoutConnect, cfg.Ports[0].TimeoutConnect)
		require.Equal(t, tc.expected.TimeoutConnection, cfg.Ports[0].TimeoutConnection)
		require.Equal(t, tc.expected.TimeoutRW, cfg.Ports[0].TimeoutRW)
	}
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
