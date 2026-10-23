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

type SBTTool struct{}

func init() {
	Register(&SBTTool{})
}

func (s *SBTTool) Name() string {
	return "sbt"
}

func (s *SBTTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if !opts.Force {
		if existingDir, ok := findExistingCandidate(doguHome, "sbt", versionSpec, opts.IncludePrerelease); ok {
			return buildResultFromDir("sbt", versionSpec, existingDir), nil
		}
	}

	baseURL := "https://github.com/sbt/sbt/releases/download"
	if mirrorBaseURL != "" {
		baseURL = strings.TrimSuffix(mirrorBaseURL, "/")
	}

	normVer := resolveSBTVersion(ctx, versionSpec, opts.IncludePrerelease)

	ext := "tgz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	filename := fmt.Sprintf("sbt-%s.%s", normVer, ext)
	downloadURL := fmt.Sprintf("%s/v%s/%s", baseURL, normVer, filename)

	installDir := filepath.Join(doguHome, "candidates", "sbt", versionSpec)
	cacheFile := filepath.Join(doguHome, "cache", "sbt", filename)

	if err := downloader.DownloadAndExtract(ctx, downloadURL, cacheFile, installDir, downloader.WithRewriteRule(opts.RewriteRules)); err != nil {
		return nil, fmt.Errorf("failed to download sbt: %w", err)
	}

	sbtHome := downloader.FindSubDirWithDir(installDir, "bin")
	binDir := filepath.Join(sbtHome, "bin")

	majorVer := env.ExtractMajorVersion(normVer)
	envVars := map[string]string{
		env.FormatEnvKey("sbt", majorVer): sbtHome,
		env.FormatEnvKey("sbt", ""):        sbtHome,
		"SBT_HOME":                        sbtHome,
	}

	return &InstallResult{
		ToolName:      "sbt",
		RequestedVer:  versionSpec,
		NormalizedVer: normVer,
		InstallDir:    installDir,
		HomeDir:       sbtHome,
		BinDirs:       []string{binDir},
		EnvVars:       envVars,
	}, nil
}

func resolveSBTVersion(ctx context.Context, versionSpec string, includePrerelease bool) string {
	if versions, err := FetchGitHubReleases(ctx, "sbt", "sbt"); err == nil && len(versions) > 0 {
		if best, err := SelectBestVersion(versions, versionSpec, includePrerelease); err == nil {
			return CleanVersion(best)
		}
	}
	return CleanVersion(versionSpec)
}
