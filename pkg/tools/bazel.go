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

type BazelTool struct{}

func init() {
	Register(&BazelTool{})
}

func (b *BazelTool) Name() string {
	return "bazel"
}

func (b *BazelTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, force bool) (*InstallResult, error) {
	if !force {
		if existingDir, ok := findExistingCandidate(doguHome, "bazel", versionSpec); ok {
			return buildResultFromDir("bazel", versionSpec, existingDir), nil
		}
	}

	baseURL := "https://github.com/bazelbuild/bazel/releases/download"
	if mirrorBaseURL != "" {
		baseURL = strings.TrimSuffix(mirrorBaseURL, "/")
	}

	normVer := resolveBazelVersion(ctx, versionSpec)

	osName, archName := mapBazelTarget()
	binName := fmt.Sprintf("bazel-%s-%s-%s", normVer, osName, archName)
	targetBin := "bazel"
	if osName == "windows" {
		binName += ".exe"
		targetBin = "bazel.exe"
	}

	downloadURL := fmt.Sprintf("%s/%s/%s", baseURL, normVer, binName)

	installDir := filepath.Join(doguHome, "candidates", "bazel", versionSpec)
	cacheFile := filepath.Join(doguHome, "cache", "bazel", binName)

	if err := downloader.SaveSingleBinary(ctx, downloadURL, cacheFile, installDir, targetBin); err != nil {
		return nil, fmt.Errorf("failed to download bazel: %w", err)
	}

	majorVer := env.ExtractMajorVersion(normVer)
	envVars := map[string]string{
		env.FormatEnvKey("bazel", majorVer): installDir,
		env.FormatEnvKey("bazel", ""):        installDir,
		"BAZEL_HOME":                        installDir,
	}

	return &InstallResult{
		ToolName:      "bazel",
		RequestedVer:  versionSpec,
		NormalizedVer: normVer,
		InstallDir:    installDir,
		HomeDir:       installDir,
		BinDirs:       []string{installDir},
		EnvVars:       envVars,
	}, nil
}

func resolveBazelVersion(ctx context.Context, versionSpec string) string {
	if versions, err := FetchGitHubReleases(ctx, "bazelbuild", "bazel"); err == nil && len(versions) > 0 {
		if best, err := SelectBestVersion(versions, versionSpec); err == nil {
			return CleanVersion(best)
		}
	}
	return CleanVersion(versionSpec)
}

func mapBazelTarget() (string, string) {
	osName := "linux"
	switch runtime.GOOS {
	case "darwin":
		osName = "darwin"
	case "windows":
		osName = "windows"
	}

	archName := "x86_64"
	if runtime.GOARCH == "arm64" {
		archName = "arm64"
	}

	return osName, archName
}
