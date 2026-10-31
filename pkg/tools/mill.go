package tools

import (
	"context"
	"fmt"
	"runtime"
)

type MillTool struct{}

func init() {
	Register(&MillTool{})
}

func (m *MillTool) Name() string {
	return "mill"
}

func (m *MillTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if res, ok, err := checkExistingCandidate(doguHome, "mill", versionSpec, opts, nil); ok || err != nil {
		return res, err
	}

	baseURL := resolveBaseURL("https://github.com/com-lihaoyi/mill/releases/download", mirrorBaseURL)
	normVer := resolveMillVersion(ctx, versionSpec, opts.IncludePrerelease)

	assetName := normVer
	binaryName := "mill"
	if runtime.GOOS == "windows" {
		assetName = normVer + ".bat"
		binaryName = "mill.bat"
	}

	return installBinaryTool(ctx, BinaryInstallConfig{
		ToolName:      "mill",
		VersionSpec:   versionSpec,
		NormalizedVer: normVer,
		DownloadURL:   fmt.Sprintf("%s/%s/%s", baseURL, normVer, assetName),
		CacheFileName: fmt.Sprintf("mill-%s-%s", normVer, binaryName),
		TargetBin:     binaryName,
		DoguHome:      doguHome,
		Opts:          opts,
		ExtraHomeEnv:  []string{"MILL_HOME"},
		ExtraEnv:      map[string]string{"MILL_VERSION": normVer},
	})
}

func resolveMillVersion(ctx context.Context, versionSpec string, includePrerelease bool) string {
	return ResolveGitHubVersion(ctx, "com-lihaoyi", "mill", versionSpec, includePrerelease)
}
