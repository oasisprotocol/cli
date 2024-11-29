package build

import (
	"archive/tar"
	"compress/bzip2"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
)

const artifactCacheDir = "build_cache"

// maybeDownloadArtifact downloads the given artifact and optionally verifies its integrity against
// the provided hash.
func maybeDownloadArtifact(kind, uri, knownHash string) string {
	fmt.Printf("Downloading %s artifact...\n", kind)
	fmt.Printf("  URI: %s\n", uri)
	if knownHash != "" {
		fmt.Printf("  Hash: %s\n", knownHash)
	}

	url, err := url.Parse(uri)
	if err != nil {
		cobra.CheckErr(fmt.Errorf("failed to parse %s artifact URL: %w", kind, err))
	}

	// In case the URI represents a local file, just return it.
	if url.Host == "" {
		return url.Path
	}

	// TODO: Prune cache.
	cacheHash := hash.NewFromBytes([]byte(uri)).Hex()
	cacheFn, err := xdg.CacheFile(filepath.Join("oasis", artifactCacheDir, cacheHash))
	if err != nil {
		cobra.CheckErr(fmt.Errorf("failed to create cache directory for %s artifact: %w", kind, err))
	}

	f, err := os.Create(cacheFn)
	if err != nil {
		cobra.CheckErr(fmt.Errorf("failed to create file for %s artifact: %w", kind, err))
	}
	defer f.Close()

	// Download the remote artifact.
	res, err := http.Get(uri) //nolint:gosec,noctx
	if err != nil {
		cobra.CheckErr(fmt.Errorf("failed to download %s artifact: %w", kind, err))
	}
	defer res.Body.Close()

	// Compute the SHA256 hash while downloading the artifact.
	h := sha256.New()
	rd := io.TeeReader(res.Body, h)

	if _, err = io.Copy(f, rd); err != nil {
		cobra.CheckErr(fmt.Errorf("failed to download %s artifact: %w", kind, err))
	}

	// Verify integrity if available.
	if knownHash != "" {
		artifactHash := fmt.Sprintf("%x", h.Sum(nil))
		if artifactHash != knownHash {
			cobra.CheckErr(fmt.Errorf("hash mismatch for %s artifact (expected: %s got: %s)", kind, knownHash, artifactHash))
		}
	}

	return cacheFn
}

// extractArchive extracts the given tar.bz2 archive into the target output directory.
func extractArchive(fn, outputDir string) error {
	f, err := os.Open(fn)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer f.Close()

	rd := tar.NewReader(bzip2.NewReader(f))

	existingPaths := make(map[string]struct{})
	cleanupPath := func(path string) (string, error) {
		// Sanitize path to ensure it doesn't escape to any parent directories.
		path = filepath.Clean(filepath.Join(outputDir, path))
		if !strings.HasPrefix(path, outputDir) {
			return "", fmt.Errorf("malformed path in archive")
		}
		return path, nil
	}

	modTimes := make(map[string]time.Time)

FILES:
	for {
		var header *tar.Header
		header, err = rd.Next()
		switch {
		case errors.Is(err, io.EOF):
			// We are done.
			break FILES
		case err != nil:
			// Failed to read archive.
			return fmt.Errorf("error reading archive: %w", err)
		case header == nil:
			// Bad archive.
			return fmt.Errorf("malformed archive")
		}

		var path string
		path, err = cleanupPath(header.Name)
		if err != nil {
			return err
		}
		if _, ok := existingPaths[path]; ok {
			continue // Make sure we never handle a path twice.
		}
		existingPaths[path] = struct{}{}
		modTimes[path] = header.ModTime

		switch header.Typeflag {
		case tar.TypeDir:
			// Directory.
			if err = os.MkdirAll(path, header.FileInfo().Mode()); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeLink:
			// Hard link.
			var linkPath string
			linkPath, err = cleanupPath(header.Linkname)
			if err != nil {
				return err
			}

			if err = os.Link(linkPath, path); err != nil {
				return fmt.Errorf("failed to create hard link: %w", err)
			}
		case tar.TypeSymlink:
			// Symbolic link.
			if err = os.Symlink(header.Linkname, path); err != nil {
				return fmt.Errorf("failed to create soft link: %w", err)
			}
		case tar.TypeChar, tar.TypeBlock, tar.TypeFifo:
			// Device or FIFO node.
			if err = extractHandleSpecialNode(path, header); err != nil {
				return err
			}
		case tar.TypeReg:
			// Regular file.
			if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			var fh *os.File
			fh, err = os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}
			if _, err = io.Copy(fh, rd); err != nil { //nolint:gosec
				fh.Close()
				return fmt.Errorf("failed to copy data: %w", err)
			}
			fh.Close()
		default:
			// Skip unsupported types.
			continue
		}
	}

	// Update all modification times at the end to ensure they are correct.
	for path, mtime := range modTimes {
		if err = extractChtimes(path, mtime, mtime); err != nil {
			return fmt.Errorf("failed to change file '%s' timestamps: %w", path, err)
		}
	}

	return nil
}

// copyFile copies the file at path src to a file at path dst using the given mode.
func copyFile(src, dst string, mode os.FileMode) error {
	sf, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open '%s': %w", src, err)
	}
	defer sf.Close()

	df, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("failed to create '%s': %w", dst, err)
	}
	defer df.Close()

	_, err = io.Copy(df, sf)
	return err
}

// computeDirSize computes the size of the given directory.
func computeDirSize(path string) (int64, error) {
	var size int64
	err := filepath.WalkDir(path, func(_ string, d fs.DirEntry, derr error) error {
		if derr != nil {
			return derr
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		size += fi.Size()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return size, nil
}

// ensureBinaryExists checks whether the given binary name exists in path and returns a nice error
// message suggesting to install a given package if it doesn't.
func ensureBinaryExists(name, pkg string) error {
	if path, err := exec.LookPath(name); err != nil || path == "" {
		return fmt.Errorf("missing '%s' binary, please install %s or similar", name, pkg)
	}
	return nil
}

// createExt4Fs creates an ext4 filesystem in the given file using directory dir to populate it.
//
// Returns the size of the created filesystem image in bytes.
func createExt4Fs(fn, dir string) (int64, error) {
	const mkfsExt4Bin = "mkfs.ext4"
	if err := ensureBinaryExists(mkfsExt4Bin, "e2fsprogs"); err != nil {
		return 0, err
	}

	// Compute filesystem size in bytes.
	fsSize, err := computeDirSize(dir)
	if err != nil {
		return 0, err
	}
	fsSize /= 1024                // Convert to kilobytes.
	fsSize = (fsSize * 150) / 100 // Scale by overhead factor of 1.5.

	// Execute mkfs.ext4.
	cmd := exec.Command( //nolint:gosec
		mkfsExt4Bin,
		"-E", "root_owner=0:0",
		"-d", dir,
		fn,
		fmt.Sprintf("%dK", fsSize),
	)
	var out strings.Builder
	cmd.Stderr = &out
	cmd.Stdout = &out
	if err = cmd.Run(); err != nil {
		return 0, fmt.Errorf("%w\n%s", err, out.String())
	}

	// Measure the size of the resulting image.
	fi, err := os.Stat(fn)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// createVerityHashTree creates the verity Merkle hash tree and returns the root hash.
func createVerityHashTree(fsFn, hashFn string) (string, error) {
	// Print a nicer error message in case veritysetup is missing.
	const veritysetupBin = "veritysetup"
	if err := ensureBinaryExists(veritysetupBin, "cryptsetup-bin"); err != nil {
		return "", err
	}

	rootHashFn := hashFn + ".roothash"

	cmd := exec.Command( //nolint:gosec
		veritysetupBin, "format",
		"--data-block-size=4096",
		"--hash-block-size=4096",
		"--root-hash-file="+rootHashFn,
		fsFn,
		hashFn,
	)
	var out strings.Builder
	cmd.Stderr = &out
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w\n%s", err, out.String())
	}

	data, err := os.ReadFile(rootHashFn)
	if err != nil {
		return "", fmt.Errorf("failed to read dm-verity root hash: %w", err)
	}
	return string(data), nil
}

// concatFiles appends the contents of file b to a.
func concatFiles(a, b string) error {
	df, err := os.OpenFile(a, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer df.Close()

	sf, err := os.Open(b)
	if err != nil {
		return err
	}
	defer sf.Close()

	_, err = io.Copy(df, sf)
	return err
}
