package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dogu-dev/dogu/pkg/downloader"
	"github.com/dogu-dev/dogu/pkg/env"
)

type AntTool struct{}

func init() {
	Register(&AntTool{})
}

func (a *AntTool) Name() string {
	return "ant"
}

func (a *AntTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, force bool) (*InstallResult, error) {
	if !force {
		if existingDir, ok := findExistingCandidate(doguHome, "ant", versionSpec); ok {
			return buildResultFromDir("ant", versionSpec, existingDir), nil
		}
	}

	baseURL := "https://archive.apache.org/dist/ant/binaries"
	if mirrorBaseURL != "" {
		baseURL = strings.TrimSuffix(mirrorBaseURL, "/")
	}

	normVer := resolveAntVersion(ctx, versionSpec, baseURL)

	filename := fmt.Sprintf("apache-ant-%s-bin.tar.gz", normVer)
	downloadURL := fmt.Sprintf("%s/%s", baseURL, filename)

	installDir := filepath.Join(doguHome, "candidates", "ant", versionSpec)
	cacheFile := filepath.Join(doguHome, "cache", "ant", filename)

	if err := downloader.DownloadAndExtract(ctx, downloadURL, cacheFile, installDir); err != nil {
		return nil, fmt.Errorf("failed to download ant: %w", err)
	}

	antHome := downloader.FindSubDirWithDir(installDir, "bin")
	binDir := filepath.Join(antHome, "bin")

	majorVer := env.ExtractMajorVersion(normVer)
	envVars := map[string]string{
		env.FormatEnvKey("ant", majorVer): antHome,
		env.FormatEnvKey("ant", ""):        antHome,
		"ANT_HOME":                        antHome,
	}

	return &InstallResult{
		ToolName:      "ant",
		RequestedVer:  versionSpec,
		NormalizedVer: normVer,
		InstallDir:    installDir,
		HomeDir:       antHome,
		BinDirs:       []string{binDir},
		EnvVars:       envVars,
	}, nil
}

func resolveAntVersion(ctx context.Context, versionSpec string, baseURL string) string {
	if versions, err := FetchApacheAntVersions(ctx, baseURL); err == nil && len(versions) > 0 {
		if best, err := SelectBestVersion(versions, versionSpec); err == nil {
			return CleanVersion(best)
		}
	}
	return CleanVersion(versionSpec)
}
