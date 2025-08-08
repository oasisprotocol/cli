package cmd

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	progress "github.com/oasisprotocol/cli/cmd/common"

	cliVersion "github.com/oasisprotocol/cli/version"
	"github.com/spf13/cobra"
)

const (
	apiTimeout           = 15 * time.Second
	downloadTimeout      = 5 * time.Minute
	maxChecksumSize      = 16 * 1024
	retryAfterMaxSeconds = 60 // upper bound for GitHub's Retry-After header
)

const (
	osWindows     = "windows"
	binaryName    = "oasis"
	binaryNameWin = binaryName + ".exe"
)

var httpClient = &http.Client{
	Transport: &http.Transport{
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		Proxy:                 http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	},
}

var (
	githubAPIBase = "https://api.github.com"
	download      = doDownload
	osExecutable  = os.Executable
	// I have to use a string for DisableUpdateCmd because of the limitations of the -X flag in Go build.
	DisableUpdateCmd = "false"
)

// semverDiffers reports whether versions a and b differ (ignoring leading 'v').
func semverDiffers(a, b string) (bool, error) {
	normalize := func(s string) string {
		s = strings.TrimPrefix(s, "v")
		// Strip pre-release (-) and build metadata (+)
		if idx := strings.IndexAny(s, "+-"); idx != -1 {
			s = s[:idx]
		}
		return s
	}

	aNorm := normalize(a)
	bNorm := normalize(b)

	if aNorm == "" || bNorm == "" {
		return false, fmt.Errorf("invalid version string (empty)")
	}
	return aNorm != bNorm, nil
}

// ghRelease mirrors the subset of fields returned by GitHub’s
// “Get the latest release” endpoint.
// https://docs.github.com/en/rest/releases/releases?apiVersion=2022-11-28#get-the-latest-release
type ghRelease struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func fetchLatest(ctx context.Context) (*ghRelease, error) {
	url := githubAPIBase + "/repos/oasisprotocol/cli/releases/latest"

	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		req.Header.Set("User-Agent", "oasis-cli-updater")
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		if t := os.Getenv("GITHUB_TOKEN"); t != "" {
			req.Header.Set("Authorization", "Bearer "+t)
		}

		res, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		if res.StatusCode == http.StatusForbidden {
			if sec, _ := strconv.Atoi(res.Header.Get("Retry-After")); sec > 0 && sec < retryAfterMaxSeconds {
				res.Body.Close()
				time.Sleep(time.Duration(sec) * time.Second)
				continue
			}
		}

		if res.StatusCode != http.StatusOK {
			err = fmt.Errorf("GitHub API: %s", res.Status)
			res.Body.Close()
			return nil, err
		}

		var rel ghRelease
		decodeErr := json.NewDecoder(res.Body).Decode(&rel)
		res.Body.Close()
		return &rel, decodeErr
	}
}

var archRE = map[string]*regexp.Regexp{
	"amd64": regexp.MustCompile(`(?:^|[-_])(amd64|x86_64|x64)(?:[-_.]|$)`),
	"arm64": regexp.MustCompile(`(?:^|[-_])(arm64|aarch64|universal)(?:[-_.]|$)`),
}

// archAliases returns candidate substrings ordered by preference.
func archAliases(arch string) []string {
	switch arch {
	case "amd64":
		return []string{"amd64", "x86_64", "x64", "universal", "all"}
	case "arm64":
		return []string{"arm64", "aarch64", "universal", "all"}
	default:
		return []string{arch, "all"}
	}
}

func matches(goos, goarch, asset string) bool {
	if !strings.Contains(asset, goos) {
		return false
	}
	if re, ok := archRE[goarch]; ok {
		return re.MatchString(asset)
	}
	return strings.Contains(asset, goarch)
}

func findAssetFor(rel *ghRelease, goos, goarch string) (name, url string, err error) {
	for _, wantArch := range archAliases(goarch) {
		for _, a := range rel.Assets {
			if matches(goos, wantArch, strings.ToLower(a.Name)) {
				return a.Name, a.BrowserDownloadURL, nil
			}
		}
	}
	names := make([]string, 0, len(rel.Assets))
	for _, a := range rel.Assets {
		names = append(names, a.Name)
	}
	return "", "", fmt.Errorf("no asset for %s/%s (available: %v)", goos, goarch, names)
}

func findAsset(rel *ghRelease) (name, url string, err error) {
	return findAssetFor(rel, runtime.GOOS, runtime.GOARCH)
}

func findSha256For(rel *ghRelease) (url string, ok bool) {
	for _, a := range rel.Assets {
		lower := strings.ToLower(a.Name)
		if strings.Contains(lower, "sha256") || strings.Contains(lower, "checksums") {
			return a.BrowserDownloadURL, true
		}
	}
	return "", false
}

// doDownload fetches the URL to a temporary file and reports its path.
func doDownload(ctx context.Context, url string) (string, error) {
	if !strings.HasPrefix(url, "https://") {
		return "", errors.New("refusing to download from non-HTTPS URL")
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	res, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %s", res.Status)
	}

	tmp, err := os.CreateTemp("", "oasis-update-*")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	pw := progress.NewProgressWriter(res.ContentLength, progress.WithMessage("Downloading..."))
	_, err = io.Copy(tmp, io.TeeReader(res.Body, pw))
	if err != nil {
		return "", err
	}
	return tmp.Name(), nil
}

func replaceExecutable(dest string, r io.Reader, perm os.FileMode) error {
	dir := filepath.Dir(dest)

	tmp, err := os.CreateTemp(dir, ".oasis-new-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	cleanup := true
	defer func() {
		if cleanup {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err = io.Copy(tmp, r); err != nil {
		return err
	}

	if perm == 0 {
		perm = 0o755
	}
	if err = tmp.Chmod(perm); err != nil && runtime.GOOS != osWindows {
		return err
	}

	if err = tmp.Close(); err != nil {
		return err
	}

	if err = os.Rename(tmpPath, dest); err == nil {
		cleanup = false
		return nil
	}

	if runtime.GOOS == osWindows {
		pending := dest + ".new"
		if err2 := os.Rename(tmpPath, pending); err2 != nil {
			return fmt.Errorf("rename failed (%v / %v)", err, err2)
		}
		scheduleWindowsMove(pending, dest)
		cleanup = false
		fmt.Println("Update staged. It will be applied automatically on next launch.")
		return nil
	}

	return err
}

func scheduleWindowsMove(src, dst string) {
	if runtime.GOOS != osWindows {
		return
	}

	curPID := os.Getpid()

	// Prefer modern PowerShell Core if available.
	shell := "powershell"
	if pwsh, err := exec.LookPath("pwsh"); err == nil {
		shell = pwsh
	}

	psScript := fmt.Sprintf(
		`Wait-Process -Id %d -Timeout 15; Move-Item -LiteralPath %q -Destination %q -Force`,
		curPID, src, dst,
	)
	psCmd := exec.Command(shell,
		"-NoLogo", "-NoProfile", "-Command", psScript,
	)
	if err := psCmd.Start(); err == nil {
		return
	}

	// Fallback to classic cmd.exe with timeout; /nobreak prevents key-press abort.
	_ = exec.Command("cmd", "/C",
		"timeout", "/t", "2", "/nobreak", ">", "nul", "&&",
		"move", "/Y", src, dst,
	).Start()
}

func finishPendingWindowsUpdate(exePath string) {
	if runtime.GOOS != osWindows {
		return
	}
	pending := exePath + ".new"
	if _, err := os.Stat(pending); err == nil {
		_ = os.Rename(pending, exePath)
	}
}

func extractBinary(binPath, assetPath, assetName string) error {
	lower := strings.ToLower(assetName)

	switch {
	case strings.HasSuffix(lower, ".zip"):
		zr, err := zip.OpenReader(assetPath)
		if err != nil {
			return err
		}
		defer zr.Close()

		for _, f := range zr.File {
			base := filepath.Base(f.Name)
			if base == binaryName || base == binaryNameWin {
				rc, err := f.Open()
				if err != nil {
					return err
				}
				defer rc.Close()
				return replaceExecutable(binPath, rc, f.Mode())
			}
		}
		return errors.New("binary not found in zip")

	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		f, err := os.Open(assetPath)
		if err != nil {
			return err
		}
		defer f.Close()

		gzr, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gzr.Close()

		tr := tar.NewReader(gzr)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			base := filepath.Base(hdr.Name)
			if hdr.Typeflag == tar.TypeReg && (base == binaryName || base == binaryNameWin) {
				return replaceExecutable(binPath, tr, hdr.FileInfo().Mode())
			}
		}
		return errors.New("binary not found in tar.gz")

	default:
		f, err := os.Open(assetPath)
		if err != nil {
			return err
		}
		defer f.Close()
		return replaceExecutable(binPath, f, 0)
	}
}

func verifySHA256(filePath, shaPath, assetName string) error {
	info, err := os.Stat(shaPath)
	if err != nil {
		return err
	}
	if info.Size() > maxChecksumSize {
		return fmt.Errorf("checksum file too large: %d bytes", info.Size())
	}

	f, err := os.Open(shaPath)
	if err != nil {
		return err
	}
	defer f.Close()

	var hashHex string
	scanner := bufio.NewScanner(f)
	base := filepath.Base(assetName)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue // ignore blanks & comments
		}
		fields := strings.Fields(line)
		if len(fields) == 1 {
			// Single-line checksum file.
			hashHex = fields[0]
			break
		}
		if len(fields) >= 2 && fields[1] == base {
			hashHex = fields[0]
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if hashHex == "" {
		return fmt.Errorf("checksum for %q not found in %q", base, shaPath)
	}

	want, err := hex.DecodeString(hashHex)
	if err != nil || len(want) != sha256.Size {
		return errors.New("invalid sha256 checksum")
	}

	bin, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer bin.Close()

	h := sha256.New()
	if _, err = io.Copy(h, bin); err != nil {
		return err
	}
	got := h.Sum(nil)

	if !bytes.Equal(got, want) {
		return fmt.Errorf("checksum mismatch: expected %s got %s",
			hashHex, hex.EncodeToString(got))
	}
	return nil
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Download and install the newest Oasis CLI release",
	Long: `Checks GitHub releases for a newer Oasis CLI version that matches
your platform, prints the changelog, optionally asks for confirmation,
then downloads and replaces the current binary.`,
	Args: cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		exePath, err := osExecutable()
		cobra.CheckErr(err)
		finishPendingWindowsUpdate(exePath)

		apiCtx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()

		fmt.Println("Checking for updates...")
		rel, err := fetchLatest(apiCtx)
		cobra.CheckErr(err)

		current := cliVersion.Software
		latest := rel.TagName

		isNewer, err := semverDiffers(latest, current)
		cobra.CheckErr(err)
		if !isNewer {
			fmt.Printf("Oasis CLI is already up to date (%s).\n", current)
			return
		}

		assetName, assetURL, err := findAsset(rel)
		cobra.CheckErr(err)

		fmt.Printf("New version %s available (%s).\n", latest, assetName)

		fmt.Printf("\n─ Changelog for %s ─\n%s\n", latest, rel.Body)

		// Ask for confirmation unless -y/--yes used.
		progress.Confirm("Upgrade now?", "upgrade cancelled")

		dlCtx, cancelDL := context.WithTimeout(context.Background(), downloadTimeout)
		defer cancelDL()

		assetPath, err := download(dlCtx, assetURL)
		cobra.CheckErr(err)
		defer func() { _ = os.Remove(assetPath) }()

		shaURL, ok := findSha256For(rel) // updated call
		if !ok {
			cobra.CheckErr(fmt.Errorf("sha256 checksum not found"))
		}

		fmt.Println("Verifying integrity...")
		shaPath, err2 := download(dlCtx, shaURL)
		cobra.CheckErr(err2)
		defer func() { _ = os.Remove(shaPath) }()
		cobra.CheckErr(verifySHA256(assetPath, shaPath, assetName))
		fmt.Println("Checksum verified OK")

		fmt.Println("Installing...")
		cobra.CheckErr(extractBinary(exePath, assetPath, assetName))

		fmt.Printf("Updated to %s\n", latest)
	},
}

func init() {
	if DisableUpdateCmd == "true" {
		return
	}
	updateCmd.Flags().AddFlagSet(progress.AnswerYesFlag)
	rootCmd.AddCommand(updateCmd)
}
