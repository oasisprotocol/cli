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
	require.ErrorContains(err, "name cannot be empty")

	// Empty version.
	m.Name = "my-simple-app"
	err = m.Validate()
	require.ErrorContains(err, "version cannot be empty")

	// Invalid version.
	m.Version = "invalidversion"
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

	// Should be valid without deployments.
	m.Resources.CPUCount = 1
	err = m.Validate()
	require.NoError(err, "must work without deployments")

	// Missing network in deployment.
	m.Deployments = map[string]*Deployment{
		"default": {},
	}
	err = m.Validate()
	require.ErrorContains(err, "bad deployment 'default': network cannot be empty")

	// Invalid app ID.
	m.Deployments["default"].AppID = "foo"
	err = m.Validate()
	require.ErrorContains(err, "bad deployment 'default': malformed app ID")

	// Missing network in deployment.
	m.Deployments["default"].AppID = "rofl1qpa9ydy3qmka3yrqzx0pxuvyfexf9mlh75hker5j"
	err = m.Validate()
	require.ErrorContains(err, "bad deployment 'default': network cannot be empty")

	// Missing paratime in deployment.
	m.Deployments["default"].Network = "foo"
	err = m.Validate()
	require.ErrorContains(err, "bad deployment 'default': paratime cannot be empty")

	// Finally, everything is valid.
	m.Deployments["default"].ParaTime = "bar"
	err = m.Validate()
	require.NoError(err)

	// Add ephemeral storage configuration.
	m.Resources.Storage = &StorageConfig{}
	err = m.Validate()
	require.ErrorContains(err, "bad resources config: bad storage config: unsupported storage kind")

	m.Resources.Storage.Kind = "ram"
	err = m.Validate()
	require.ErrorContains(err, "bad resources config: bad storage config: storage size must be at least 16M")

	m.Resources.Storage.Size = 16
	err = m.Validate()
	require.NoError(err)
}

const serializedYamlManifest = `
name: my-simple-app
version: 0.1.0
tee: tdx
kind: container
resources:
    memory: 16
    cpus: 1
    storage:
        kind: ram
        size: 16
deployments:
    default:
        app_id: rofl1qpa9ydy3qmka3yrqzx0pxuvyfexf9mlh75hker5j
        network: foo
        paratime: bar
        admin: blah
        trust_root:
            height: 24805610
        policy:
            quotes:
                pcs:
                    tcb_validity_period: 30
                    min_tcb_evaluation_data_number: 17
                    tdx: {}
            enclaves:
                - BeCAq9adUtEvhDYmYbZjMBvbIf1CmlVOD57z6aEGlRMAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==
                - P1vM2Zp/qP1Kx07ObFuBDBFCuHGji2gPxrnts7G6g3gAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==
            endorsements:
                - any: {}
            fees: endorsing_node
            max_expiration: 3
scripts:
    pre-build: foo
    post-build: bar
`

func TestManifestSerialization(t *testing.T) {
	require := require.New(t)

	var m Manifest
	err := yaml.Unmarshal([]byte(serializedYamlManifest), &m)
	require.NoError(err, "yaml.Unmarshal")
	err = m.Validate()
	require.NoError(err, "m.Validate")
	require.Equal("my-simple-app", m.Name)
	require.Equal("0.1.0", m.Version)
	require.Equal("tdx", m.TEE)
	require.Equal("container", m.Kind)
	require.EqualValues(16, m.Resources.Memory)
	require.EqualValues(1, m.Resources.CPUCount)
	require.NotNil(m.Resources.Storage)
	require.Equal("ram", m.Resources.Storage.Kind)
	require.EqualValues(16, m.Resources.Storage.Size)
	require.Len(m.Deployments, 1)
	require.Contains(m.Deployments, "default")
	require.Equal("rofl1qpa9ydy3qmka3yrqzx0pxuvyfexf9mlh75hker5j", m.Deployments["default"].AppID)
	require.Equal("foo", m.Deployments["default"].Network)
	require.Equal("bar", m.Deployments["default"].ParaTime)
	require.Equal("blah", m.Deployments["default"].Admin)
	require.EqualValues(24805610, m.Deployments["default"].TrustRoot.Height)
	require.Equal("foo", m.Scripts["pre-build"])
	require.Equal("bar", m.Scripts["post-build"])

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
	require.Len(m.Deployments, 1)
	require.Contains(m.Deployments, "default")
	require.Equal("rofl1qpa9ydy3qmka3yrqzx0pxuvyfexf9mlh75hker5j", m.Deployments["default"].AppID)

	err = os.Remove(manifestFn)
	require.NoError(err)

	manifestFn = "rofl.yaml"
	err = os.WriteFile(manifestFn, []byte(serializedYamlManifest), 0o600)
	require.NoError(err)
	m, err = LoadManifest()
	require.NoError(err)
	require.Len(m.Deployments, 1)
	require.Contains(m.Deployments, "default")
	require.Equal("rofl1qpa9ydy3qmka3yrqzx0pxuvyfexf9mlh75hker5j", m.Deployments["default"].AppID)
}
