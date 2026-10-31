package tools

import (
	"context"
	"fmt"
	"runtime"
)

type UVTool struct{}

func init() {
	Register(&UVTool{})
}

func (u *UVTool) Name() string {
	return "uv"
}

func (u *UVTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if res, ok, err := checkExistingCandidate(doguHome, "uv", versionSpec, opts, nil); ok || err != nil {
		return res, err
	}

	baseURL := resolveBaseURL("https://github.com/astral-sh/uv/releases/download", mirrorBaseURL)
	normVer := resolveUVVersion(ctx, versionSpec, opts.IncludePrerelease)

	target := mapUVTarget()
	filename := fmt.Sprintf("uv-%s.tar.gz", target)
	if runtime.GOOS == "windows" {
		filename = fmt.Sprintf("uv-%s.zip", target)
	}

	downloadURL := fmt.Sprintf("%s/%s/%s", baseURL, normVer, filename)

	return installArchiveTool(ctx, ArchiveInstallConfig{
		ToolName:      "uv",
		VersionSpec:   versionSpec,
		NormalizedVer: normVer,
		DownloadURL:   downloadURL,
		FileName:      filename,
		DoguHome:      doguHome,
		Opts:          opts,
		FindDirs: func(installDir string) (string, []string) {
			return findDirWithBinary(installDir, "uv")
		},
	})
}

func resolveUVVersion(ctx context.Context, versionSpec string, includePrerelease bool) string {
	if versions, err := FetchPyPIVersions(ctx, "uv"); err == nil && len(versions) > 0 {
		if best, err := SelectBestVersion(versions, versionSpec, includePrerelease); err == nil {
			return CleanVersion(best)
		}
	}
	if versions, err := FetchGitHubReleases(ctx, "astral-sh", "uv"); err == nil && len(versions) > 0 {
		if best, err := SelectBestVersion(versions, versionSpec, includePrerelease); err == nil {
			return CleanVersion(best)
		}
	}
	return CleanVersion(versionSpec)
}

func mapUVTarget() string {
	arch := "x86_64"
	if runtime.GOARCH == "arm64" {
		arch = "aarch64"
	}

	switch runtime.GOOS {
	case "darwin":
		return arch + "-apple-darwin"
	case "windows":
		return arch + "-pc-windows-msvc"
	default:
		return arch + "-unknown-linux-gnu"
	}
}
