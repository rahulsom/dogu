package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dogu-dev/dogu/pkg/downloader"
	"github.com/dogu-dev/dogu/pkg/env"
)

type PythonTool struct{}

func init() {
	Register(&PythonTool{})
}

func (p *PythonTool) Name() string {
	return "python"
}

func (p *PythonTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, force bool) (*InstallResult, error) {
	if !force {
		if existingDir, ok := findExistingCandidate(doguHome, "python", versionSpec); ok {
			res := buildResultFromDir("python", versionSpec, existingDir)
			for _, b := range res.BinDirs {
				ensurePythonSymlink(b)
			}
			return res, nil
		}
	}

	normVer := strings.TrimPrefix(versionSpec, "v")

	baseURL := "https://github.com/indygreg/python-build-standalone/releases/download"
	if mirrorBaseURL != "" {
		baseURL = strings.TrimSuffix(mirrorBaseURL, "/")
	}

	target := mapPythonTarget()
	filename := fmt.Sprintf("cpython-%s+%s-%s-install_only.tar.gz", normVer, "20240107", target)
	if mirrorBaseURL != "" {
		filename = fmt.Sprintf("python-%s-%s.tar.gz", normVer, target)
	}

	downloadURL := fmt.Sprintf("%s/20240107/%s", baseURL, filename)
	if mirrorBaseURL != "" {
		downloadURL = fmt.Sprintf("%s/%s", baseURL, filename)
	}

	installDir := filepath.Join(doguHome, "candidates", "python", versionSpec)
	cacheFile := filepath.Join(doguHome, "cache", "python", filename)

	if err := downloader.DownloadAndExtract(ctx, downloadURL, cacheFile, installDir); err != nil {
		return nil, fmt.Errorf("failed to download python: %w", err)
	}

	pythonHome := downloader.FindSubDirWithDir(installDir, "bin")
	binDir := filepath.Join(pythonHome, "bin")
	if runtime.GOOS == "windows" {
		binDir = pythonHome
	}
	ensurePythonSymlink(binDir)

	majorVer := env.ExtractMajorVersion(normVer)
	envVars := map[string]string{
		env.FormatEnvKey("python", majorVer): pythonHome,
		env.FormatEnvKey("python", ""):        pythonHome,
		"PYTHONHOME":                        pythonHome,
	}

	return &InstallResult{
		ToolName:      "python",
		RequestedVer:  versionSpec,
		NormalizedVer: normVer,
		InstallDir:    installDir,
		HomeDir:       pythonHome,
		BinDirs:       []string{binDir},
		EnvVars:       envVars,
	}, nil
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
