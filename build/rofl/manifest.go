package rofl

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// Well-known scripts.
const (
	ScriptBuildPre   = "build-pre"
	ScriptBuildPost  = "build-post"
	ScriptBundlePost = "bundle-post"
)

// Manifest is the ROFL app manifest that configures various aspects of the app in a single place.
type Manifest struct {
	// Name is the human readable ROFL app name.
	Name string `yaml:"name" json:"name"`
	// Version is the ROFL app version.
	Version string `yaml:"version" json:"version"`
	// TEE is the type of TEE to build for.
	TEE string `yaml:"tee" json:"tee"`
	// Kind is the kind of ROFL app to build.
	Kind string `yaml:"kind" json:"kind"`
	// Resources are the requested ROFL app resources.
	Resources ResourcesConfig `yaml:"resources" json:"resources"`
	// Artifacts are the optional artifact location overrides.
	Artifacts *ArtifactsConfig `yaml:"artifacts,omitempty" json:"artifacts,omitempty"`

	// Deployments are the ROFL app deployments.
	Deployments map[string]*Deployment `yaml:"deployments" json:"deployments"`

	// Scripts are custom scripts that are executed by the build system at specific stages.
	Scripts map[string]string `yaml:"scripts,omitempty" json:"scripts,omitempty"`

	// sourceFn is the filename from which the manifest has been loaded.
	sourceFn string
}

// ManifestExists checks whether a manifest file exist. No attempt is made to load, parse or
// validate any of the found manifest files.
func ManifestExists() bool {
	for _, fn := range ManifestFileNames {
		_, err := os.Stat(fn)
		switch {
		case errors.Is(err, os.ErrNotExist):
			continue
		default:
			return true
		}
	}
	return false
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
		m.sourceFn, _ = filepath.Abs(f.Name()) // Record source filename.

		f.Close()
		return &m, nil
	}
	return nil, fmt.Errorf("no ROFL app manifest found (tried: %s)", strings.Join(ManifestFileNames, ", "))
}

// Validate validates the manifest for correctness.
func (m *Manifest) Validate() error {
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

	for name, d := range m.Deployments {
		if d == nil {
			return fmt.Errorf("bad deployment: %s", name)
		}
		if err := d.Validate(); err != nil {
			return fmt.Errorf("bad deployment '%s': %w", name, err)
		}
	}
	if _, ok := m.Deployments[DefaultDeploymentName]; !ok {
		return fmt.Errorf("must define at least the '%s' deployment", DefaultDeploymentName)
	}

	return nil
}

// SourceFileName returns the filename of the manifest file from which the manifest was loaded or
// an empty string in case the filename is not available.
func (m *Manifest) SourceFileName() string {
	return m.sourceFn
}

// Save serializes the manifest and writes it to the file returned by `SourceFileName`, overwriting
// any previous manifest.
func (m *Manifest) Save() error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(m.SourceFileName(), data, 0o644) //nolint: gosec
}

// DefaultDeploymentName is the name of the default deployment that must always be defined and is
// used in case no deployment is passed.
const DefaultDeploymentName = "default"

// Deployment describes a single ROFL app deployment.
type Deployment struct {
	// AppID is the Bech32-encoded ROFL app ID.
	AppID string `yaml:"app_id,omitempty" json:"app_id,omitempty"`
	// Network is the identifier of the network to deploy to.
	Network string `yaml:"network" json:"network"`
	// ParaTime is the identifier of the paratime to deploy to.
	ParaTime string `yaml:"paratime" json:"paratime"`
	// Admin is the identifier of the admin account.
	Admin string `yaml:"admin,omitempty" json:"admin,omitempty"`
	// Debug is a flag denoting whether this is a debuggable deployment.
	Debug bool `yaml:"debug,omitempty" json:"debug,omitempty"`
	// TrustRoot is the optional trust root configuration.
	TrustRoot *TrustRootConfig `yaml:"trust_root,omitempty" json:"trust_root,omitempty"`
	// Policy is the ROFL app policy.
	Policy *rofl.AppAuthPolicy `yaml:"policy,omitempty" json:"policy,omitempty"`
	// Metadata contains custom metadata.
	Metadata map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	// Secrets contains encrypted secrets.
	Secrets []*SecretConfig `yaml:"secrets,omitempty" json:"secrets,omitempty"`
}

// Validate validates the manifest for correctness.
func (d *Deployment) Validate() error {
	if len(d.AppID) > 0 {
		var appID rofl.AppID
		if err := appID.UnmarshalText([]byte(d.AppID)); err != nil {
			return fmt.Errorf("malformed app ID: %w", err)
		}
	}
	if d.Network == "" {
		return fmt.Errorf("network cannot be empty")
	}
	if d.ParaTime == "" {
		return fmt.Errorf("paratime cannot be empty")
	}
	for _, s := range d.Secrets {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("bad secret: %w", err)
		}
	}
	return nil
}

// HasAppID returns true iff the deployment has an application identifier set.
func (d *Deployment) HasAppID() bool {
	return len(d.AppID) > 0
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
	// Storage is the storage configuration.
	Storage *StorageConfig `yaml:"storage,omitempty" json:"storage,omitempty"`
}

// Validate validates the resources configuration for correctness.
func (r *ResourcesConfig) Validate() error {
	if r.Memory < 16 {
		return fmt.Errorf("memory size must be at least 16M")
	}
	if r.CPUCount < 1 {
		return fmt.Errorf("vCPU count must be at least 1")
	}
	if r.Storage != nil {
		err := r.Storage.Validate()
		if err != nil {
			return fmt.Errorf("bad storage config: %w", err)
		}
	}
	return nil
}

// Supported storage kinds.
const (
	StorageKindNone           = "none"
	StorageKindDiskEphemeral  = "disk-ephemeral"
	StorageKindDiskPersistent = "disk-persistent"
	StorageKindRAM            = "ram"
)

// StorageConfig is the storage configuration.
type StorageConfig struct {
	// Kind is the storage kind.
	Kind string `yaml:"kind" json:"kind"`
	// Size is the amount of storage in megabytes.
	Size uint64 `yaml:"size" json:"size"`
}

// Validate validates the storage configuration for correctness.
func (e *StorageConfig) Validate() error {
	switch e.Kind {
	case StorageKindNone, StorageKindDiskEphemeral, StorageKindDiskPersistent, StorageKindRAM:
	default:
		return fmt.Errorf("unsupported storage kind: %s", e.Kind)
	}

	if e.Size < 16 {
		return fmt.Errorf("storage size must be at least 16M")
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
