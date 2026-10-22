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

type MillTool struct{}

func init() {
	Register(&MillTool{})
}

func (m *MillTool) Name() string {
	return "mill"
}

func (m *MillTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if !opts.Force {
		if existingDir, ok := findExistingCandidate(doguHome, "mill", versionSpec, opts.IncludePrerelease); ok {
			return buildResultFromDir("mill", versionSpec, existingDir), nil
		}
	}

	baseURL := "https://github.com/com-lihaoyi/mill/releases/download"
	if mirrorBaseURL != "" {
		baseURL = strings.TrimSuffix(mirrorBaseURL, "/")
	}

	normVer := resolveMillVersion(ctx, versionSpec, opts.IncludePrerelease)

	assetName := normVer
	binaryName := "mill"
	if runtime.GOOS == "windows" {
		assetName = normVer + ".bat"
		binaryName = "mill.bat"
	}

	downloadURL := fmt.Sprintf("%s/%s/%s", baseURL, normVer, assetName)

	installDir := filepath.Join(doguHome, "candidates", "mill", versionSpec)
	cacheFile := filepath.Join(doguHome, "cache", "mill", fmt.Sprintf("mill-%s-%s", normVer, binaryName))

	if err := downloader.SaveSingleBinary(ctx, downloadURL, cacheFile, installDir, binaryName); err != nil {
		return nil, fmt.Errorf("failed to download mill: %w", err)
	}

	majorVer := env.ExtractMajorVersion(normVer)
	envVars := map[string]string{
		env.FormatEnvKey("mill", majorVer): installDir,
		env.FormatEnvKey("mill", ""):        installDir,
		"MILL_HOME":                         installDir,
		"MILL_VERSION":                      normVer,
	}

	return &InstallResult{
		ToolName:      "mill",
		RequestedVer:  versionSpec,
		NormalizedVer: normVer,
		InstallDir:    installDir,
		HomeDir:       installDir,
		BinDirs:       []string{installDir},
		EnvVars:       envVars,
	}, nil
}

func resolveMillVersion(ctx context.Context, versionSpec string, includePrerelease bool) string {
	if versions, err := FetchGitHubReleases(ctx, "com-lihaoyi", "mill"); err == nil && len(versions) > 0 {
		if best, err := SelectBestVersion(versions, versionSpec, includePrerelease); err == nil {
			return CleanVersion(best)
		}
	}
	return CleanVersion(versionSpec)
}
