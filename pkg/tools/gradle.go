package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dogu-dev/dogu/pkg/downloader"
	"github.com/dogu-dev/dogu/pkg/env"
)

type GradleTool struct{}

func init() {
	Register(&GradleTool{})
}

func (g *GradleTool) Name() string {
	return "gradle"
}

func (g *GradleTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, force bool) (*InstallResult, error) {
	if !force {
		if existingDir, ok := findExistingCandidate(doguHome, "gradle", versionSpec); ok {
			return buildResultFromDir("gradle", versionSpec, existingDir), nil
		}
	}

	baseURL := "https://services.gradle.org/distributions"
	if mirrorBaseURL != "" {
		baseURL = strings.TrimSuffix(mirrorBaseURL, "/")
	}

	normVer := resolveGradleVersion(ctx, versionSpec, mirrorBaseURL)

	filename := fmt.Sprintf("gradle-%s-bin.zip", normVer)
	downloadURL := fmt.Sprintf("%s/%s", baseURL, filename)

	installDir := filepath.Join(doguHome, "candidates", "gradle", versionSpec)
	cacheFile := filepath.Join(doguHome, "cache", "gradle", filename)

	if err := downloader.DownloadAndExtract(ctx, downloadURL, cacheFile, installDir); err != nil {
		return nil, fmt.Errorf("failed to download gradle: %w", err)
	}

	gradleHome := downloader.FindSubDirWithDir(installDir, "bin")
	binDir := filepath.Join(gradleHome, "bin")

	majorVer := env.ExtractMajorVersion(normVer)
	envVars := map[string]string{
		env.FormatEnvKey("gradle", majorVer): gradleHome,
		env.FormatEnvKey("gradle", ""):        gradleHome,
		"GRADLE_HOME":                        gradleHome,
	}

	return &InstallResult{
		ToolName:      "gradle",
		RequestedVer:  versionSpec,
		NormalizedVer: normVer,
		InstallDir:    installDir,
		HomeDir:       gradleHome,
		BinDirs:       []string{binDir},
		EnvVars:       envVars,
	}, nil
}

func resolveGradleVersion(ctx context.Context, versionSpec string, mirrorBaseURL string) string {
	if versions, err := FetchGradleVersions(ctx, mirrorBaseURL); err == nil && len(versions) > 0 {
		if best, err := SelectBestVersion(versions, versionSpec); err == nil {
			return CleanVersion(best)
		}
	}
	return CleanVersion(versionSpec)
}
