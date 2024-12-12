package rofl

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/oasisprotocol/oasis-core/go/common/version"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"
)

// ManifestFileNames are the manifest file names that are tried when loading the manifest.
var ManifestFileNames = []string{
	"rofl.yml",
	"rofl.yaml",
}

// Supported ROFL app kinds.
const (
	AppKindRaw       = "raw"
	AppKindContainer = "container"
)

// Supported TEE types.
const (
	TEETypeSGX = "sgx"
	TEETypeTDX = "tdx"
)

// Manifest is the ROFL app manifest that configures various aspects of the app in a single place.
type Manifest struct {
	// AppID is the Bech32-encoded ROFL app ID.
	AppID string `yaml:"app_id" json:"app_id"`
	// Name is the human readable ROFL app name.
	Name string `yaml:"name" json:"name"`
	// Version is the ROFL app version.
	Version string `yaml:"version" json:"version"`
	// Network is the identifier of the network to deploy to by default.
	Network string `yaml:"network,omitempty" json:"network,omitempty"`
	// ParaTime is the identifier of the paratime to deploy to by default.
	ParaTime string `yaml:"paratime,omitempty" json:"paratime,omitempty"`
	// TEE is the type of TEE to build for.
	TEE string `yaml:"tee" json:"tee"`
	// Kind is the kind of ROFL app to build.
	Kind string `yaml:"kind" json:"kind"`
	// TrustRoot is the optional trust root configuration.
	TrustRoot *TrustRootConfig `yaml:"trust_root,omitempty" json:"trust_root,omitempty"`
	// Resources are the requested ROFL app resources.
	Resources ResourcesConfig `yaml:"resources" json:"resources"`
	// Artifacts are the optional artifact location overrides.
	Artifacts *ArtifactsConfig `yaml:"artifacts,omitempty" json:"artifacts,omitempty"`

	// Policy is the ROFL app policy to deploy by default.
	Policy *rofl.AppAuthPolicy `yaml:"policy,omitempty" json:"policy,omitempty"`
	// Admin is the identifier of the admin account.
	Admin string `yaml:"admin,omitempty" json:"admin,omitempty"`
}

// LoadManifest attempts to find and load the ROFL app manifest from a local file.
func LoadManifest() (*Manifest, error) {
	for _, fn := range ManifestFileNames {
		f, err := os.Open(fn)
		switch {
		case err == nil:
		case errors.Is(err, os.ErrNotExist):
			continue
		default:
			return nil, fmt.Errorf("failed to load manifest from '%s': %w", fn, err)
		}

		var m Manifest
		dec := yaml.NewDecoder(f)
		if err = dec.Decode(&m); err != nil {
			f.Close()
			return nil, fmt.Errorf("malformed manifest '%s': %w", fn, err)
		}
		if err = m.Validate(); err != nil {
			f.Close()
			return nil, fmt.Errorf("invalid manifest '%s': %w", fn, err)
		}

		f.Close()
		return &m, nil
	}
	return nil, fmt.Errorf("no ROFL app manifest found (tried: %s)", strings.Join(ManifestFileNames, ", "))
}

// Validate validates the manifest for correctness.
func (m *Manifest) Validate() error {
	if len(m.AppID) == 0 {
		return fmt.Errorf("app ID cannot be empty")
	}
	var appID rofl.AppID
	if err := appID.UnmarshalText([]byte(m.AppID)); err != nil {
		return fmt.Errorf("malformed app ID: %w", err)
	}

	if len(m.Name) == 0 {
		return fmt.Errorf("name cannot be empty")
	}

	if len(m.Version) == 0 {
		return fmt.Errorf("version cannot be empty")
	}
	if _, err := version.FromString(m.Version); err != nil {
		return fmt.Errorf("malformed version: %w", err)
	}

	switch m.TEE {
	case TEETypeSGX, TEETypeTDX:
	default:
		return fmt.Errorf("unsupported TEE type: %s", m.TEE)
	}

	switch m.Kind {
	case AppKindRaw:
	case AppKindContainer:
		if m.TEE != TEETypeTDX {
			return fmt.Errorf("containers are only supported under TDX")
		}
	default:
		return fmt.Errorf("unsupported app kind: %s", m.Kind)
	}

	if err := m.Resources.Validate(); err != nil {
		return fmt.Errorf("bad resources config: %w", err)
	}

	return nil
}

// TrustRootConfig is the trust root configuration.
type TrustRootConfig struct {
	// Height is the consensus layer block height where to take the trust root.
	Height uint64 `yaml:"height,omitempty" json:"height,omitempty"`
	// Hash is the consensus layer block header hash corresponding to the passed height.
	Hash string `yaml:"hash,omitempty" json:"hash,omitempty"`
}

// ResourcesConfig is the resources configuration.
type ResourcesConfig struct {
	// Memory is the amount of memory needed by the app in megabytes.
	Memory uint64 `yaml:"memory" json:"memory"`
	// CPUCount is the number of vCPUs needed by the app.
	CPUCount uint8 `yaml:"cpus" json:"cpus"`
	// EphemeralStorage is the ephemeral storage configuration.
	EphemeralStorage *EphemeralStorageConfig `yaml:"ephemeral_storage,omitempty" json:"ephemeral_storage,omitempty"`
}

// Validate validates the resources configuration for correctness.
func (r *ResourcesConfig) Validate() error {
	if r.Memory < 16 {
		return fmt.Errorf("memory size must be at least 16M")
	}
	if r.CPUCount < 1 {
		return fmt.Errorf("vCPU count must be at least 1")
	}
	if r.EphemeralStorage != nil {
		err := r.EphemeralStorage.Validate()
		if err != nil {
			return fmt.Errorf("bad ephemeral storage config: %w", err)
		}
	}
	return nil
}

// Supported ephemeral storage kinds.
const (
	EphemeralStorageKindNone = "none"
	EphemeralStorageKindDisk = "disk"
	EphemeralStorageKindRAM  = "ram"
)

// EphemeralStorageConfig is the ephemeral storage configuration.
type EphemeralStorageConfig struct {
	// Kind is the storage kind.
	Kind string `yaml:"kind" json:"kind"`
	// Size is the amount of ephemeral storage in megabytes.
	Size uint64 `yaml:"size" json:"size"`
}

// Validate validates the ephemeral storage configuration for correctness.
func (e *EphemeralStorageConfig) Validate() error {
	switch e.Kind {
	case EphemeralStorageKindNone, EphemeralStorageKindDisk, EphemeralStorageKindRAM:
	default:
		return fmt.Errorf("unsupported ephemeral storage kind: %s", e.Kind)
	}

	if e.Size < 16 {
		return fmt.Errorf("ephemeral storage size must be at least 16M")
	}
	return nil
}

// ArtifactsConfig is the artifact location override configuration.
type ArtifactsConfig struct {
	// Firmware is the URI/path to the firmware artifact (empty to use default).
	Firmware string `yaml:"firmware,omitempty" json:"firmware,omitempty"`
	// Kernel is the URI/path to the kernel artifact (empty to use default).
	Kernel string `yaml:"kernel,omitempty" json:"kernel,omitempty"`
	// Stage2 is the URI/path to the stage 2 disk artifact (empty to use default).
	Stage2 string `yaml:"stage2,omitempty" json:"stage2,omitempty"`
	// Container is the container artifacts configuration.
	Container ContainerArtifactsConfig `yaml:"container,omitempty" json:"container,omitempty"`
}

// ContainerArtifactsConfig is the container artifacts configuration.
type ContainerArtifactsConfig struct {
	// Runtime is the URI/path to the container runtime artifact (empty to use default).
	Runtime string `yaml:"runtime,omitempty" json:"runtime,omitempty"`
	// Compose is the URI/path to the docker-compose.yaml artifact (empty to use default).
	Compose string `yaml:"compose,omitempty" json:"compose,omitempty"`
}
