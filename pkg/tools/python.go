package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/dogu-dev/dogu/pkg/downloader"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

type PythonTool struct{}

func init() {
	Register(&PythonTool{})
}

func (p *PythonTool) Name() string {
	return "python"
}

func (p *PythonTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	postPython := func(res *InstallResult) error {
		res.EnvVars["PYTHONHOME"] = res.HomeDir
		for _, b := range res.BinDirs {
			ensurePythonSymlink(b)
		}
		return nil
	}

	if res, ok, err := checkExistingCandidate(doguHome, "python", versionSpec, opts, postPython); ok || err != nil {
		return res, err
	}

	baseURL := resolveBaseURL("https://github.com/indygreg/python-build-standalone/releases/download", mirrorBaseURL)
	normVer, tag, filename, err := resolvePythonAsset(ctx, versionSpec, opts.IncludePrerelease)
	if err != nil {
		return nil, err
	}
	downloadURL := fmt.Sprintf("%s/%s/%s", baseURL, tag, filename)

	return installArchiveTool(ctx, ArchiveInstallConfig{
		ToolName:      "python",
		VersionSpec:   versionSpec,
		NormalizedVer: normVer,
		DownloadURL:   downloadURL,
		FileName:      filename,
		DoguHome:      doguHome,
		Opts:          opts,
		ExtraHomeEnv:  []string{"PYTHONHOME"},
		FindDirs: func(installDir string) (string, []string) {
			pythonHome := downloader.FindSubDirWithDir(installDir, "bin")
			binDir := filepath.Join(pythonHome, "bin")
			if runtime.GOOS == "windows" {
				binDir = pythonHome
			}
			return pythonHome, []string{binDir}
		},
		PostInstall: postPython,
	})
}

type pythonRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
	} `json:"assets"`
}

func fetchPythonReleases(ctx context.Context) ([]pythonRelease, error) {
	data, err := fetchReleaseData(ctx, "https://api.github.com/repos/indygreg/python-build-standalone/releases?per_page=100")
	if err != nil {
		return nil, err
	}
	var releases []pythonRelease
	if err := json.Unmarshal(data, &releases); err != nil {
		return nil, err
	}
	return releases, nil
}

var pythonVersionPattern = regexp.MustCompile(`^cpython-([0-9]+\.[0-9]+\.[0-9]+(?:[abrc]+[0-9]+)?)[+-]`)

func pythonAssetVersion(name string) string {
	if match := pythonVersionPattern.FindStringSubmatch(name); len(match) > 1 {
		return match[1]
	}
	return ""
}

func pythonAssetMatchesTarget(name string) bool {
	target := mapPythonTarget()
	if strings.HasSuffix(name, "-"+target+"-install_only.tar.gz") {
		return true
	}
	return runtime.GOOS == "windows" &&
		strings.HasSuffix(name, "-"+strings.TrimSuffix(target, "-shared")+"-install_only.tar.gz")
}

func resolvePythonAsset(ctx context.Context, spec string, prerelease bool) (version, tag, filename string, err error) {
	releases, err := fetchPythonReleases(ctx)
	if err != nil {
		if spec == "" {
			return "", "", "", fmt.Errorf("resolve latest Python version: %w", err)
		}
		return legacyPythonAsset(spec, prerelease)
	}
	var versions []string
	for _, release := range releases {
		for _, asset := range release.Assets {
			if pythonAssetMatchesTarget(asset.Name) {
				if v := pythonAssetVersion(asset.Name); v != "" {
					versions = append(versions, v)
				}
			}
		}
	}
	version, err = SelectBestVersion(versions, spec, prerelease)
	if err != nil {
		if spec != "" {
			return legacyPythonAsset(spec, prerelease)
		}
		return "", "", "", err
	}
	for _, release := range releases {
		for _, asset := range release.Assets {
			if pythonAssetVersion(asset.Name) == version && pythonAssetMatchesTarget(asset.Name) &&
				(filename == "" || CompareVersions(release.TagName, tag) > 0) {
				tag, filename = release.TagName, asset.Name
			}
		}
	}
	return version, tag, filename, nil
}

func legacyPythonAsset(spec string, prerelease bool) (version, tag, filename string, err error) {
	version, err = resolveDiscoveredVersion([]string{"3.8.18", "3.9.18", "3.10.13", "3.11.7", "3.12.1"}, nil, spec, prerelease)
	return version, "20240107", fmt.Sprintf("cpython-%s+20240107-%s-install_only.tar.gz", version, mapPythonTarget()), err
}

func ensurePythonSymlink(binDir string) {
	if runtime.GOOS == "windows" {
		target := filepath.Join(binDir, "python.exe")
		if _, err := os.Stat(target); os.IsNotExist(err) {
			py3 := filepath.Join(binDir, "python3.exe")
			if fi, err := os.Stat(py3); err == nil && !fi.IsDir() {
				data, err := os.ReadFile(py3)
				if err == nil {
					_ = os.WriteFile(target, data, 0755)
				}
			}
		}
	} else {
		target := filepath.Join(binDir, "python")
		if _, err := os.Stat(target); os.IsNotExist(err) {
			py3 := filepath.Join(binDir, "python3")
			if fi, err := os.Stat(py3); err == nil && !fi.IsDir() {
				_ = os.Symlink("python3", target)
			}
		}
	}
}

func mapPythonTarget() string {
	arch := "x86_64"
	if runtime.GOARCH == "arm64" {
		arch = "aarch64"
	}

	switch runtime.GOOS {
	case "darwin":
		return arch + "-apple-darwin"
	case "windows":
		return arch + "-pc-windows-msvc-shared"
	default:
		return arch + "-unknown-linux-gnu"
	}
}

type UVTool struct{}

func init() {
	Register(&UVTool{})
}

func (u *UVTool) Name() string {
	return "uv"
}

func (u *UVTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if res, ok, err := checkExistingCandidate(doguHome, "uv", versionSpec, opts, nil); ok || err != nil {
		return res, err
	}

	baseURL := resolveBaseURL("https://github.com/astral-sh/uv/releases/download", mirrorBaseURL)
	normVer, err := resolveUVVersion(ctx, versionSpec, opts.IncludePrerelease)
	if err != nil {
		return nil, err
	}

	target := mapUVTarget()
	filename := fmt.Sprintf("uv-%s.tar.gz", target)
	if runtime.GOOS == "windows" {
		filename = fmt.Sprintf("uv-%s.zip", target)
	}

	downloadURL := fmt.Sprintf("%s/%s/%s", baseURL, normVer, filename)

	return installArchiveTool(ctx, ArchiveInstallConfig{
		ToolName:      "uv",
		VersionSpec:   versionSpec,
		NormalizedVer: normVer,
		DownloadURL:   downloadURL,
		FileName:      filename,
		DoguHome:      doguHome,
		Opts:          opts,
		FindDirs: func(installDir string) (string, []string) {
			binary := "uv"
			if runtime.GOOS == "windows" {
				binary += ".exe"
			}
			return findDirWithBinary(installDir, binary)
		},
	})
}

func resolveUVVersion(ctx context.Context, versionSpec string, includePrerelease bool) (string, error) {
	if versions, err := FetchPyPIVersions(ctx, "uv"); err == nil && len(versions) > 0 {
		if best, err := SelectBestVersion(versions, versionSpec, includePrerelease); err == nil {
			return CleanVersion(best), nil
		}
	}
	versions, err := FetchGitHubReleases(ctx, "astral-sh", "uv")
	return resolveDiscoveredVersion(versions, err, versionSpec, includePrerelease)
}

func mapUVTarget() string {
	arch := "x86_64"
	if runtime.GOARCH == "arm64" {
		arch = "aarch64"
	}

	switch runtime.GOOS {
	case "darwin":
		return arch + "-apple-darwin"
	case "windows":
		return arch + "-pc-windows-msvc"
	default:
		return arch + "-unknown-linux-gnu"
	}
}
