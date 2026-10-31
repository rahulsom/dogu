package tools

import (
	"context"
	"fmt"
	"runtime"
)

type BunTool struct{}

func init() {
	Register(&BunTool{})
}

func (b *BunTool) Name() string {
	return "bun"
}

func (b *BunTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if res, ok, err := checkExistingCandidate(doguHome, "bun", versionSpec, opts, nil); ok || err != nil {
		return res, err
	}

	baseURL := resolveBaseURL("https://github.com/oven-sh/bun/releases/download", mirrorBaseURL)
	tag, normVer := resolveBunVersion(ctx, versionSpec, opts.IncludePrerelease)

	targetOS, targetArch := mapBunTarget()
	filename := fmt.Sprintf("bun-%s-%s.zip", targetOS, targetArch)
	downloadURL := fmt.Sprintf("%s/%s/%s", baseURL, tag, filename)

	return installArchiveTool(ctx, ArchiveInstallConfig{
		ToolName:      "bun",
		VersionSpec:   versionSpec,
		NormalizedVer: normVer,
		DownloadURL:   downloadURL,
		FileName:      filename,
		DoguHome:      doguHome,
		Opts:          opts,
		FindDirs: func(installDir string) (string, []string) {
			return findDirWithBinary(installDir, "bun")
		},
	})
}

func resolveBunVersion(ctx context.Context, versionSpec string, includePrerelease bool) (tag string, normVer string) {
	if versions, err := FetchGitHubReleases(ctx, "oven-sh", "bun"); err == nil && len(versions) > 0 {
		if best, err := SelectBestVersion(versions, versionSpec, includePrerelease); err == nil {
			clean := CleanVersion(best)
			return "bun-v" + clean, "v" + clean
		}
	}

	clean := CleanVersion(versionSpec)
	return "bun-v" + clean, "v" + clean
}

func mapBunTarget() (string, string) {
	osName := "linux"
	switch runtime.GOOS {
	case "darwin":
		osName = "darwin"
	case "windows":
		osName = "windows"
	}

	archName := "x64"
	if runtime.GOARCH == "arm64" {
		archName = "aarch64"
	}

	return osName, archName
}
