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

type BunTool struct{}

func init() {
	Register(&BunTool{})
}

func (b *BunTool) Name() string {
	return "bun"
}

func (b *BunTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, force bool) (*InstallResult, error) {
	if !force {
		if existingDir, ok := findExistingCandidate(doguHome, "bun", versionSpec); ok {
			return buildResultFromDir("bun", versionSpec, existingDir), nil
		}
	}

	normVer := versionSpec
	if !strings.HasPrefix(normVer, "bun-v") && !strings.HasPrefix(normVer, "v") {
		normVer = "v" + normVer
	}
	tag := strings.TrimPrefix(normVer, "bun-")

	baseURL := "https://github.com/oven-sh/bun/releases/download"
	if mirrorBaseURL != "" {
		baseURL = strings.TrimSuffix(mirrorBaseURL, "/")
	}

	targetOS, targetArch := mapBunTarget()
	filename := fmt.Sprintf("bun-%s-%s.zip", targetOS, targetArch)
	downloadURL := fmt.Sprintf("%s/%s/%s", baseURL, tag, filename)

	installDir := filepath.Join(doguHome, "candidates", "bun", versionSpec)
	cacheFile := filepath.Join(doguHome, "cache", "bun", filename)

	if err := downloader.DownloadAndExtract(ctx, downloadURL, cacheFile, installDir); err != nil {
		return nil, fmt.Errorf("failed to download bun: %w", err)
	}

	bunHome := downloader.FindSubDirWithDir(installDir, "bun")
	if fi, err := filepath.Glob(filepath.Join(installDir, "*", "bun")); err == nil && len(fi) > 0 {
		bunHome = filepath.Dir(fi[0])
	}

	majorVer := env.ExtractMajorVersion(normVer)
	envVars := map[string]string{
		env.FormatEnvKey("bun", majorVer): bunHome,
		env.FormatEnvKey("bun", ""):        bunHome,
	}

	return &InstallResult{
		ToolName:      "bun",
		RequestedVer:  versionSpec,
		NormalizedVer: normVer,
		InstallDir:    installDir,
		HomeDir:       bunHome,
		BinDirs:       []string{bunHome},
		EnvVars:       envVars,
	}, nil
}

func mapBunTarget() (string, string) {
	osName := "linux"
	switch runtime.GOOS {
	case "darwin":
		osName = "darwin"
	case "windows":
		osName = "windows"
	}

	archName := "x64"
	if runtime.GOARCH == "arm64" {
		archName = "aarch64"
	}

	return osName, archName
}
