package tools

import (
	"context"
	"fmt"
	"runtime"
)

type SBTTool struct{}

func init() {
	Register(&SBTTool{})
}

func (s *SBTTool) Name() string {
	return "sbt"
}

func (s *SBTTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if res, ok, err := checkExistingCandidate(doguHome, "sbt", versionSpec, opts, nil); ok || err != nil {
		return res, err
	}

	baseURL := resolveBaseURL("https://github.com/sbt/sbt/releases/download", mirrorBaseURL)
	normVer := resolveSBTVersion(ctx, versionSpec, opts.IncludePrerelease)

	ext := "tgz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	filename := fmt.Sprintf("sbt-%s.%s", normVer, ext)
	downloadURL := fmt.Sprintf("%s/v%s/%s", baseURL, normVer, filename)

	return installArchiveTool(ctx, ArchiveInstallConfig{
		ToolName:      "sbt",
		VersionSpec:   versionSpec,
		NormalizedVer: normVer,
		DownloadURL:   downloadURL,
		FileName:      filename,
		DoguHome:      doguHome,
		Opts:          opts,
		ExtraHomeEnv:  []string{"SBT_HOME"},
	})
}

func resolveSBTVersion(ctx context.Context, versionSpec string, includePrerelease bool) string {
	return ResolveGitHubVersion(ctx, "sbt", "sbt", versionSpec, includePrerelease)
}
