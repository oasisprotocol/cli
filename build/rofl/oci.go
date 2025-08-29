package rofl

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/runtime/bundle"

	"github.com/oasisprotocol/cli/cmd/common/progress"
)

const (
	ociTypeOrcConfig   = "application/vnd.oasis.orc.config.v1+json"
	ociTypeOrcLayer    = "application/vnd.oasis.orc.layer.v1"
	ociTypeOrcArtifact = "application/vnd.oasis.orc"
)

// DefaultOCIRegistry is the default OCI registry.
const DefaultOCIRegistry = "rofl.sh"

// progressBarUpdateInterval specifies how often the progress bar should be updated.
const progressBarUpdateInterval = 1 * time.Second

// TargetWithProgress wraps oras.Target and provides updates via a progress bar.
type TargetWithProgress struct {
	oras.Target

	Message        string
	UpdateInterval time.Duration
	BytesRead      *atomic.Uint64
	BytesTotal     uint64

	stopUpdate chan struct{}
	wg         sync.WaitGroup
}

// NewTargetWithProgress creates a new TargetWithProgress.
// bytesTotal is the size to use for the 100% value (use 0 if unknown).
// msg is the message to display in front of the progress bar.
func NewTargetWithProgress(target oras.Target, bytesTotal uint64, msg string) *TargetWithProgress {
	return &TargetWithProgress{
		Target:         target,
		Message:        msg,
		UpdateInterval: progressBarUpdateInterval,
		BytesRead:      &atomic.Uint64{},
		BytesTotal:     bytesTotal,
		stopUpdate:     make(chan struct{}),
	}
}

// Push wraps the oras.Target Push method with progress bar updates.
func (t *TargetWithProgress) Push(ctx context.Context, desc v1.Descriptor, content io.Reader) error {
	var progReader io.Reader

	// Wrap the reader with our variant.
	pReader := &progressReader{
		reader:    content,
		bytesRead: t.BytesRead,
	}
	progReader = pReader

	// If the reader also has a WriteTo method, wrap it appropriately.
	if wt, ok := content.(io.WriterTo); ok {
		progReader = &progressWriterToReader{
			progressReader: pReader,
			writerTo:       wt,
		}
	}

	// Do the actual push using our wrappers.
	return t.Target.Push(ctx, desc, progReader)
}

// StartProgress starts updating the progress bar.
func (t *TargetWithProgress) StartProgress() {
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

		ticker := time.NewTicker(t.UpdateInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				progress.PrintProgressBar(os.Stderr, t.Message, t.BytesRead.Load(), t.BytesTotal, false)
			case <-t.stopUpdate:
				return
			}
		}
	}()
}

// StopProgress stops the progress bar updates.
func (t *TargetWithProgress) StopProgress() {
	if t.stopUpdate != nil {
		// Print the final stage of the progress bar.
		progress.PrintProgressBar(os.Stderr, t.Message, t.BytesRead.Load(), t.BytesTotal, true)

		close(t.stopUpdate)
		t.stopUpdate = nil
	}
	t.wg.Wait()
}

type progressReader struct {
	reader    io.Reader
	bytesRead *atomic.Uint64
}

func (pr *progressReader) Read(b []byte) (int, error) {
	n, err := pr.reader.Read(b)
	if n > 0 {
		pr.bytesRead.Add(uint64(n))
	}
	return n, err
}

type progressWriterToReader struct {
	*progressReader

	writerTo io.WriterTo
}

func (pwr *progressWriterToReader) WriteTo(w io.Writer) (int64, error) {
	written, err := pwr.writerTo.WriteTo(w)
	if written > 0 {
		pwr.bytesRead.Add(uint64(written))
	}
	return written, err
}

// PushBundleToOciRepository pushes an ORC bundle to the given remote OCI repository.
//
// Returns the OCI manifest digest and the ORC manifest hash.
func PushBundleToOciRepository(bundleFn, dst string) (string, hash.Hash, error) {
	ctx := context.Background()

	atoms := strings.Split(dst, ":")
	if len(atoms) != 2 {
		return "", hash.Hash{}, fmt.Errorf("malformed OCI repository reference (repo:tag required)")
	}
	dst = atoms[0]
	tag := atoms[1]

	// Open the bundle.
	bnd, err := bundle.Open(bundleFn)
	if err != nil {
		return "", hash.Hash{}, fmt.Errorf("failed to open bundle: %w", err)
	}
	defer bnd.Close()

	// Create a temporary file store to build the OCI layers.
	tmpDir, err := os.MkdirTemp("", "oasis-orc2oci")
	if err != nil {
		return "", hash.Hash{}, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	storeDir := filepath.Join(tmpDir, "oci")
	store, err := file.New(storeDir)
	if err != nil {
		return "", hash.Hash{}, fmt.Errorf("failed to create temporary OCI store: %w", err)
	}
	defer store.Close()

	bundleDir := filepath.Join(tmpDir, "bundle")
	if err = bnd.WriteExploded(bundleDir); err != nil {
		return "", hash.Hash{}, fmt.Errorf("failed to explode bundle: %w", err)
	}

	// Keep track of the total size of the bundle as we add files to it.
	var totalSize int64

	getFileSize := func(path string) int64 {
		f, err := os.Stat(path)
		if err != nil {
			// This shouldn't happen, because store.Add should fail if the file doesn't exist.
			panic(err)
		}
		return f.Size()
	}

	// Generate the config object from the manifest.
	const manifestName = "META-INF/MANIFEST.MF"
	manifestPath := filepath.Join(bundleDir, manifestName)
	configDsc, err := store.Add(ctx, manifestName, ociTypeOrcConfig, manifestPath)
	if err != nil {
		return "", hash.Hash{}, fmt.Errorf("failed to add config object from manifest: %w", err)
	}
	totalSize += getFileSize(manifestPath)

	// Add other files as layers.
	layers := make([]v1.Descriptor, 0, len(bnd.Data)-1)
	fns := slices.Sorted(maps.Keys(bnd.Data)) // Ensure deterministic order.
	for _, fn := range fns {
		if fn == manifestName {
			continue
		}

		var layerDsc v1.Descriptor
		filePath := filepath.Join(bundleDir, fn)
		layerDsc, err = store.Add(ctx, fn, ociTypeOrcLayer, filePath)
		if err != nil {
			return "", hash.Hash{}, fmt.Errorf("failed to add OCI layer: %w", err)
		}
		totalSize += getFileSize(filePath)

		layers = append(layers, layerDsc)
	}

	// Pack the OCI manifest.
	opts := oras.PackManifestOptions{
		Layers:           layers,
		ConfigDescriptor: &configDsc,
		ManifestAnnotations: map[string]string{
			// Use a fixed crated timestamp to avoid changing the manifest digest for no reason.
			v1.AnnotationCreated: "2025-03-31T00:00:00Z",
		},
	}
	manifestDescriptor, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, ociTypeOrcArtifact, opts)
	if err != nil {
		return "", hash.Hash{}, fmt.Errorf("failed to pack OCI manifest: %w", err)
	}

	// Tag the manifest.
	if err = store.Tag(ctx, manifestDescriptor, tag); err != nil {
		return "", hash.Hash{}, fmt.Errorf("failed to tag OCI manifest: %w", err)
	}

	// Connect to remote repository.
	repo, err := remote.NewRepository(dst)
	if err != nil {
		return "", hash.Hash{}, fmt.Errorf("failed to init remote OCI repository: %w", err)
	}
	creds, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
	if err != nil {
		return "", hash.Hash{}, fmt.Errorf("failed to init OCI credential store: %w", err)
	}
	client := &auth.Client{
		Client:     retry.DefaultClient,
		Cache:      auth.NewCache(),
		Credential: credentials.Credential(creds),
	}
	if strings.Contains(dst, DefaultOCIRegistry) {
		client.Header = http.Header{
			"X-Rofl-Registry-Caller": []string{"cli"},
		}
	}
	repo.Client = client

	repoSize := uint64(totalSize) //nolint:gosec
	repoWithProgress := NewTargetWithProgress(repo, repoSize, "Pushing...")
	repoWithProgress.StartProgress()
	defer repoWithProgress.StopProgress()

	// Push to remote repository.
	if _, err = oras.Copy(ctx, store, tag, repoWithProgress, tag, oras.DefaultCopyOptions); err != nil {
		return "", hash.Hash{}, fmt.Errorf("failed to push to remote OCI repository: %w", err)
	}

	// Force progress to 100% in case the ORC already exists on the remote.
	// This is necessary so we get a full progressbar instead of an empty one
	// when we're done.
	repoWithProgress.BytesRead.Store(repoWithProgress.BytesTotal)

	return manifestDescriptor.Digest.String(), bnd.Manifest.Hash(), nil
}
