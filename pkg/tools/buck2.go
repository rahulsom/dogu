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

type Buck2Tool struct{}

func init() {
	Register(&Buck2Tool{})
}

func (b *Buck2Tool) Name() string {
	return "buck2"
}

func (b *Buck2Tool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if !opts.Force {
		if existingDir, ok := findExistingCandidate(doguHome, "buck2", versionSpec, opts.IncludePrerelease); ok {
			return buildResultFromDir("buck2", versionSpec, existingDir), nil
		}
	}

	baseURL := "https://github.com/facebook/buck2/releases/download"
	if mirrorBaseURL != "" {
		baseURL = strings.TrimSuffix(mirrorBaseURL, "/")
	}

	normVer := resolveBuck2Version(ctx, versionSpec, opts.IncludePrerelease)

	arch, sys := mapBuck2Target()
	binName := fmt.Sprintf("buck2-%s-%s.zst", arch, sys)
	targetBin := "buck2"
	if runtime.GOOS == "windows" {
		targetBin = "buck2.exe"
	}

	downloadURL := fmt.Sprintf("%s/%s/%s", baseURL, normVer, binName)

	installDir := filepath.Join(doguHome, "candidates", "buck2", versionSpec)
	cacheFile := filepath.Join(doguHome, "cache", "buck2", binName)

	if err := downloader.SaveSingleBinary(ctx, downloadURL, cacheFile, installDir, targetBin, downloader.WithRewriteRule(opts.RewriteRules)); err != nil {
		return nil, fmt.Errorf("failed to download buck2: %w", err)
	}

	majorVer := env.ExtractMajorVersion(normVer)
	envVars := map[string]string{
		env.FormatEnvKey("buck2", majorVer): installDir,
		env.FormatEnvKey("buck2", ""):        installDir,
		"BUCK2_HOME":                        installDir,
	}

	return &InstallResult{
		ToolName:      "buck2",
		RequestedVer:  versionSpec,
		NormalizedVer: normVer,
		InstallDir:    installDir,
		HomeDir:       installDir,
		BinDirs:       []string{installDir},
		EnvVars:       envVars,
	}, nil
}

func resolveBuck2Version(ctx context.Context, versionSpec string, includePrerelease bool) string {
	if versions, err := FetchGitHubReleases(ctx, "facebook", "buck2"); err == nil && len(versions) > 0 {
		if best, err := SelectBestVersion(versions, versionSpec, includePrerelease); err == nil {
			return CleanVersion(best)
		}
	}
	return CleanVersion(versionSpec)
}

func mapBuck2Target() (string, string) {
	arch := "x86_64"
	if runtime.GOARCH == "arm64" {
		arch = "aarch64"
	}

	switch runtime.GOOS {
	case "darwin":
		return arch, "apple-darwin"
	case "windows":
		return arch, "pc-windows-msvc.exe"
	default:
		return arch, "unknown-linux-gnu"
	}
}
