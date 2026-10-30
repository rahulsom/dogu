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

func (m *MavenTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if !opts.Force {
		if existingDir, ok := findExistingCandidate(doguHome, "maven", versionSpec, opts.IncludePrerelease); ok {
			return buildResultFromDir("maven", versionSpec, existingDir), nil
		}
	}

	baseURL := "https://archive.apache.org/dist/maven/maven-3"
	if mirrorBaseURL != "" {
		baseURL = strings.TrimSuffix(mirrorBaseURL, "/")
	}

	normVer := resolveMavenVersion(ctx, versionSpec, baseURL, opts.IncludePrerelease)

	filename := fmt.Sprintf("apache-maven-%s-bin.tar.gz", normVer)
	downloadURL := fmt.Sprintf("%s/%s/binaries/%s", baseURL, normVer, filename)

	installDir := filepath.Join(doguHome, "candidates", "maven", normVer)
	cacheFile := filepath.Join(doguHome, "cache", "maven", filename)

	if err := downloader.DownloadAndExtract(ctx, downloadURL, cacheFile, installDir, downloader.WithRewriteRule(opts.RewriteRules)); err != nil {
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

func resolveMavenVersion(ctx context.Context, versionSpec string, baseURL string, includePrerelease bool) string {
	if versions, err := FetchApacheMavenVersions(ctx, baseURL); err == nil && len(versions) > 0 {
		if best, err := SelectBestVersion(versions, versionSpec, includePrerelease); err == nil {
			return CleanVersion(best)
		}
	}
	return CleanVersion(versionSpec)
}
