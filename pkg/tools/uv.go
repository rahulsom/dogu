package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dogu-dev/dogu/pkg/downloader"
	"github.com/dogu-dev/dogu/pkg/env"
)

type UVTool struct{}

func init() {
	Register(&UVTool{})
}

func (u *UVTool) Name() string {
	return "uv"
}

func (u *UVTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, force bool) (*InstallResult, error) {
	if !force {
		if existingDir, ok := findExistingCandidate(doguHome, "uv", versionSpec); ok {
			return buildResultFromDir("uv", versionSpec, existingDir), nil
		}
	}

	normVer := strings.TrimPrefix(versionSpec, "v")

	baseURL := "https://github.com/astral-sh/uv/releases/download"
	if mirrorBaseURL != "" {
		baseURL = strings.TrimSuffix(mirrorBaseURL, "/")
	}

	target := mapUVTarget()
	filename := fmt.Sprintf("uv-%s.tar.gz", target)
	if runtime.GOOS == "windows" {
		filename = fmt.Sprintf("uv-%s.zip", target)
	}

	downloadURL := fmt.Sprintf("%s/%s/%s", baseURL, normVer, filename)

	installDir := filepath.Join(doguHome, "candidates", "uv", versionSpec)
	cacheFile := filepath.Join(doguHome, "cache", "uv", filename)

	if err := downloader.DownloadAndExtract(ctx, downloadURL, cacheFile, installDir); err != nil {
		return nil, fmt.Errorf("failed to download uv: %w", err)
	}

	uvHome := downloader.FindSubDirWithDir(installDir, "uv")
	if fi, err := filepath.Glob(filepath.Join(installDir, "*", "uv")); err == nil && len(fi) > 0 {
		uvHome = filepath.Dir(fi[0])
	}

	majorVer := env.ExtractMajorVersion(normVer)
	envVars := map[string]string{
		env.FormatEnvKey("uv", majorVer): uvHome,
		env.FormatEnvKey("uv", ""):        uvHome,
	}

	return &InstallResult{
		ToolName:      "uv",
		RequestedVer:  versionSpec,
		NormalizedVer: normVer,
		InstallDir:    installDir,
		HomeDir:       uvHome,
		BinDirs:       []string{uvHome},
		EnvVars:       envVars,
	}, nil
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
