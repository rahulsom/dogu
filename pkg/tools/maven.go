package tools

import (
	"context"
	"fmt"
)

type MavenTool struct{}

func init() {
	Register(&MavenTool{})
}

func (m *MavenTool) Name() string {
	return "maven"
}

func (m *MavenTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if res, ok, err := checkExistingCandidate(doguHome, "maven", versionSpec, opts, nil); ok || err != nil {
		return res, err
	}

	baseURL := resolveBaseURL("https://archive.apache.org/dist/maven/maven-3", mirrorBaseURL)
	normVer := resolveMavenVersion(ctx, versionSpec, baseURL, opts.IncludePrerelease)

	filename := fmt.Sprintf("apache-maven-%s-bin.tar.gz", normVer)
	downloadURL := fmt.Sprintf("%s/%s/binaries/%s", baseURL, normVer, filename)

	return installArchiveTool(ctx, ArchiveInstallConfig{
		ToolName:      "maven",
		VersionSpec:   versionSpec,
		NormalizedVer: normVer,
		DownloadURL:   downloadURL,
		FileName:      filename,
		DoguHome:      doguHome,
		Opts:          opts,
		ExtraHomeEnv:  []string{"M2_HOME", "MAVEN_HOME"},
	})
}

func resolveMavenVersion(ctx context.Context, versionSpec string, baseURL string, includePrerelease bool) string {
	if versions, err := FetchApacheMavenVersions(ctx, baseURL); err == nil && len(versions) > 0 {
		if best, err := SelectBestVersion(versions, versionSpec, includePrerelease); err == nil {
			return CleanVersion(best)
		}
	}
	return CleanVersion(versionSpec)
}
