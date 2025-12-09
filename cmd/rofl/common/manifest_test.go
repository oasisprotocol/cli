package common

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/cli/build/rofl"
	cliVersion "github.com/oasisprotocol/cli/version"
)

func TestGetOrcFilename(t *testing.T) {
	require := require.New(t)

	for _, tc := range []struct {
		name       string
		deployment string
		expected   string
	}{
		{"rofl-scheduler", "mainnet", "rofl-scheduler.mainnet.orc"},
		{"ROFL Scheduler", "mainnet", "rofl-scheduler.mainnet.orc"},
		{"   This is a    test   ", "mainnet", "this-is-a-test.mainnet.orc"},
	} {
		manifest := &rofl.Manifest{Name: tc.name}
		fn := GetOrcFilename(manifest, "mainnet")
		require.Equal(tc.expected, fn)
	}
}

func TestCheckToolingVersion(t *testing.T) {
	const testVersion = "1.0.0"

	// Save original stderr and restore after test.
	origStderr := os.Stderr
	defer func() { os.Stderr = origStderr }()

	// Helper to capture stderr output.
	captureStderr := func(fn func()) string {
		r, w, _ := os.Pipe()
		os.Stderr = w
		fn()
		w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		return buf.String()
	}

	t.Run("NoTooling", func(t *testing.T) {
		manifest := &rofl.Manifest{}
		output := captureStderr(func() {
			checkToolingVersion(manifest)
		})
		require.Contains(t, output, "no tooling version")
	})

	t.Run("EmptyVersion", func(t *testing.T) {
		manifest := &rofl.Manifest{
			Tooling: &rofl.ToolingConfig{Version: ""},
		}
		output := captureStderr(func() {
			checkToolingVersion(manifest)
		})
		require.Contains(t, output, "no tooling version")
	})

	t.Run("OlderVersion", func(t *testing.T) {
		// Set a known current version for testing.
		oldVersion := cliVersion.Software
		cliVersion.Software = testVersion
		defer func() { cliVersion.Software = oldVersion }()

		manifest := &rofl.Manifest{
			Tooling: &rofl.ToolingConfig{Version: "0.9.0"},
		}
		output := captureStderr(func() {
			checkToolingVersion(manifest)
		})
		require.Contains(t, output, "older CLI version")
		require.Contains(t, output, "0.9.0")
	})

	t.Run("SameVersion", func(t *testing.T) {
		oldVersion := cliVersion.Software
		cliVersion.Software = testVersion
		defer func() { cliVersion.Software = oldVersion }()

		manifest := &rofl.Manifest{
			Tooling: &rofl.ToolingConfig{Version: testVersion},
		}
		output := captureStderr(func() {
			checkToolingVersion(manifest)
		})
		require.Empty(t, output)
	})

	t.Run("NewerVersion", func(t *testing.T) {
		oldVersion := cliVersion.Software
		cliVersion.Software = testVersion
		defer func() { cliVersion.Software = oldVersion }()

		manifest := &rofl.Manifest{
			Tooling: &rofl.ToolingConfig{Version: "2.0.0"},
		}
		output := captureStderr(func() {
			checkToolingVersion(manifest)
		})
		require.Contains(t, output, "newer CLI version")
		require.Contains(t, output, "upgrading the CLI")
	})

	t.Run("InvalidManifestVersion", func(t *testing.T) {
		manifest := &rofl.Manifest{
			Tooling: &rofl.ToolingConfig{Version: "invalid"},
		}
		output := captureStderr(func() {
			checkToolingVersion(manifest)
		})
		// Should warn about invalid manifest version.
		require.Contains(t, output, "Failed to parse manifest tooling version")
	})

	t.Run("InvalidCurrentVersion", func(t *testing.T) {
		oldVersion := cliVersion.Software
		cliVersion.Software = "invalid-current"
		defer func() { cliVersion.Software = oldVersion }()

		manifest := &rofl.Manifest{
			Tooling: &rofl.ToolingConfig{Version: "1.0.0"},
		}
		// Should panic when CLI version is invalid (software bug).
		require.Panics(t, func() {
			checkToolingVersion(manifest)
		})
	})
}
