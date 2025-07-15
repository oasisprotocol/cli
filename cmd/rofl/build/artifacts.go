package build

import (
	"archive/tar"
	"compress/bzip2"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
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

	"github.com/oasisprotocol/cli/build/env"
)

const artifactCacheDir = "build_cache"

// maybeDownloadArtifact downloads the given artifact and optionally verifies its integrity against
// the hash provided in the URI fragment.
func maybeDownloadArtifact(kind, uri string) string {
	fmt.Printf("Downloading %s artifact...\n", kind)
	fmt.Printf("  URI: %s\n", uri)

	url, err := url.Parse(uri)
	if err != nil {
		cobra.CheckErr(fmt.Errorf("failed to parse %s artifact URL: %w", kind, err))
	}

	// In case the URI represents a local file, check that it exists and return it.
	if url.Host == "" {
		_, err = os.Stat(url.Path)
		cobra.CheckErr(err)
		return url.Path
	}

	// If the URI contains a fragment and the known hash is empty, treat it as a known hash.
	var knownHash string
	if url.Fragment != "" {
		knownHash = url.Fragment
	}
	if knownHash != "" {
		fmt.Printf("  Hash: %s\n", knownHash)
	}

	// TODO: Prune cache.
	cacheHash := hash.NewFromBytes([]byte(uri)).Hex()
	cacheFn, err := xdg.CacheFile(filepath.Join("oasis", artifactCacheDir, cacheHash))
	if err != nil {
		cobra.CheckErr(fmt.Errorf("failed to create cache directory for %s artifact: %w", kind, err))
	}

	// First attempt to use the cached artifact.
	f, err := os.Open(cacheFn)
	switch {
	case err == nil:
		// Already exists in cache.
		if knownHash != "" {
			h := sha256.New()
			if _, err = io.Copy(h, f); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to verify cached %s artifact: %w", kind, err))
			}
			artifactHash := fmt.Sprintf("%x", h.Sum(nil))
			if artifactHash != knownHash {
				cobra.CheckErr(fmt.Errorf("corrupted cached %s artifact file '%s' (expected: %s got: %s)", kind, cacheFn, knownHash, artifactHash))
			}
		}
		f.Close()

		fmt.Printf("  (using cached artifact)\n")
	case errors.Is(err, os.ErrNotExist):
		// Does not exist in cache, download.
		f, err = os.Create(cacheFn)
		if err != nil {
			cobra.CheckErr(fmt.Errorf("failed to create file for %s artifact: %w", kind, err))
		}
		defer f.Close()

		// Download the remote artifact.
		var res *http.Response
		res, err = http.Get(uri) //nolint:gosec,noctx
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
	default:
		cobra.CheckErr(fmt.Errorf("failed to open cached %s artifact: %w", kind, err))
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
	err := os.MkdirAll(filepath.Dir(dst), 0o755)
	if err != nil {
		return fmt.Errorf("failed to create destination directory for '%s': %w", dst, err)
	}

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

	if _, err = io.Copy(df, sf); err != nil {
		return fmt.Errorf("failed to copy '%s': %w", src, err)
	}

	// Ensure times are constant for deterministic builds.
	if err = extractChtimes(dst, time.Time{}, time.Time{}); err != nil {
		return fmt.Errorf("failed to change atime/mtime for '%s': %w", dst, err)
	}

	return nil
}

// ensureBinaryExists checks whether the given binary name exists in path and returns a nice error
// message suggesting to install a given package if it doesn't.
func ensureBinaryExists(buildEnv env.ExecEnv, name, pkg string) error {
	if !buildEnv.HasBinary(name) {
		return fmt.Errorf("missing '%s' binary, please install %s or similar", name, pkg)
	}
	return nil
}

// createSquashFs creates a squashfs filesystem in the given file using directory dir to populate
// it.
//
// Returns the size of the created filesystem image in bytes.
func createSquashFs(buildEnv env.ExecEnv, fn, dir string) (int64, error) {
	const mkSquashFsBin = "mksquashfs"
	if err := ensureBinaryExists(buildEnv, mkSquashFsBin, "squashfs-tools"); err != nil {
		return 0, err
	}

	// Execute mksquashfs.
	cmd := exec.Command(
		mkSquashFsBin,
		dir,
		fn,
		"-comp", "gzip",
		"-noappend",
		"-mkfs-time", "1234",
		"-all-time", "1234",
		"-root-time", "1234",
		"-all-root",
		"-reproducible",
	)
	var out strings.Builder
	cmd.Stderr = &out
	cmd.Stdout = &out
	if err := buildEnv.WrapCommand(cmd); err != nil {
		return 0, err
	}
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("%w\n%s", err, out.String())
	}

	// Measure the size of the resulting image.
	fi, err := os.Stat(fn)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// sha256File computes a SHA-256 digest of the file with the given filename and returns a
// hex-encoded hash.
func sha256File(fn string) (string, error) {
	f, err := os.Open(fn)
	if err != nil {
		return "", fmt.Errorf("failed to open filesystem file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to read filesystem file: %w", err)
	}
	return hex.EncodeToString(h.Sum([]byte{})), nil
}

// createVerityHashTree creates the verity Merkle hash tree and returns the root hash.
func createVerityHashTree(buildEnv env.ExecEnv, fsFn, hashFn string) (string, error) {
	// Print a nicer error message in case veritysetup is missing.
	const veritysetupBin = "veritysetup"
	if err := ensureBinaryExists(buildEnv, veritysetupBin, "cryptsetup-bin"); err != nil {
		return "", err
	}

	// Generate a deterministic salt by hashing the filesystem.
	salt, err := sha256File(fsFn)
	if err != nil {
		return "", err
	}

	rootHashFn := hashFn + ".roothash"

	cmd := exec.Command( //nolint:gosec
		veritysetupBin, "format",
		"--data-block-size=4096",
		"--hash-block-size=4096",
		"--uuid=00000000-0000-0000-0000-000000000000",
		"--salt="+salt,
		"--root-hash-file="+rootHashFn,
		fsFn,
		hashFn,
	)
	var out strings.Builder
	cmd.Stderr = &out
	cmd.Stdout = &out
	if err = buildEnv.WrapCommand(cmd); err != nil {
		return "", err
	}
	if err = cmd.Run(); err != nil {
		return "", fmt.Errorf("%w\n%s", err, out.String())
	}

	if err = buildEnv.FixPermissions(fsFn); err != nil {
		return "", err
	}
	if err = buildEnv.FixPermissions(hashFn); err != nil {
		return "", err
	}
	if err = buildEnv.FixPermissions(rootHashFn); err != nil {
		return "", err
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

// padWithEmptySpace pads the given file with empty space to make it the given size. See
// `appendEmptySpace` for details.
func padWithEmptySpace(fn string, size uint64) error {
	fi, err := os.Stat(fn)
	if err != nil {
		return err
	}

	currentSize := uint64(fi.Size()) //nolint: gosec
	if currentSize >= size {
		return nil
	}
	_, err = appendEmptySpace(fn, size-currentSize, 1)
	return err
}

// appendEmptySpace appends empty space to the given file. If the filesystem supports sparse files,
// this should not actually take any extra space.
//
// The function ensures that the given space respects alignment by adding padding as needed.
//
// Returns the offset where the empty space starts.
func appendEmptySpace(fn string, size uint64, align uint64) (uint64, error) {
	f, err := os.OpenFile(fn, os.O_RDWR, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}
	offset := uint64(fi.Size()) //nolint: gosec

	// Ensure proper alignment.
	if size%align != 0 {
		return 0, fmt.Errorf("size is not properly aligned")
	}
	offset += (align - (offset % align)) % align

	if err = f.Truncate(int64(offset + size)); err != nil { //nolint: gosec
		return 0, err
	}

	return offset, nil
}

// convertToQcow2 converts a raw image to qcow2 format.
func convertToQcow2(buildEnv env.ExecEnv, fn string) error {
	const qemuImgBin = "qemu-img"
	if err := ensureBinaryExists(buildEnv, qemuImgBin, "qemu-utils"); err != nil {
		return err
	}

	tmpOutFn := fn + ".qcow2"

	// Execute qemu-img.
	cmd := exec.Command(
		qemuImgBin,
		"convert",
		"-O", "qcow2",
		fn,
		tmpOutFn,
	)
	var out strings.Builder
	cmd.Stderr = &out
	cmd.Stdout = &out

	if err := buildEnv.WrapCommand(cmd); err != nil {
		return err
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w\n%s", err, out.String())
	}

	if err := os.Rename(tmpOutFn, fn); err != nil {
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}
	return nil
}
