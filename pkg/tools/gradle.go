package tools

import (
	"context"
	"fmt"
)

type GradleTool struct{}

func init() {
	Register(&GradleTool{})
}

func (g *GradleTool) Name() string {
	return "gradle"
}

func (g *GradleTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if res, ok, err := checkExistingCandidate(doguHome, "gradle", versionSpec, opts, nil); ok || err != nil {
		return res, err
	}

	baseURL := resolveBaseURL("https://services.gradle.org/distributions", mirrorBaseURL)
	normVer := resolveGradleVersion(ctx, versionSpec, mirrorBaseURL, opts.IncludePrerelease)

	filename := fmt.Sprintf("gradle-%s-bin.zip", normVer)
	downloadURL := fmt.Sprintf("%s/%s", baseURL, filename)

	return installArchiveTool(ctx, ArchiveInstallConfig{
		ToolName:      "gradle",
		VersionSpec:   versionSpec,
		NormalizedVer: normVer,
		DownloadURL:   downloadURL,
		FileName:      filename,
		DoguHome:      doguHome,
		Opts:          opts,
		ExtraHomeEnv:  []string{"GRADLE_HOME"},
	})
}

func resolveGradleVersion(ctx context.Context, versionSpec string, mirrorBaseURL string, includePrerelease bool) string {
	if versions, err := FetchGradleVersions(ctx, mirrorBaseURL); err == nil && len(versions) > 0 {
		if best, err := SelectBestVersion(versions, versionSpec, includePrerelease); err == nil {
			return CleanVersion(best)
		}
	}
	return CleanVersion(versionSpec)
}
