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

// ensureBinaryExists checks whether the given binary name exists in path and returns a nice error
// message suggesting to install a given package if it doesn't.
func ensureBinaryExists(buildEnv env.ExecEnv, name, pkg string) error {
	if !buildEnv.HasBinary(name) {
		return fmt.Errorf("missing '%s' binary, please install %s or similar", name, pkg)
	}
	return nil
}

// cleanEnvForReproducibility filters out locale and timestamp environment variables and sets
// consistent values for reproducible builds.
func cleanEnvForReproducibility() []string {
	env := []string{}
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "LC_") && !strings.HasPrefix(e, "LANG=") && !strings.HasPrefix(e, "TZ=") && !strings.HasPrefix(e, "SOURCE_DATE_EPOCH=") {
			env = append(env, e)
		}
	}
	return append(env, "LC_ALL=C", "TZ=UTC", "SOURCE_DATE_EPOCH=0")
}

// extraFile represents a file to be injected into the rootfs.
type extraFile struct {
	HostPath string
	TarPath  string
	Mode     os.FileMode
}

// normalizeHeader normalizes a tar header for reproducible builds.
// Uses GNU format because sqfstar < 4.6 has a bug with PAX headers
// that incorrectly strips pathname components in linkpath for symlinks.
func normalizeHeader(header *tar.Header) {
	header.Format = tar.FormatGNU
	header.Uid = 0
	header.Gid = 0
	header.Uname = ""
	header.Gname = ""
	header.ModTime = time.Unix(0, 0)
	header.AccessTime = time.Time{}
	header.ChangeTime = time.Time{}
	header.PAXRecords = nil
}

// createSquashFsFromTar creates a squashfs filesystem by streaming a tar.bz2 template
// and injecting extra files, without extracting to disk.
//
// Returns the size of the created filesystem image in bytes.
func createSquashFsFromTar(buildEnv env.ExecEnv, outputFn, templateFn string, extraFiles []extraFile) (int64, error) {
	const (
		sqfstarBin  = "sqfstar"
		fakerootBin = "fakeroot"
	)
	if err := ensureBinaryExists(buildEnv, sqfstarBin, "squashfs-tools"); err != nil {
		return 0, err
	}
	if err := ensureBinaryExists(buildEnv, fakerootBin, fakerootBin); err != nil {
		return 0, err
	}

	// Open the template tar.bz2.
	templateFile, err := os.Open(templateFn)
	if err != nil {
		return 0, fmt.Errorf("failed to open template archive: %w", err)
	}
	defer templateFile.Close()

	bzReader := bzip2.NewReader(templateFile)
	tarReader := tar.NewReader(bzReader)

	// Convert output path for container environment if needed.
	outputFnEnv, err := buildEnv.PathToEnv(outputFn)
	if err != nil {
		return 0, fmt.Errorf("failed to convert output path: %w", err)
	}

	cmd := exec.Command(
		fakerootBin,
		"--",
		sqfstarBin,
		outputFnEnv,
		"-reproducible",
		"-comp", "gzip",
		"-b", "1M",
		"-processors", "1",
		"-noappend",
		"-mkfs-time", "0",
		"-all-time", "0",
		"-nopad",
		"-all-root",
		"-force-uid", "0",
		"-force-gid", "0",
	)
	cmd.Env = cleanEnvForReproducibility()

	var cmdOut strings.Builder
	cmd.Stderr = &cmdOut
	cmd.Stdout = &cmdOut

	// Create pipe for stdin.
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return 0, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	if err := buildEnv.WrapCommand(cmd); err != nil {
		return 0, err
	}
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start sqfstar: %w", err)
	}

	// Create tar writer that writes to both sqfstar's stdin and a hash for verification.
	tarHasher := sha256.New()
	tarWriter := tar.NewWriter(io.MultiWriter(stdinPipe, tarHasher))

	// Ensure cleanup on error.
	var cmdErr error
	defer func() {
		if cmdErr != nil {
			tarWriter.Close()
			stdinPipe.Close()
			cmd.Wait() //nolint:errcheck
		}
	}()

	// Build a map of extra files by their tar path for quick lookup.
	extraFileMap := make(map[string]extraFile)
	for _, ef := range extraFiles {
		extraFileMap[ef.TarPath] = ef
	}

	// Track directories seen from the template to ensure parent dirs exist for extra files.
	seenDirs := make(map[string]bool)

	// Copy entries from template, skipping ones we'll replace.
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			cmdErr = fmt.Errorf("error reading template archive: %w", err)
			return 0, cmdErr
		}

		// Normalize the path.
		cleanPath := strings.TrimPrefix(header.Name, "./")

		// Track directories from template.
		if header.Typeflag == tar.TypeDir {
			seenDirs[strings.TrimSuffix(cleanPath, "/")] = true
		}

		// Skip if this path will be replaced by an extra file.
		if _, willReplace := extraFileMap[cleanPath]; willReplace {
			// Drain the body if it's a regular file.
			if header.Typeflag == tar.TypeReg && header.Size > 0 {
				if _, err := io.Copy(io.Discard, io.LimitReader(tarReader, header.Size)); err != nil {
					cmdErr = fmt.Errorf("failed to skip file content: %w", err)
					return 0, cmdErr
				}
			}
			continue
		}

		// Normalize header for reproducibility.
		normalizeHeader(header)

		if err := tarWriter.WriteHeader(header); err != nil {
			cmdErr = fmt.Errorf("failed to write tar header: %w", err)
			return 0, cmdErr
		}

		// Copy body for regular files.
		if header.Typeflag == tar.TypeReg {
			const maxFileSize = 500 * 1024 * 1024 // 500 MB sanity limit
			if header.Size > maxFileSize {
				cmdErr = fmt.Errorf("file too large: %s (%d bytes)", header.Name, header.Size)
				return 0, cmdErr
			}
			if _, err := io.Copy(tarWriter, io.LimitReader(tarReader, header.Size)); err != nil {
				cmdErr = fmt.Errorf("failed to copy file content: %w", err)
				return 0, cmdErr
			}
		}
	}

	// Add extra files, creating parent directories as needed.
	for _, ef := range extraFileMap {
		if err := addFileToTar(tarWriter, ef.HostPath, ef.TarPath, ef.Mode, seenDirs); err != nil {
			cmdErr = fmt.Errorf("failed to add extra file %s: %w", ef.TarPath, err)
			return 0, cmdErr
		}
	}

	// Close tar writer and stdin pipe.
	if err := tarWriter.Close(); err != nil {
		cmdErr = fmt.Errorf("failed to close tar writer: %w", err)
		return 0, cmdErr
	}

	// Print tar hash for verification.
	fmt.Printf("TAR archive SHA256: %s\n", hex.EncodeToString(tarHasher.Sum(nil)))

	if err := stdinPipe.Close(); err != nil {
		cmdErr = fmt.Errorf("failed to close stdin pipe: %w", err)
		return 0, cmdErr
	}

	// Wait for sqfstar to finish.
	if err := cmd.Wait(); err != nil {
		return 0, fmt.Errorf("sqfstar failed: %w\nOutput: %s", err, cmdOut.String())
	}

	// Measure the size of the resulting image.
	fi, err := os.Stat(outputFn)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// addFileToTar adds a file from the host filesystem to a tar archive,
// creating any missing parent directory entries first.
func addFileToTar(tw *tar.Writer, hostPath, tarPath string, mode os.FileMode, seenDirs map[string]bool) error {
	// Create missing parent directories.
	var dirsToCreate []string
	for dir := filepath.Dir(tarPath); dir != "." && dir != "/" && dir != ""; dir = filepath.Dir(dir) {
		if !seenDirs[dir] {
			dirsToCreate = append(dirsToCreate, dir)
			seenDirs[dir] = true
		}
	}
	for i := len(dirsToCreate) - 1; i >= 0; i-- {
		dirHeader := &tar.Header{
			Name:     "./" + dirsToCreate[i] + "/",
			Mode:     0o755,
			Typeflag: tar.TypeDir,
		}
		normalizeHeader(dirHeader)
		if err := tw.WriteHeader(dirHeader); err != nil {
			return fmt.Errorf("failed to write directory header: %w", err)
		}
	}

	// Add the file.
	f, err := os.Open(hostPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	header := &tar.Header{
		Name:     "./" + tarPath,
		Mode:     int64(mode),
		Size:     fi.Size(),
		Typeflag: tar.TypeReg,
	}
	normalizeHeader(header)

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	if _, err := io.Copy(tw, f); err != nil {
		return fmt.Errorf("failed to write content: %w", err)
	}

	return nil
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

	// Translate paths for container environments.
	fsFnEnv, err := buildEnv.PathToEnv(fsFn)
	if err != nil {
		return "", fmt.Errorf("failed to translate filesystem path for container: %w", err)
	}

	hashFnEnv, err := buildEnv.PathToEnv(hashFn)
	if err != nil {
		return "", fmt.Errorf("failed to translate hash path for container: %w", err)
	}

	rootHashFn := hashFn + ".roothash"
	rootHashFnEnv, err := buildEnv.PathToEnv(rootHashFn)
	if err != nil {
		return "", fmt.Errorf("failed to translate roothash path for container: %w", err)
	}

	cmd := exec.Command( //nolint:gosec
		veritysetupBin, "format",
		"--data-block-size=4096",
		"--hash-block-size=4096",
		"--uuid=00000000-0000-0000-0000-000000000000",
		"--salt="+salt,
		"--root-hash-file="+rootHashFnEnv,
		fsFnEnv,
		hashFnEnv,
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
func padWithEmptySpace(buildEnv env.ExecEnv, fn string, size uint64) error {
	fi, err := os.Stat(fn)
	if err != nil {
		return err
	}

	currentSize := uint64(fi.Size()) //nolint: gosec
	if currentSize >= size {
		return nil
	}
	_, err = appendEmptySpace(buildEnv, fn, size-currentSize, 1)
	return err
}

// appendEmptySpace appends empty space to the given file. If the filesystem supports sparse files,
// this should not actually take any extra space. Padding is performed inside the build environment
// (host or container) so size observations are consistent.
//
// The function ensures that the given space respects alignment by adding padding as needed.
//
// Returns the offset where the empty space starts.
func appendEmptySpace(buildEnv env.ExecEnv, fn string, size uint64, align uint64) (uint64, error) {
	fnEnv, err := buildEnv.PathToEnv(fn)
	if err != nil {
		return 0, fmt.Errorf("failed to translate path for padding: %w", err)
	}

	fi, err := os.Stat(fn)
	if err != nil {
		return 0, err
	}
	offset := uint64(fi.Size()) //nolint: gosec

	// Ensure proper alignment.
	if size%align != 0 {
		return 0, fmt.Errorf("size is not properly aligned")
	}
	offset += (align - (offset % align)) % align

	newSize := offset + size
	cmd := exec.Command("truncate", "-s", fmt.Sprintf("%d", newSize), fnEnv) //nolint:gosec // fnEnv is derived from internal build state
	if err := buildEnv.WrapCommand(cmd); err != nil {
		return 0, err
	}
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("failed to pad file: %w", err)
	}

	return offset, nil
}

// verifyRawImage verifies the raw stage2 image has valid dm-verity.
// This verification step also ensures the file data is fully consistent before
// qcow2 conversion, which is necessary for reliable image generation.
func verifyRawImage(buildEnv env.ExecEnv, stage2 *tdxStage2) error {
	const veritysetupBin = "veritysetup"
	if err := ensureBinaryExists(buildEnv, veritysetupBin, "cryptsetup-bin"); err != nil {
		return err
	}

	fnEnv, err := buildEnv.PathToEnv(stage2.fn)
	if err != nil {
		return err
	}

	cmd := exec.Command( //nolint:gosec // Arguments are from internal build state.
		veritysetupBin, "verify",
		fmt.Sprintf("--hash-offset=%d", stage2.fsSize),
		fnEnv, fnEnv, stage2.rootHash,
	)
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := buildEnv.WrapCommand(cmd); err != nil {
		return err
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("veritysetup verify failed: %w\n%s", err, out.String())
	}

	return nil
}

// convertToQcow2 converts a raw image to qcow2 format.
func convertToQcow2(buildEnv env.ExecEnv, fn string) error {
	const qemuImgBin = "qemu-img"
	if err := ensureBinaryExists(buildEnv, qemuImgBin, "qemu-utils"); err != nil {
		return err
	}

	tmpOutFn := fn + ".qcow2"
	fnEnv, err := buildEnv.PathToEnv(fn)
	if err != nil {
		return fmt.Errorf("failed to translate input path: %w", err)
	}
	tmpOutFnEnv, err := buildEnv.PathToEnv(tmpOutFn)
	if err != nil {
		return fmt.Errorf("failed to translate output path: %w", err)
	}

	// Convert with explicit raw format to avoid misdetection.
	cmd := exec.Command(
		qemuImgBin,
		"convert",
		"-f", "raw",
		"-O", "qcow2",
		fnEnv,
		tmpOutFnEnv,
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
