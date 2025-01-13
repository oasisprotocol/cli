package rofl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestManifestValidation(t *testing.T) {
	require := require.New(t)

	// Empty manifest is not valid.
	m := Manifest{}
	err := m.Validate()
	require.ErrorContains(err, "app ID cannot be empty")

	// Invalid app ID.
	m.AppID = "foo"
	err = m.Validate()
	require.ErrorContains(err, "malformed app ID")

	// Empty name.
	m.AppID = "rofl1qpa9ydy3qmka3yrqzx0pxuvyfexf9mlh75hker5j"
	err = m.Validate()
	require.ErrorContains(err, "name cannot be empty")

	// Empty version.
	m.Name = "my-simple-app"
	err = m.Validate()
	require.ErrorContains(err, "version cannot be empty")

	// Invalid version.
	m.Version = "foo"
	err = m.Validate()
	require.ErrorContains(err, "malformed version")

	// Unsupported TEE type.
	m.Version = "0.1.0"
	err = m.Validate()
	require.ErrorContains(err, "unsupported TEE type")

	// Unsupported app kind.
	m.TEE = "sgx"
	err = m.Validate()
	require.ErrorContains(err, "unsupported app kind")

	// Containers are only supported under TDX.
	m.Kind = "container"
	err = m.Validate()
	require.ErrorContains(err, "containers are only supported under TDX")

	// Bad resources configuration.
	m.TEE = "tdx"
	err = m.Validate()
	require.ErrorContains(err, "bad resources config: memory size must be at least 16M")

	m.Resources.Memory = 16
	err = m.Validate()
	require.ErrorContains(err, "bad resources config: vCPU count must be at least 1")

	// Finally, everything is valid.
	m.Resources.CPUCount = 1
	err = m.Validate()
	require.NoError(err)

	// Add ephemeral storage configuration.
	m.Resources.EphemeralStorage = &EphemeralStorageConfig{}
	err = m.Validate()
	require.ErrorContains(err, "bad resources config: bad ephemeral storage config: unsupported ephemeral storage kind")

	m.Resources.EphemeralStorage.Kind = "ram"
	err = m.Validate()
	require.ErrorContains(err, "bad resources config: bad ephemeral storage config: ephemeral storage size must be at least 16M")

	m.Resources.EphemeralStorage.Size = 16
	err = m.Validate()
	require.NoError(err)
}

const serializedYamlManifest = `
app_id: rofl1qpa9ydy3qmka3yrqzx0pxuvyfexf9mlh75hker5j
name: my-simple-app
version: 0.1.0
tee: tdx
kind: container
resources:
    memory: 16
    cpus: 1
    ephemeral_storage:
        kind: ram
        size: 16
`

func TestManifestSerialization(t *testing.T) {
	require := require.New(t)

	var m Manifest
	err := yaml.Unmarshal([]byte(serializedYamlManifest), &m)
	require.NoError(err, "yaml.Unmarshal")
	err = m.Validate()
	require.NoError(err, "m.Validate")
	require.Equal("rofl1qpa9ydy3qmka3yrqzx0pxuvyfexf9mlh75hker5j", m.AppID)
	require.Equal("my-simple-app", m.Name)
	require.Equal("0.1.0", m.Version)
	require.Equal("tdx", m.TEE)
	require.Equal("container", m.Kind)
	require.EqualValues(16, m.Resources.Memory)
	require.EqualValues(1, m.Resources.CPUCount)
	require.NotNil(m.Resources.EphemeralStorage)
	require.Equal("ram", m.Resources.EphemeralStorage.Kind)
	require.EqualValues(16, m.Resources.EphemeralStorage.Size)

	enc, err := yaml.Marshal(m)
	require.NoError(err, "yaml.Marshal")

	var dec Manifest
	err = yaml.Unmarshal(enc, &dec)
	require.NoError(err, "yaml.Unmarshal(round-trip)")
	require.EqualValues(m, dec, "serialization should round-trip")
	err = dec.Validate()
	require.NoError(err, "dec.Validate")
}

func TestLoadManifest(t *testing.T) {
	require := require.New(t)

	tmpDir, err := os.MkdirTemp("", "oasis-test-load-manifest")
	require.NoError(err)
	defer os.RemoveAll(tmpDir)

	err = os.Chdir(tmpDir)
	require.NoError(err)

	_, err = LoadManifest()
	require.ErrorContains(err, "no ROFL app manifest found")

	manifestFn := filepath.Join(tmpDir, "rofl.yml")
	err = os.WriteFile(manifestFn, []byte("foo"), 0o600)
	require.NoError(err)
	_, err = LoadManifest()
	require.ErrorContains(err, "malformed manifest 'rofl.yml'")

	err = os.WriteFile(manifestFn, []byte(serializedYamlManifest), 0o600)
	require.NoError(err)
	m, err := LoadManifest()
	require.NoError(err)
	require.Equal("rofl1qpa9ydy3qmka3yrqzx0pxuvyfexf9mlh75hker5j", m.AppID)

	err = os.Remove(manifestFn)
	require.NoError(err)

	manifestFn = "rofl.yaml"
	err = os.WriteFile(manifestFn, []byte(serializedYamlManifest), 0o600)
	require.NoError(err)
	m, err = LoadManifest()
	require.NoError(err)
	require.Equal("rofl1qpa9ydy3qmka3yrqzx0pxuvyfexf9mlh75hker5j", m.AppID)
}
