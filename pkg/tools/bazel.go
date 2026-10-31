package tools

import (
	"context"
	"fmt"
	"runtime"
)

type BazelTool struct{}

func init() {
	Register(&BazelTool{})
}

func (b *BazelTool) Name() string {
	return "bazel"
}

func (b *BazelTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if res, ok, err := checkExistingCandidate(doguHome, "bazel", versionSpec, opts, nil); ok || err != nil {
		return res, err
	}

	baseURL := resolveBaseURL("https://github.com/bazelbuild/bazel/releases/download", mirrorBaseURL)
	normVer := resolveBazelVersion(ctx, versionSpec, opts.IncludePrerelease)

	osName, archName := mapBazelTarget()
	binName := fmt.Sprintf("bazel-%s-%s-%s", normVer, osName, archName)
	targetBin := "bazel"
	if osName == "windows" {
		binName += ".exe"
		targetBin = "bazel.exe"
	}

	return installBinaryTool(ctx, BinaryInstallConfig{
		ToolName:      "bazel",
		VersionSpec:   versionSpec,
		NormalizedVer: normVer,
		DownloadURL:   fmt.Sprintf("%s/%s/%s", baseURL, normVer, binName),
		CacheFileName: binName,
		TargetBin:     targetBin,
		DoguHome:      doguHome,
		Opts:          opts,
		ExtraHomeEnv:  []string{"BAZEL_HOME"},
	})
}

func resolveBazelVersion(ctx context.Context, versionSpec string, includePrerelease bool) string {
	return ResolveGitHubVersion(ctx, "bazelbuild", "bazel", versionSpec, includePrerelease)
}

func mapBazelTarget() (string, string) {
	osName := "linux"
	switch runtime.GOOS {
	case "darwin":
		osName = "darwin"
	case "windows":
		osName = "windows"
	}

	archName := "x86_64"
	if runtime.GOARCH == "arm64" {
		archName = "arm64"
	}

	return osName, archName
}
