package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dogu-dev/dogu/pkg/downloader"
	"github.com/dogu-dev/dogu/pkg/env"
)

type YarnTool struct{}

func init() {
	Register(&YarnTool{})
}

func (y *YarnTool) Name() string {
	return "yarn"
}

func (y *YarnTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, force bool) (*InstallResult, error) {
	if !force {
		if existingDir, ok := findExistingCandidate(doguHome, "yarn", versionSpec); ok {
			return buildResultFromDir("yarn", versionSpec, existingDir), nil
		}
	}

	normVer := strings.TrimPrefix(versionSpec, "v")

	baseURL := "https://github.com/yarnpkg/yarn/releases/download"
	if mirrorBaseURL != "" {
		baseURL = strings.TrimSuffix(mirrorBaseURL, "/")
	}

	filename := fmt.Sprintf("yarn-v%s.tar.gz", normVer)
	downloadURL := fmt.Sprintf("%s/v%s/%s", baseURL, normVer, filename)

	installDir := filepath.Join(doguHome, "candidates", "yarn", versionSpec)
	cacheFile := filepath.Join(doguHome, "cache", "yarn", filename)

	if err := downloader.DownloadAndExtract(ctx, downloadURL, cacheFile, installDir); err != nil {
		return nil, fmt.Errorf("failed to download yarn: %w", err)
	}

	yarnHome := downloader.FindSubDirWithDir(installDir, "bin")
	binDir := filepath.Join(yarnHome, "bin")

	majorVer := env.ExtractMajorVersion(normVer)
	envVars := map[string]string{
		env.FormatEnvKey("yarn", majorVer): yarnHome,
		env.FormatEnvKey("yarn", ""):        yarnHome,
	}

	return &InstallResult{
		ToolName:      "yarn",
		RequestedVer:  versionSpec,
		NormalizedVer: normVer,
		InstallDir:    installDir,
		HomeDir:       yarnHome,
		BinDirs:       []string{binDir},
		EnvVars:       envVars,
	}, nil
}
