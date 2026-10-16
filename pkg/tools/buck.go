package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dogu-dev/dogu/pkg/downloader"
	"github.com/dogu-dev/dogu/pkg/env"
)

type BuckTool struct{}

func init() {
	Register(&BuckTool{})
}

func (b *BuckTool) Name() string {
	return "buck"
}

func (b *BuckTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, force bool) (*InstallResult, error) {
	if !force {
		if existingDir, ok := findExistingCandidate(doguHome, "buck", versionSpec); ok {
			return buildResultFromDir("buck", versionSpec, existingDir), nil
		}
	}

	baseURL := "https://github.com/facebook/buck/releases/download"
	if mirrorBaseURL != "" {
		baseURL = strings.TrimSuffix(mirrorBaseURL, "/")
	}

	normVer := resolveBuckVersion(ctx, versionSpec)

	filename := fmt.Sprintf("buck-%s.yosemite.bottle.tar.gz", normVer)
	downloadURL := fmt.Sprintf("%s/v%s/%s", baseURL, normVer, filename)

	installDir := filepath.Join(doguHome, "candidates", "buck", versionSpec)
	cacheFile := filepath.Join(doguHome, "cache", "buck", filename)

	if err := downloader.DownloadAndExtract(ctx, downloadURL, cacheFile, installDir); err != nil {
		return nil, fmt.Errorf("failed to download buck: %w", err)
	}

	buckHome := downloader.FindSubDirWithDir(installDir, "bin")
	binDir := filepath.Join(buckHome, "bin")

	majorVer := env.ExtractMajorVersion(normVer)
	envVars := map[string]string{
		env.FormatEnvKey("buck", majorVer): buckHome,
		env.FormatEnvKey("buck", ""):        buckHome,
		"BUCK_HOME":                        buckHome,
	}

	return &InstallResult{
		ToolName:      "buck",
		RequestedVer:  versionSpec,
		NormalizedVer: normVer,
		InstallDir:    installDir,
		HomeDir:       buckHome,
		BinDirs:       []string{binDir},
		EnvVars:       envVars,
	}, nil
}

func resolveBuckVersion(ctx context.Context, versionSpec string) string {
	if versions, err := FetchGitHubReleases(ctx, "facebook", "buck"); err == nil && len(versions) > 0 {
		if best, err := SelectBestVersion(versions, versionSpec); err == nil {
			return CleanVersion(best)
		}
	}
	return CleanVersion(versionSpec)
}
