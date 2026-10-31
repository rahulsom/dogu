package tools

import (
	"context"
	"fmt"
	"runtime"
)

type PNPMTool struct{}

func init() {
	Register(&PNPMTool{})
}

func (p *PNPMTool) Name() string {
	return "pnpm"
}

func (p *PNPMTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if res, ok, err := checkExistingCandidate(doguHome, "pnpm", versionSpec, opts, nil); ok || err != nil {
		return res, err
	}

	normVer := resolveNPMToolVersion(ctx, "pnpm", versionSpec, mirrorBaseURL, opts.IncludePrerelease)
	baseURL := resolveBaseURL("https://github.com/pnpm/pnpm/releases/download", mirrorBaseURL)

	osName := mapPNPMOS()
	archName := mapPNPMArch()

	binFileName := fmt.Sprintf("pnpm-%s-%s", osName, archName)
	targetBinName := "pnpm"
	if osName == "win" {
		binFileName += ".exe"
		targetBinName = "pnpm.exe"
	}

	return installBinaryTool(ctx, BinaryInstallConfig{
		ToolName:      "pnpm",
		VersionSpec:   versionSpec,
		NormalizedVer: normVer,
		DownloadURL:   fmt.Sprintf("%s/v%s/%s", baseURL, normVer, binFileName),
		CacheFileName: binFileName,
		TargetBin:     targetBinName,
		DoguHome:      doguHome,
		Opts:          opts,
	})
}

func mapPNPMOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	case "windows":
		return "win"
	default:
		return "linux"
	}
}

func mapPNPMArch() string {
	switch runtime.GOARCH {
	case "arm64":
		return "arm64"
	default:
		return "x64"
	}
}
