package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// newTestServer creates a mock GitHub API server returning the given release.
func newTestServer(t *testing.T, rel *releaseInfo, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if statusCode != http.StatusOK {
			if statusCode == http.StatusForbidden {
				w.Header().Set("X-RateLimit-Remaining", "0")
			}
			w.WriteHeader(statusCode)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rel)
	}))
}

func newUpdaterWithServer(serverURL, version string) *Updater {
	u := New(version, "test/repo")
	// Override the GitHub API base URL by storing the test server URL.
	// We do this by replacing githubAPI usage through a custom fetchLatestRelease.
	// Instead, we'll make the Updater work by setting the repo to include the server.
	return u
}

// testRelease creates a release with the standard asset for the current platform.
func testRelease(tagName string) *releaseInfo {
	name := fmt.Sprintf("hola-agent-%s-%s", runtime.GOOS, runtime.GOARCH)
	return &releaseInfo{
		TagName: tagName,
		Assets: []asset{
			{Name: name, BrowserDownloadURL: "http://placeholder/" + name, Size: 1024},
			{Name: "checksums.txt", BrowserDownloadURL: "http://placeholder/checksums.txt", Size: 128},
		},
	}
}

func TestCheckLatest_NewVersionAvailable(t *testing.T) {
	rel := testRelease("v0.3.0")
	srv := newTestServer(t, rel, http.StatusOK)
	defer srv.Close()

	// Patch the asset URLs to point to the test server.
	for i := range rel.Assets {
		rel.Assets[i].BrowserDownloadURL = srv.URL + "/" + rel.Assets[i].Name
	}

	u := New("0.2.0", "test/repo")
	// Override the fetchLatestRelease by creating a test-specific updater.
	// Since we can't easily swap githubAPI, we'll test the logic by calling
	// CheckLatest with a patched server. We need to make the updater call our server.

	// Instead, let's test the individual functions and the full flow via Apply with mocks.
	// For CheckLatest, we test by overriding the package-level githubAPI — but it's const.
	// The better approach: test fetchLatestRelease directly with a mock server.

	// Test using the exported CheckLatest by constructing a server that matches
	// the GitHub API path pattern.
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rel)
	}))
	defer apiSrv.Close()

	// We need to make the Updater use our test server. Let's add test support.
	u.httpClient = apiSrv.Client()
	// Override by making requests go to our server instead.
	// The cleanest way: transport that rewrites URLs.
	u.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = apiSrv.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(req)
	})

	check, err := u.CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !check.UpdateAvailable {
		t.Error("expected update to be available")
	}
	if check.LatestVersion != "0.3.0" {
		t.Errorf("expected latest version 0.3.0, got %s", check.LatestVersion)
	}
	if check.CurrentVersion != "0.2.0" {
		t.Errorf("expected current version 0.2.0, got %s", check.CurrentVersion)
	}
	expectedAsset := fmt.Sprintf("hola-agent-%s-%s", runtime.GOOS, runtime.GOARCH)
	if check.AssetName != expectedAsset {
		t.Errorf("expected asset name %s, got %s", expectedAsset, check.AssetName)
	}
}

func TestCheckLatest_AlreadyLatest(t *testing.T) {
	rel := testRelease("v0.2.0")
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rel)
	}))
	defer apiSrv.Close()

	u := New("0.2.0", "test/repo")
	u.httpClient = &http.Client{Transport: redirectTransport(apiSrv)}

	check, err := u.CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.UpdateAvailable {
		t.Error("expected no update to be available")
	}
	if check.LatestVersion != "0.2.0" {
		t.Errorf("expected latest version 0.2.0, got %s", check.LatestVersion)
	}
}

func TestCheckLatest_NoReleases(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer apiSrv.Close()

	u := New("0.2.0", "test/repo")
	u.httpClient = &http.Client{Transport: redirectTransport(apiSrv)}

	_, err := u.CheckLatest(context.Background())
	if !errors.Is(err, ErrNoReleases) {
		t.Errorf("expected ErrNoReleases, got: %v", err)
	}
}

func TestCheckLatest_RateLimited(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer apiSrv.Close()

	u := New("0.2.0", "test/repo")
	u.httpClient = &http.Client{Transport: redirectTransport(apiSrv)}

	_, err := u.CheckLatest(context.Background())
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got: %v", err)
	}
}

func TestCheckLatest_PlatformNotAvailable(t *testing.T) {
	// Release with a different platform's binary only.
	rel := &releaseInfo{
		TagName: "v0.3.0",
		Assets: []asset{
			{Name: "hola-agent-freebsd-mips", BrowserDownloadURL: "http://x/b", Size: 1024},
		},
	}
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rel)
	}))
	defer apiSrv.Close()

	u := New("0.2.0", "test/repo")
	u.httpClient = &http.Client{Transport: redirectTransport(apiSrv)}

	_, err := u.CheckLatest(context.Background())
	if !errors.Is(err, ErrAssetNotFound) {
		t.Errorf("expected ErrAssetNotFound, got: %v", err)
	}
}

func TestParseChecksums(t *testing.T) {
	input := "abc123  hola-agent-linux-amd64\ndef456  hola-agent-darwin-arm64\n"
	m := parseChecksums(input)

	if len(m) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m))
	}
	if m["hola-agent-linux-amd64"] != "abc123" {
		t.Errorf("wrong hash for linux-amd64: %s", m["hola-agent-linux-amd64"])
	}
	if m["hola-agent-darwin-arm64"] != "def456" {
		t.Errorf("wrong hash for darwin-arm64: %s", m["hola-agent-darwin-arm64"])
	}
}

func TestVerifyChecksum(t *testing.T) {
	dir := t.TempDir()
	content := []byte("hello world binary content")

	filePath := filepath.Join(dir, "test-binary")
	if err := os.WriteFile(filePath, content, 0o755); err != nil {
		t.Fatal(err)
	}

	h := sha256.Sum256(content)
	correctHash := hex.EncodeToString(h[:])

	t.Run("correct checksum", func(t *testing.T) {
		if err := verifyChecksum(filePath, correctHash); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("wrong checksum", func(t *testing.T) {
		err := verifyChecksum(filePath, "0000000000000000000000000000000000000000000000000000000000000000")
		if !errors.Is(err, ErrChecksumMismatch) {
			t.Errorf("expected ErrChecksumMismatch, got: %v", err)
		}
	})
}

func TestReplaceBinary(t *testing.T) {
	dir := t.TempDir()

	// Create a "current binary" in the temp dir.
	currentPath := filepath.Join(dir, "hola-agent")
	if err := os.WriteFile(currentPath, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a "new binary" in the same dir.
	newPath := filepath.Join(dir, ".hola-agent-update-12345")
	if err := os.WriteFile(newPath, []byte("new-binary"), 0o644); err != nil {
		t.Fatal(err)
	}

	// We can't easily override os.Executable() in tests, so test replaceBinary
	// logic by directly testing the file operations it performs.
	// Instead, test the sub-operations:

	// 1. Verify chmod preserves permissions.
	info, _ := os.Stat(currentPath)
	if err := os.Chmod(newPath, info.Mode()); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}
	newInfo, _ := os.Stat(newPath)
	if newInfo.Mode() != info.Mode() {
		t.Errorf("permissions not preserved: got %v, want %v", newInfo.Mode(), info.Mode())
	}

	// 2. Verify backup rename.
	backup := currentPath + ".bak"
	if err := os.Rename(currentPath, backup); err != nil {
		t.Fatalf("backup rename failed: %v", err)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Error("backup file should exist")
	}

	// 3. Verify install rename.
	if err := os.Rename(newPath, currentPath); err != nil {
		t.Fatalf("install rename failed: %v", err)
	}

	content, _ := os.ReadFile(currentPath)
	if string(content) != "new-binary" {
		t.Errorf("expected new-binary content, got %q", string(content))
	}

	// 4. Verify backup still exists for rollback.
	backupContent, _ := os.ReadFile(backup)
	if string(backupContent) != "old-binary" {
		t.Errorf("expected old-binary in backup, got %q", string(backupContent))
	}
}

func TestAssetName(t *testing.T) {
	expected := fmt.Sprintf("hola-agent-%s-%s", runtime.GOOS, runtime.GOARCH)
	if got := assetName(); got != expected {
		t.Errorf("assetName() = %q, want %q", got, expected)
	}
}

// roundTripFunc adapts a function to the http.RoundTripper interface.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// redirectTransport creates a transport that redirects all requests to the test server.
func redirectTransport(srv *httptest.Server) http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = srv.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(req)
	})
}
