package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dogu-dev/dogu/pkg/downloader"
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
		for _, b := range res.BinDirs {
			ensurePythonSymlink(b)
		}
		return nil
	}

	if res, ok, err := checkExistingCandidate(doguHome, "python", versionSpec, opts, postPython); ok || err != nil {
		return res, err
	}

	baseURL := resolveBaseURL("https://github.com/indygreg/python-build-standalone/releases/download", mirrorBaseURL)
	normVer := resolvePythonVersion(ctx, versionSpec, opts.IncludePrerelease)

	target := mapPythonTarget()
	filename := fmt.Sprintf("cpython-%s+%s-%s-install_only.tar.gz", normVer, "20240107", target)
	downloadURL := fmt.Sprintf("%s/20240107/%s", baseURL, filename)

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

func resolvePythonVersion(ctx context.Context, versionSpec string, includePrerelease bool) string {
	knownVersions := []string{"3.8.18", "3.9.18", "3.10.13", "3.11.7", "3.12.1"}
	if versions, err := FetchGitHubReleases(ctx, "indygreg", "python-build-standalone"); err == nil && len(versions) > 0 {
		var extracted []string
		for _, v := range versions {
			if strings.HasPrefix(v, "cpython-") {
				parts := strings.Split(v, "+")
				extracted = append(extracted, CleanVersion(parts[0]))
			} else {
				extracted = append(extracted, CleanVersion(v))
			}
		}
		if len(extracted) > 0 {
			knownVersions = append(knownVersions, extracted...)
		}
	}

	if best, err := SelectBestVersion(knownVersions, versionSpec, includePrerelease); err == nil {
		return CleanVersion(best)
	}
	return CleanVersion(versionSpec)
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
