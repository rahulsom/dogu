package tools

import (
	"context"
	"fmt"
)

type AntTool struct{}

func init() {
	Register(&AntTool{})
}

func (a *AntTool) Name() string {
	return "ant"
}

func (a *AntTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if res, ok, err := checkExistingCandidate(doguHome, "ant", versionSpec, opts, nil); ok || err != nil {
		return res, err
	}

	baseURL := resolveBaseURL("https://archive.apache.org/dist/ant/binaries", mirrorBaseURL)
	normVer := resolveAntVersion(ctx, versionSpec, baseURL, opts.IncludePrerelease)

	filename := fmt.Sprintf("apache-ant-%s-bin.tar.gz", normVer)
	downloadURL := fmt.Sprintf("%s/%s", baseURL, filename)

	return installArchiveTool(ctx, ArchiveInstallConfig{
		ToolName:      "ant",
		VersionSpec:   versionSpec,
		NormalizedVer: normVer,
		DownloadURL:   downloadURL,
		FileName:      filename,
		DoguHome:      doguHome,
		Opts:          opts,
		ExtraHomeEnv:  []string{"ANT_HOME"},
	})
}

func resolveAntVersion(ctx context.Context, versionSpec string, baseURL string, includePrerelease bool) string {
	if versions, err := FetchApacheAntVersions(ctx, baseURL); err == nil && len(versions) > 0 {
		if best, err := SelectBestVersion(versions, versionSpec, includePrerelease); err == nil {
			return CleanVersion(best)
		}
	}
	return CleanVersion(versionSpec)
}
