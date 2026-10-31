package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type NPMTool struct{}

func init() {
	Register(&NPMTool{})
}

func (n *NPMTool) Name() string {
	return "npm"
}

func (n *NPMTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	postNPM := func(res *InstallResult) error {
		for _, b := range res.BinDirs {
			ensureNPMWrappers(b)
		}
		return nil
	}

	if res, ok, err := checkExistingCandidate(doguHome, "npm", versionSpec, opts, postNPM); ok || err != nil {
		return res, err
	}

	baseURL := resolveBaseURL("https://registry.npmjs.org/npm/-", mirrorBaseURL)
	normVer := resolveNPMToolVersion(ctx, "npm", versionSpec, mirrorBaseURL, opts.IncludePrerelease)

	filename := fmt.Sprintf("npm-%s.tgz", normVer)
	downloadURL := fmt.Sprintf("%s/%s", baseURL, filename)

	return installArchiveTool(ctx, ArchiveInstallConfig{
		ToolName:      "npm",
		VersionSpec:   versionSpec,
		NormalizedVer: normVer,
		DownloadURL:   downloadURL,
		FileName:      filename,
		DoguHome:      doguHome,
		Opts:          opts,
		PostInstall:   postNPM,
	})
}

func resolveNPMToolVersion(ctx context.Context, pkgName string, versionSpec string, mirrorBaseURL string, includePrerelease bool) string {
	if versions, err := FetchNPMVersions(ctx, pkgName, mirrorBaseURL); err == nil && len(versions) > 0 {
		if best, err := SelectBestVersion(versions, versionSpec, includePrerelease); err == nil {
			return CleanVersion(best)
		}
	}
	return CleanVersion(versionSpec)
}

func ensureNPMWrappers(binDir string) {
	npmBin := filepath.Join(binDir, "npm")
	npmScript := "#!/bin/sh\nexec node \"$(dirname \"$0\")/npm-cli.js\" \"$@\"\n"
	os.Remove(npmBin)
	_ = os.WriteFile(npmBin, []byte(npmScript), 0755)

	npmCmd := filepath.Join(binDir, "npm.cmd")
	npmCmdScript := "@\"node\" \"%~dp0\\npm-cli.js\" %*\n"
	os.Remove(npmCmd)
	_ = os.WriteFile(npmCmd, []byte(npmCmdScript), 0755)

	npxBin := filepath.Join(binDir, "npx")
	npxScript := "#!/bin/sh\nexec node \"$(dirname \"$0\")/npx-cli.js\" \"$@\"\n"
	os.Remove(npxBin)
	_ = os.WriteFile(npxBin, []byte(npxScript), 0755)

	npxCmd := filepath.Join(binDir, "npx.cmd")
	npxCmdScript := "@\"node\" \"%~dp0\\npx-cli.js\" %*\n"
	os.Remove(npxCmd)
	_ = os.WriteFile(npxCmd, []byte(npxCmdScript), 0755)
}
