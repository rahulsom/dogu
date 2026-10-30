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

type PNPMTool struct{}

func init() {
	Register(&PNPMTool{})
}

func (p *PNPMTool) Name() string {
	return "pnpm"
}

func (p *PNPMTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if !opts.Force {
		if existingDir, ok := findExistingCandidate(doguHome, "pnpm", versionSpec, opts.IncludePrerelease); ok {
			return buildResultFromDir("pnpm", versionSpec, existingDir), nil
		}
	}

	normVer := resolveNPMToolVersion(ctx, "pnpm", versionSpec, mirrorBaseURL, opts.IncludePrerelease)

	baseURL := "https://github.com/pnpm/pnpm/releases/download"
	if mirrorBaseURL != "" {
		baseURL = strings.TrimSuffix(mirrorBaseURL, "/")
	}

	osName := mapPNPMOS()
	archName := mapPNPMArch()

	binFileName := fmt.Sprintf("pnpm-%s-%s", osName, archName)
	if osName == "win" {
		binFileName += ".exe"
	}

	downloadURL := fmt.Sprintf("%s/v%s/%s", baseURL, normVer, binFileName)

	installDir := filepath.Join(doguHome, "candidates", "pnpm", normVer)
	cacheFile := filepath.Join(doguHome, "cache", "pnpm", binFileName)

	targetBinName := "pnpm"
	if osName == "win" {
		targetBinName = "pnpm.exe"
	}

	if err := downloader.SaveSingleBinary(ctx, downloadURL, cacheFile, installDir, targetBinName, downloader.WithRewriteRule(opts.RewriteRules)); err != nil {
		return nil, fmt.Errorf("failed to download pnpm: %w", err)
	}

	majorVer := env.ExtractMajorVersion(normVer)
	envVars := map[string]string{
		env.FormatEnvKey("pnpm", majorVer): installDir,
		env.FormatEnvKey("pnpm", ""):        installDir,
	}

	return &InstallResult{
		ToolName:      "pnpm",
		RequestedVer:  versionSpec,
		NormalizedVer: normVer,
		InstallDir:    installDir,
		HomeDir:       installDir,
		BinDirs:       []string{installDir},
		EnvVars:       envVars,
	}, nil
}

func mapPNPMOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	case "windows":
		return "win"
	default:
		return "linux"
	}
}

func mapPNPMArch() string {
	switch runtime.GOARCH {
	case "arm64":
		return "arm64"
	default:
		return "x64"
	}
}
