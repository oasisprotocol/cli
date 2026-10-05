package cmd

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	cliVersion "github.com/oasisprotocol/cli/version"
)

func makeReleaseJSON(tag, body, assetName, assetURL string) string {
	return fmt.Sprintf(`{
	"tag_name": %q,
	"body": %q,
	"assets": [
		{"name": %q, "browser_download_url": %q},
		{"name": %q, "browser_download_url": %q}
	]
}`, tag, body, assetName, assetURL, assetName+".sha256", assetURL+".sha256")
}

func tarGz(t *testing.T, path, exeName string, content []byte) {
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create tarGz: %v", err)
	}
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)

	hdr := &tar.Header{Name: exeName, Mode: 0o755, Size: int64(len(content))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("hdr: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tw close: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("gzw close: %v", err)
	}
	_ = f.Close()
}

func TestUpdate_PositiveFlow(t *testing.T) {
	tmp := t.TempDir()
	exeName := "oasis"
	if runtime.GOOS == osWindows {
		exeName += ".exe"
	}
	binPath := filepath.Join(tmp, exeName)

	if err := os.WriteFile(binPath, []byte("old"), 0o600); err != nil {
		t.Fatalf("prep bin: %v", err)
	}

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/oasisprotocol/cli/releases/latest":
			fmt.Fprint(w, makeReleaseJSON("v99.0.0", "body", "oasis-"+runtime.GOOS+"-"+runtime.GOARCH+".tar.gz", ts.URL+"/asset"))
		case "/asset":
			asset := filepath.Join(tmp, "asset.tgz")
			tarGz(t, asset, exeName, []byte("new"))
			http.ServeFile(w, r, asset)
		case "/asset.sha256":
			asset := filepath.Join(tmp, "asset.tgz")
			f, err := os.Open(asset)
			if err != nil {
				t.Fatalf("open asset for sha: %v", err)
			}
			defer f.Close()

			h := sha256.New()
			if _, err := io.Copy(h, f); err != nil {
				t.Fatalf("hash asset: %v", err)
			}
			sum := hex.EncodeToString(h.Sum(nil))
			// Single-field format is accepted by verifySHA256.
			fmt.Fprintln(w, sum)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	oldAPI := githubAPIBase
	oldDL := download
	githubAPIBase = ts.URL
	// Custom downloader that works over plain HTTP for the test server.
	download = func(ctx context.Context, url string) (string, error) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return "", fmt.Errorf("download failed: %s", res.Status)
		}

		tmp, err := os.CreateTemp("", "oasis-update-test-*")
		if err != nil {
			return "", err
		}
		defer tmp.Close()

		if _, err = io.Copy(tmp, res.Body); err != nil {
			return "", err
		}
		return tmp.Name(), nil
	}
	defer func() {
		githubAPIBase = oldAPI
		download = oldDL
	}()

	exit := runTestUpdate(binPath)
	if exit != 0 {
		t.Fatalf("unexpected exit code %d", exit)
	}

	got, _ := os.ReadFile(binPath)

	// Non-Windows platforms: the binary must be replaced in place.
	if runtime.GOOS != osWindows {
		if string(got) != "new" {
			t.Fatalf("binary not replaced, got %q", got)
		}
		return
	}

	// Windows: updater may stage a replacement binary when in-place rename fails.
	if _, err := os.Stat(binPath + ".new"); err == nil {
		// Staged file exists; success.
		return
	}

	// No staged file; ensure the binary was replaced in place.
	if string(got) != "new" {
		t.Fatalf("binary not replaced or staged on windows; got %q", got)
	}
}

func TestUpdate_AlreadyUpToDate(t *testing.T) {
	tmp := t.TempDir()
	exe := filepath.Join(tmp, "oasis")
	os.WriteFile(exe, []byte("current"), 0o600) //nolint:errcheck

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, makeReleaseJSON("v0.0.0", "-", "-", "-"))
	}))
	defer ts.Close()

	oldAPI := githubAPIBase
	githubAPIBase = ts.URL
	defer func() { githubAPIBase = oldAPI }()

	// Override current CLI version to match latest so updater thinks we're up to date.
	oldVer := cliVersion.Software
	cliVersion.Software = "0.0.0"
	defer func() { cliVersion.Software = oldVer }()

	if code := runTestUpdate(exe); code != 0 {
		t.Fatalf("expected 0 exit, got %d", code)
	}
}

// runTestUpdate is a local runner that avoids cobra / os.Exit in tests.
func runTestUpdate(oasisBinaryPath string) int {
	// monkey patch os.Executable
	oldFn := osExecutable
	osExecutable = func() (string, error) { return oasisBinaryPath, nil }
	defer func() { osExecutable = oldFn }()

	// Ensure a clean slate for cobra flags between tests.
	rootCmd.ResetFlags()
	rootCmd.SetArgs([]string{"update", "--yes"})
	err := rootCmd.Execute()
	if err != nil {
		return 1
	}
	return 0
}
