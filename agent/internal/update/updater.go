package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const githubAPI = "https://api.github.com"

// Updater checks for and applies agent updates from GitHub Releases.
type Updater struct {
	currentVersion string
	repo           string
	httpClient     *http.Client
}

// New creates an Updater for the given repository and current version.
func New(currentVersion, repo string) *Updater {
	return &Updater{
		currentVersion: currentVersion,
		repo:           repo,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
	}
}

// releaseInfo holds information about the latest GitHub release.
type releaseInfo struct {
	TagName string  `json:"tag_name"`
	Assets  []asset `json:"assets"`
}

// asset represents a single file attached to a GitHub release.
type asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int    `json:"size"`
}

// UpdateCheck is the result of checking for available updates.
type UpdateCheck struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	AssetName       string `json:"asset_name,omitempty"`
	AssetSize       int    `json:"asset_size,omitempty"`
}

// CheckLatest queries GitHub for the latest release and compares versions.
func (u *Updater) CheckLatest(ctx context.Context) (*UpdateCheck, error) {
	rel, err := u.fetchLatestRelease(ctx)
	if err != nil {
		return nil, err
	}

	latestVersion := stripVPrefix(rel.TagName)
	cmp, err := compareVersions(u.currentVersion, latestVersion)
	if err != nil {
		return nil, fmt.Errorf("comparing versions: %w", err)
	}

	check := &UpdateCheck{
		CurrentVersion:  u.currentVersion,
		LatestVersion:   latestVersion,
		UpdateAvailable: cmp < 0,
	}

	name := assetName()
	for _, a := range rel.Assets {
		if a.Name == name {
			check.AssetName = a.Name
			check.AssetSize = a.Size
			return check, nil
		}
	}

	// If no update available, missing asset is not an error.
	if !check.UpdateAvailable {
		return check, nil
	}

	return nil, fmt.Errorf("%w: %s", ErrAssetNotFound, name)
}

// Apply downloads the latest release binary, verifies its checksum,
// and replaces the current binary. Returns nil on success.
func (u *Updater) Apply(ctx context.Context) error {
	rel, err := u.fetchLatestRelease(ctx)
	if err != nil {
		return err
	}

	latestVersion := stripVPrefix(rel.TagName)
	cmp, err := compareVersions(u.currentVersion, latestVersion)
	if err != nil {
		return fmt.Errorf("comparing versions: %w", err)
	}
	if cmp >= 0 {
		return ErrAlreadyLatest
	}

	name := assetName()
	var binaryURL string
	var checksumsURL string
	for _, a := range rel.Assets {
		switch a.Name {
		case name:
			binaryURL = a.BrowserDownloadURL
		case "checksums.txt":
			checksumsURL = a.BrowserDownloadURL
		}
	}
	if binaryURL == "" {
		return fmt.Errorf("%w: %s", ErrAssetNotFound, name)
	}
	if checksumsURL == "" {
		return ErrChecksumsNotFound
	}

	slog.Info("downloading checksums", "url", checksumsURL)
	checksums, err := u.downloadChecksums(ctx, checksumsURL)
	if err != nil {
		return fmt.Errorf("downloading checksums: %w", err)
	}

	expectedHash, ok := checksums[name]
	if !ok {
		return fmt.Errorf("%w: no entry for %s in checksums.txt", ErrChecksumMismatch, name)
	}

	slog.Info("downloading binary", "asset", name, "version", latestVersion)
	tmpPath, err := u.downloadAsset(ctx, binaryURL)
	if err != nil {
		return fmt.Errorf("downloading binary: %w", err)
	}
	defer func() {
		// Clean up temp file on any error (replaceBinary renames it on success).
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	if err = verifyChecksum(tmpPath, expectedHash); err != nil {
		return err
	}

	slog.Info("replacing binary", "version", latestVersion)
	if err = replaceBinary(tmpPath); err != nil {
		return fmt.Errorf("replacing binary: %w", err)
	}

	slog.Info("agent updated successfully", "from", u.currentVersion, "to", latestVersion)
	return nil
}

// fetchLatestRelease calls the GitHub API for the latest release.
func (u *Updater) fetchLatestRelease(ctx context.Context) (*releaseInfo, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", githubAPI, u.repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "hola-agent/"+u.currentVersion)

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching latest release: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// continue below
	case http.StatusNotFound:
		return nil, ErrNoReleases
	case http.StatusForbidden:
		if resp.Header.Get("X-RateLimit-Remaining") == "0" {
			return nil, ErrRateLimited
		}
		return nil, fmt.Errorf("GitHub API returned 403")
	default:
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var rel releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decoding release: %w", err)
	}
	return &rel, nil
}

// downloadAsset downloads a URL to a temp file in the same directory as the
// current binary (required for os.Rename to work across filesystems).
func (u *Updater) downloadAsset(ctx context.Context, url string) (string, error) {
	execPath, err := executablePath()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(execPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "hola-agent/"+u.currentVersion)

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp(dir, ".hola-agent-update-*")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	defer tmp.Close()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("writing download: %w", err)
	}

	return tmp.Name(), nil
}

// downloadChecksums fetches checksums.txt and parses it into a map[filename]hash.
func (u *Updater) downloadChecksums(ctx context.Context, url string) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "hola-agent/"+u.currentVersion)

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksums download returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading checksums: %w", err)
	}

	return parseChecksums(string(body)), nil
}

// parseChecksums parses sha256sum-format text into a map[filename]hash.
// Format: "<hash>  <filename>" (two spaces, per sha256sum convention).
func parseChecksums(text string) map[string]string {
	m := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 {
			m[parts[1]] = parts[0]
		}
	}
	return m
}

// verifyChecksum computes SHA256 of the file and compares with expected.
func verifyChecksum(filePath, expectedHash string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening file for checksum: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("computing checksum: %w", err)
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, expectedHash) {
		return fmt.Errorf("%w: expected %s, got %s", ErrChecksumMismatch, expectedHash, actual)
	}
	return nil
}

// replaceBinary backs up the current binary and replaces it with the new one.
func replaceBinary(newBinaryPath string) error {
	execPath, err := executablePath()
	if err != nil {
		return err
	}

	// Preserve the current binary's permissions.
	info, err := os.Stat(execPath)
	if err != nil {
		return fmt.Errorf("stat current binary: %w", err)
	}
	if err := os.Chmod(newBinaryPath, info.Mode()); err != nil {
		return fmt.Errorf("chmod new binary: %w", err)
	}

	backup := execPath + ".bak"
	if err := os.Rename(execPath, backup); err != nil {
		return fmt.Errorf("backup current binary: %w", err)
	}

	if err := os.Rename(newBinaryPath, execPath); err != nil {
		// Attempt rollback.
		slog.Error("install failed, rolling back", "error", err)
		if rbErr := os.Rename(backup, execPath); rbErr != nil {
			slog.Error("rollback also failed", "error", rbErr)
		}
		return fmt.Errorf("install new binary: %w", err)
	}

	return nil
}

// executablePath resolves the real path to the running binary.
func executablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolving executable path: %w", err)
	}
	real, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolving symlinks: %w", err)
	}
	return real, nil
}

// assetName returns the expected binary name for the current platform.
func assetName() string {
	return fmt.Sprintf("hola-agent-%s-%s", runtime.GOOS, runtime.GOARCH)
}
