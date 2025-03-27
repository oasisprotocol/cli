package rofl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"

	"github.com/oasisprotocol/oasis-core/go/runtime/bundle"
)

const (
	ociTypeOrcConfig   = "application/vnd.oasis.orc.config.v1+json"
	ociTypeOrcLayer    = "application/vnd.oasis.orc.layer.v1"
	ociTypeOrcArtifact = "application/vnd.oasis.orc"
)

// PushBundleToOciRepository pushes an ORC bundle to the given remote OCI repository.
func PushBundleToOciRepository(bundleFn, dst, tag string) error {
	ctx := context.Background()

	// Open the bundle.
	bnd, err := bundle.Open(bundleFn)
	if err != nil {
		return fmt.Errorf("failed to open bundle: %w", err)
	}
	defer bnd.Close()

	// Create a temporary file store to build the OCI layers.
	tmpDir, err := os.MkdirTemp("", "oasis-orc2oci")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	storeDir := filepath.Join(tmpDir, "oci")
	store, err := file.New(storeDir)
	if err != nil {
		return fmt.Errorf("failed to create temporary OCI store: %w", err)
	}
	defer store.Close()

	bundleDir := filepath.Join(tmpDir, "bundle")
	if err = bnd.WriteExploded(bundleDir); err != nil {
		return fmt.Errorf("failed to explode bundle: %w", err)
	}

	// Generate the config object from the manifest.
	const manifestName = "META-INF/MANIFEST.MF"
	configDsc, err := store.Add(ctx, manifestName, ociTypeOrcConfig, filepath.Join(bundleDir, manifestName))
	if err != nil {
		return fmt.Errorf("failed to add config object from manifest: %w", err)
	}

	// Add other files as layers.
	layers := make([]v1.Descriptor, 0, len(bnd.Data)-1)
	for fn := range bnd.Data {
		if fn == manifestName {
			continue
		}

		layerDsc, err := store.Add(ctx, fn, ociTypeOrcLayer, filepath.Join(bundleDir, fn))
		if err != nil {
			return fmt.Errorf("failed to add OCI layer: %w", err)
		}

		layers = append(layers, layerDsc)
	}

	// Pack the OCI manifest.
	opts := oras.PackManifestOptions{
		Layers:           layers,
		ConfigDescriptor: &configDsc,
	}
	manifestDescriptor, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, ociTypeOrcArtifact, opts)
	if err != nil {
		return fmt.Errorf("failed to pack OCI manifest: %w", err)
	}

	// Tag the manifest.
	if err = store.Tag(ctx, manifestDescriptor, tag); err != nil {
		return fmt.Errorf("failed to tag OCI manifest: %w", err)
	}

	// Connect to remote repository.
	repo, err := remote.NewRepository(dst)
	if err != nil {
		return fmt.Errorf("failed to init remote OCI repository: %w", err)
	}
	creds, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
	if err != nil {
		return fmt.Errorf("failed to init OCI credential store: %w", err)
	}
	repo.Client = &auth.Client{
		Client:     retry.DefaultClient,
		Cache:      auth.NewCache(),
		Credential: credentials.Credential(creds),
	}

	// Push to remote repository.
	if _, err = oras.Copy(ctx, store, tag, repo, tag, oras.DefaultCopyOptions); err != nil {
		return fmt.Errorf("failed to push to remote OCI repository: %w", err)
	}

	return nil
}
