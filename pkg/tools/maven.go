package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dogu-dev/dogu/pkg/downloader"
	"github.com/dogu-dev/dogu/pkg/env"
)

type MavenTool struct{}

func init() {
	Register(&MavenTool{})
}

func (m *MavenTool) Name() string {
	return "maven"
}

func (m *MavenTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, force bool) (*InstallResult, error) {
	if !force {
		if existingDir, ok := findExistingCandidate(doguHome, "maven", versionSpec); ok {
			return buildResultFromDir("maven", versionSpec, existingDir), nil
		}
	}

	normVer := strings.TrimPrefix(versionSpec, "v")

	baseURL := "https://archive.apache.org/dist/maven/maven-3"
	if mirrorBaseURL != "" {
		baseURL = strings.TrimSuffix(mirrorBaseURL, "/")
	}

	filename := fmt.Sprintf("apache-maven-%s-bin.tar.gz", normVer)
	downloadURL := fmt.Sprintf("%s/%s/binaries/%s", baseURL, normVer, filename)

	installDir := filepath.Join(doguHome, "candidates", "maven", versionSpec)
	cacheFile := filepath.Join(doguHome, "cache", "maven", filename)

	if err := downloader.DownloadAndExtract(ctx, downloadURL, cacheFile, installDir); err != nil {
		return nil, fmt.Errorf("failed to download maven from %s: %w", downloadURL, err)
	}

	m2Home := downloader.FindSubDirWithDir(installDir, "bin")
	binDir := filepath.Join(m2Home, "bin")

	majorVer := env.ExtractMajorVersion(normVer)
	envVars := map[string]string{
		env.FormatEnvKey("maven", majorVer): m2Home,
		env.FormatEnvKey("maven", ""):        m2Home,
		"M2_HOME":                           m2Home,
		"MAVEN_HOME":                        m2Home,
	}

	return &InstallResult{
		ToolName:      "maven",
		RequestedVer:  versionSpec,
		NormalizedVer: normVer,
		InstallDir:    installDir,
		HomeDir:       m2Home,
		BinDirs:       []string{binDir},
		EnvVars:       envVars,
	}, nil
}
