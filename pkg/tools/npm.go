package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dogu-dev/dogu/pkg/downloader"
	"github.com/dogu-dev/dogu/pkg/env"
)

type NPMTool struct{}

func init() {
	Register(&NPMTool{})
}

func (n *NPMTool) Name() string {
	return "npm"
}

func (n *NPMTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, force bool) (*InstallResult, error) {
	if !force {
		if existingDir, ok := findExistingCandidate(doguHome, "npm", versionSpec); ok {
			return buildResultFromDir("npm", versionSpec, existingDir), nil
		}
	}

	normVer := strings.TrimPrefix(versionSpec, "v")

	baseURL := "https://registry.npmjs.org/npm/-"
	if mirrorBaseURL != "" {
		baseURL = strings.TrimSuffix(mirrorBaseURL, "/")
	}

	filename := fmt.Sprintf("npm-%s.tgz", normVer)
	downloadURL := fmt.Sprintf("%s/%s", baseURL, filename)

	installDir := filepath.Join(doguHome, "candidates", "npm", versionSpec)
	cacheFile := filepath.Join(doguHome, "cache", "npm", filename)

	if err := downloader.DownloadAndExtract(ctx, downloadURL, cacheFile, installDir); err != nil {
		return nil, fmt.Errorf("failed to download npm: %w", err)
	}

	npmHome := downloader.FindSubDirWithDir(installDir, "bin")
	binDir := filepath.Join(npmHome, "bin")

	majorVer := env.ExtractMajorVersion(normVer)
	envVars := map[string]string{
		env.FormatEnvKey("npm", majorVer): npmHome,
		env.FormatEnvKey("npm", ""):        npmHome,
	}

	return &InstallResult{
		ToolName:      "npm",
		RequestedVer:  versionSpec,
		NormalizedVer: normVer,
		InstallDir:    installDir,
		HomeDir:       npmHome,
		BinDirs:       []string{binDir},
		EnvVars:       envVars,
	}, nil
}
