package tools

import (
	"context"
	"fmt"
	"runtime"
)

type Buck2Tool struct{}

func init() {
	Register(&Buck2Tool{})
}

func (b *Buck2Tool) Name() string {
	return "buck2"
}

func (b *Buck2Tool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if res, ok, err := checkExistingCandidate(doguHome, "buck2", versionSpec, opts, nil); ok || err != nil {
		return res, err
	}

	baseURL := resolveBaseURL("https://github.com/facebook/buck2/releases/download", mirrorBaseURL)
	normVer := resolveBuck2Version(ctx, versionSpec, opts.IncludePrerelease)

	arch, sys := mapBuck2Target()
	binName := fmt.Sprintf("buck2-%s-%s.zst", arch, sys)
	targetBin := "buck2"
	if runtime.GOOS == "windows" {
		targetBin = "buck2.exe"
	}

	return installBinaryTool(ctx, BinaryInstallConfig{
		ToolName:      "buck2",
		VersionSpec:   versionSpec,
		NormalizedVer: normVer,
		DownloadURL:   fmt.Sprintf("%s/%s/%s", baseURL, normVer, binName),
		CacheFileName: binName,
		TargetBin:     targetBin,
		DoguHome:      doguHome,
		Opts:          opts,
		ExtraHomeEnv:  []string{"BUCK2_HOME"},
	})
}

func resolveBuck2Version(ctx context.Context, versionSpec string, includePrerelease bool) string {
	return ResolveGitHubVersion(ctx, "facebook", "buck2", versionSpec, includePrerelease)
}

func mapBuck2Target() (string, string) {
	arch := "x86_64"
	if runtime.GOARCH == "arm64" {
		arch = "aarch64"
	}

	switch runtime.GOOS {
	case "darwin":
		return arch, "apple-darwin"
	case "windows":
		return arch, "pc-windows-msvc.exe"
	default:
		return arch, "unknown-linux-gnu"
	}
}
