package rofl

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	buildRofl "github.com/oasisprotocol/cli/build/rofl"
	cliCommon "github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

func TestPublicVarRejectsInvalidNames(t *testing.T) {
	for _, name := range []string{
		"",
		"API-URL",
		"API URL",
		"service.API_URL",
		"env.API_URL",
		"å",
	} {
		key, err := publicVarMetadataKey(name)
		require.Error(t, err)
		require.Empty(t, key)
	}
}

func TestParsePublicVarRejectsInvalidValues(t *testing.T) {
	for _, value := range [][]byte{
		{0xff},
	} {
		parsed, err := parsePublicVarValue(value)
		require.Error(t, err)
		require.Empty(t, parsed)
	}
}

func TestPublicVarCommandsUpdateManifest(t *testing.T) {
	require := require.New(t)

	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	const (
		testGreetingVar   = "GREETING"
		testGreetingValue = "hello"
		testAPIURLVar     = "API_URL"
		testAPIURLValue   = "https://example.com?a=b#c"
		testConfigVar     = "CONFIG"
		testConfigValue   = `{"mode":"prod"}`
	)

	prevConfig := *cliConfig.Global()
	cliConfig.ResetDefaults()
	t.Cleanup(func() {
		*cliConfig.Global() = prevConfig
	})

	const manifestYAML = `
name: public-var-test
version: 0.1.0
tee: tdx
kind: container
resources:
  memory: 512
  cpus: 1
  storage:
    kind: disk-persistent
    size: 512
deployments:
  testnet:
    default: true
    network: testnet
    paratime: sapphire
`
	err := os.WriteFile(filepath.Join(tmpDir, "rofl.yaml"), []byte(manifestYAML), 0o600)
	require.NoError(err)

	// Test public-var set and import
	greetingFile := filepath.Join(tmpDir, "greeting.txt")
	err = os.WriteFile(greetingFile, []byte(testGreetingValue), 0o600)
	require.NoError(err)
	publicVarSetCmd.Run(publicVarSetCmd, []string{testGreetingVar, greetingFile})

	envFile := filepath.Join(tmpDir, ".env")
	env := fmt.Sprintf("# public vars\n%s=%s # trailing comment\n%s=%s\n", testAPIURLVar, testAPIURLValue, testConfigVar, testConfigValue)
	err = os.WriteFile(envFile, []byte(env), 0o600)
	require.NoError(err)
	publicVarImportCmd.Run(publicVarImportCmd, []string{envFile})

	manifest, err := buildRofl.LoadManifest()
	require.NoError(err)
	metadata := manifest.Deployments["testnet"].Metadata
	require.Equal(testGreetingValue, metadata[publicVarMetadataPrefix+testGreetingVar])
	require.Equal(testAPIURLValue, metadata[publicVarMetadataPrefix+testAPIURLVar])
	require.Equal(testConfigValue, metadata[publicVarMetadataPrefix+testConfigVar])

	// Test public-var get
	output := captureStdout(t, func() {
		publicVarGetCmd.Run(publicVarGetCmd, []string{testGreetingVar})
	})
	require.Contains(output, fmt.Sprintf("Name:        %s", testGreetingVar))
	require.Contains(output, fmt.Sprintf("Value:       %s", testGreetingValue))

	// Test public-var rm
	publicVarRmCmd.Run(publicVarRmCmd, []string{testGreetingVar})
	manifest, err = buildRofl.LoadManifest()
	require.NoError(err)
	metadata = manifest.Deployments["testnet"].Metadata
	require.NotContains(metadata, publicVarMetadataPrefix+testGreetingVar)
	require.Equal(testAPIURLValue, metadata[publicVarMetadataPrefix+testAPIURLVar])
	require.Equal(testConfigValue, metadata[publicVarMetadataPrefix+testConfigVar])

	// Override public variable to an empty string.
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	err = os.WriteFile(emptyFile, nil, 0o600)
	require.NoError(err)
	withForce(t, true, func() {
		publicVarSetCmd.Run(publicVarSetCmd, []string{testConfigVar, emptyFile})
	})
	manifest, err = buildRofl.LoadManifest()
	require.NoError(err)
	metadata = manifest.Deployments["testnet"].Metadata
	require.Equal(testAPIURLValue, metadata[publicVarMetadataPrefix+testAPIURLVar])
	require.Equal("", metadata[publicVarMetadataPrefix+testConfigVar])
}

func withForce(t *testing.T, enabled bool, fn func()) {
	t.Helper()

	boolFlagValue := func(v bool) string {
		if v {
			return "true"
		}
		return "false"
	}

	prev := cliCommon.IsForce()
	require.NoError(t, cliCommon.ForceFlag.Set("force", boolFlagValue(enabled)))
	defer func() {
		require.NoError(t, cliCommon.ForceFlag.Set("force", boolFlagValue(prev)))
	}()

	fn()
}
