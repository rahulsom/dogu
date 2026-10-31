package tools

import (
	"context"
	"fmt"
)

type YarnTool struct{}

func init() {
	Register(&YarnTool{})
}

func (y *YarnTool) Name() string {
	return "yarn"
}

func (y *YarnTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if res, ok, err := checkExistingCandidate(doguHome, "yarn", versionSpec, opts, nil); ok || err != nil {
		return res, err
	}

	normVer := resolveNPMToolVersion(ctx, "yarn", versionSpec, mirrorBaseURL, opts.IncludePrerelease)
	baseURL := resolveBaseURL("https://github.com/yarnpkg/yarn/releases/download", mirrorBaseURL)

	filename := fmt.Sprintf("yarn-v%s.tar.gz", normVer)
	downloadURL := fmt.Sprintf("%s/v%s/%s", baseURL, normVer, filename)

	return installArchiveTool(ctx, ArchiveInstallConfig{
		ToolName:      "yarn",
		VersionSpec:   versionSpec,
		NormalizedVer: normVer,
		DownloadURL:   downloadURL,
		FileName:      filename,
		DoguHome:      doguHome,
		Opts:          opts,
	})
}
