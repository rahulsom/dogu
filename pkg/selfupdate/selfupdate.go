package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/dogu-dev/dogu/pkg/tools"
	"github.com/dogu-dev/dogu/pkg/version"
)

const (
	DefaultGitHubOwner = "rahulsom"
	DefaultGitHubRepo  = "dogu"
)

// GitHubBaseURL allows overriding the GitHub API endpoint during tests.
var GitHubBaseURL = "https://api.github.com"

// ReleaseAsset represents an individual downloadable asset in a GitHub release.
type ReleaseAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int64  `json:"size"`
}

// Release represents a GitHub release.
type Release struct {
	TagName string         `json:"tag_name"`
	Name    string         `json:"name"`
	Assets  []ReleaseAsset `json:"assets"`
}

// CheckForUpdate checks GitHub for a newer release with a compatible binary.
// Returns the latest release, a boolean indicating if an update is available, and an error if any.
func CheckForUpdate(ctx context.Context, currentVersion, owner, repo string) (*Release, bool, error) {
	if owner == "" {
		owner = DefaultGitHubOwner
	}
	if repo == "" {
		repo = DefaultGitHubRepo
	}

	reqURL := fmt.Sprintf("%s/repos/%s/%s/releases/latest", GitHubBaseURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("failed to check github releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("github api returned status %s", resp.Status)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, false, fmt.Errorf("failed to decode release payload: %w", err)
	}

	latestVer := rel.TagName
	if latestVer == "" {
		latestVer = rel.Name
	}
	if latestVer == "" {
		return nil, false, fmt.Errorf("release has no tag or name")
	}

	// Compare currentVersion and latestVer
	if isUpdateAvailable(currentVersion, latestVer) {
		if !hasCompatibleAsset(&rel) {
			return &rel, false, nil
		}
		return &rel, true, nil
	}

	return &rel, false, nil
}

// isUpdateAvailable returns true if latestVersion is strictly greater than currentVersion.
func isUpdateAvailable(currentVersion, latestVersion string) bool {
	if currentVersion == "" || currentVersion == "dev" || currentVersion == "(devel)" {
		return false
	}
	cmp := tools.CompareVersions(latestVersion, currentVersion)
	return cmp > 0
}

// SuggestUpdateNotification prints a user-friendly suggestion that a newer version is available.
func SuggestUpdateNotification(out io.Writer, currentVersion, latestVersion string) {
	alertStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")) // Yellow/amber
	cmdStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))    // Vibrant green
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))              // Muted gray

	msg := fmt.Sprintf("\n%s A new version of dogu is available: %s -> %s\n%s Run '%s' to update.\n",
		alertStyle.Render("╭─ Update available:"),
		dimStyle.Render(currentVersion),
		alertStyle.Render(latestVersion),
		alertStyle.Render("╰─"),
		cmdStyle.Render("dogu self-update"),
	)
	fmt.Fprint(out, msg)
}

// FindMatchingAsset finds the asset best matching GOOS, GOARCH, and arm suffix for the current platform.
func FindMatchingAsset(assets []ReleaseAsset, goos, goarch, goarm string) (*ReleaseAsset, error) {
	var armSuffix string
	if goarm != "" {
		armSuffix = "v" + goarm
	}

	targetExt := ".tar.gz"
	if goos == "windows" {
		targetExt = ".zip"
	}

	for _, asset := range assets {
		name := strings.ToLower(asset.Name)
		if strings.HasSuffix(name, targetExt) {
			if strings.Contains(name, "_"+goos+"_") || strings.Contains(name, "-"+goos+"-") {
				if armSuffix != "" {
					if strings.Contains(name, "_"+goarch+armSuffix) || strings.Contains(name, "-"+goarch+armSuffix) {
						return &asset, nil
					}
				} else {
					if strings.Contains(name, "_"+goarch) || strings.Contains(name, "-"+goarch) {
						if !strings.Contains(name, goarch+"v") {
							return &asset, nil
						}
					}
				}
			}
		}
	}

	binSuffix := ""
	if goos == "windows" {
		binSuffix = ".exe"
	}
	exactBinName := fmt.Sprintf("dogu-%s-%s%s%s", goos, goarch, armSuffix, binSuffix)
	for _, asset := range assets {
		if strings.EqualFold(asset.Name, exactBinName) {
			return &asset, nil
		}
	}

	return nil, fmt.Errorf("no matching release asset found for os=%s arch=%s arm=%s", goos, goarch, goarm)
}

// SelfUpdate updates the running dogu binary to the latest available GitHub release.
func SelfUpdate(ctx context.Context, currentVersion, owner, repo string) error {
	if !version.IsBinaryDistribution() {
		return fmt.Errorf("self-update is only available for official binary distributions, not for development builds or 'go run'")
	}

	if owner == "" {
		owner = DefaultGitHubOwner
	}
	if repo == "" {
		repo = DefaultGitHubRepo
	}

	rel, updateAvailable, err := CheckForUpdate(ctx, currentVersion, owner, repo)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	latestVer := rel.TagName
	if latestVer == "" {
		latestVer = rel.Name
	}

	if !updateAvailable {
		fmt.Printf("No newer release with a compatible binary is available (%s).\n", currentVersion)
		return nil
	}

	fmt.Printf("Updating dogu from %s to %s...\n", currentVersion, latestVer)

	goarm := os.Getenv("GOARM")
	asset, err := FindMatchingAsset(rel.Assets, runtime.GOOS, runtime.GOARCH, goarm)
	if err != nil {
		return fmt.Errorf("could not find compatible binary in release %s: %w", latestVer, err)
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine executable path: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = resolved
	}

	if err := updateExecutable(ctx, rel, asset, exePath); err != nil {
		return err
	}

	fmt.Printf("Successfully updated dogu to %s!\n", latestVer)
	return nil
}

func updateExecutable(ctx context.Context, rel *Release, asset *ReleaseAsset, exePath string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	expectedHash, err := releaseChecksum(ctx, client, rel, asset.Name)
	if err != nil {
		return err
	}

	destDir := filepath.Dir(exePath)
	download, err := os.CreateTemp("", "dogu-download-*")
	if err != nil {
		systemTempErr := err
		download, err = os.CreateTemp(destDir, "dogu-download-*")
		if err != nil {
			return fmt.Errorf("failed to create temporary download in system temp directory (%v) or executable directory %s: %w", systemTempErr, destDir, err)
		}
	}
	defer os.Remove(download.Name())
	defer download.Close()

	req, err := http.NewRequestWithContext(ctx, "GET", asset.DownloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download release asset: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download asset failed with HTTP %s", resp.Status)
	}

	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(download, hash), resp.Body)
	if err != nil {
		return fmt.Errorf("failed to download release asset: %w", err)
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), expectedHash) {
		return fmt.Errorf("SHA-256 checksum verification failed for %s", asset.Name)
	}
	if _, err := download.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to rewind release asset: %w", err)
	}

	tempFile, err := os.CreateTemp(destDir, "dogu-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file in %s: %w", destDir, err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	defer tempFile.Close()

	lowerName := strings.ToLower(asset.Name)
	if strings.HasSuffix(lowerName, ".tar.gz") || strings.HasSuffix(lowerName, ".tgz") {
		if err := extractBinaryFromTarGz(download, tempFile); err != nil {
			return fmt.Errorf("failed to extract binary from tar.gz: %w", err)
		}
	} else if strings.HasSuffix(lowerName, ".zip") {
		if err := extractBinaryFromZip(download, size, tempFile); err != nil {
			return fmt.Errorf("failed to extract binary from zip: %w", err)
		}
	} else {
		if _, err := io.Copy(tempFile, download); err != nil {
			return fmt.Errorf("failed to write binary content: %w", err)
		}
	}

	if err := tempFile.Chmod(0755); err != nil {
		log.Debug("chmod on updated temp binary failed", "err", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close updated binary: %w", err)
	}

	if err := replaceExecutable(tempPath, exePath); err != nil {
		return fmt.Errorf("failed to replace executable %s: %w", exePath, err)
	}

	return nil
}

func releaseChecksum(ctx context.Context, client *http.Client, rel *Release, assetName string) (string, error) {
	var checksumURL string
	for _, asset := range rel.Assets {
		if asset.Name == "checksums.txt" {
			checksumURL = asset.DownloadURL
			break
		}
	}
	if checksumURL == "" {
		return "", fmt.Errorf("release does not include checksums.txt")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", checksumURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create checksums request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download checksums.txt: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download checksums.txt failed with HTTP %s", resp.Status)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != assetName {
			continue
		}
		digest, err := hex.DecodeString(fields[0])
		if err != nil || len(digest) != sha256.Size {
			return "", fmt.Errorf("no valid checksum found for %s", assetName)
		}
		return fields[0], nil
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read checksums.txt: %w", err)
	}
	return "", fmt.Errorf("no valid checksum found for %s", assetName)
}

func extractBinaryFromTarGz(r io.Reader, targetWriter io.Writer) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		base := filepath.Base(header.Name)
		if base == "dogu" || base == "dogu.exe" {
			_, err := io.Copy(targetWriter, tr)
			return err
		}
	}

	return fmt.Errorf("dogu binary not found inside archive")
}

func extractBinaryFromZip(ra io.ReaderAt, size int64, targetWriter io.Writer) error {
	zr, err := zip.NewReader(ra, size)
	if err != nil {
		return err
	}

	for _, f := range zr.File {
		base := filepath.Base(f.Name)
		if base == "dogu" || base == "dogu.exe" {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()
			_, err = io.Copy(targetWriter, rc)
			return err
		}
	}

	return fmt.Errorf("dogu binary not found inside zip archive")
}

func replaceExecutable(tempPath, exePath string) error {
	if runtime.GOOS == "windows" {
		oldExePath := exePath + ".old"
		_ = os.Remove(oldExePath)
		if err := os.Rename(exePath, oldExePath); err != nil {
			return err
		}
		if err := os.Rename(tempPath, exePath); err != nil {
			_ = os.Rename(oldExePath, exePath)
			return err
		}
		_ = os.Remove(oldExePath)
		return nil
	}

	return os.Rename(tempPath, exePath)
}

func hasCompatibleAsset(rel *Release) bool {
	asset, err := FindMatchingAsset(rel.Assets, runtime.GOOS, runtime.GOARCH, os.Getenv("GOARM"))
	if err != nil || asset.DownloadURL == "" {
		return false
	}
	for _, releaseAsset := range rel.Assets {
		if releaseAsset.Name == "checksums.txt" && releaseAsset.DownloadURL != "" {
			return true
		}
	}
	return false
}
