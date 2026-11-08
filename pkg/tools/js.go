package tools

import (
	"bufio"
	"context"
	"fmt"
	"github.com/dogu-dev/dogu/pkg/downloader"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type BunTool struct{}

func init() {
	Register(&BunTool{})
}

func (b *BunTool) Name() string {
	return "bun"
}

func (b *BunTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if res, ok, err := checkExistingCandidate(doguHome, "bun", versionSpec, opts, nil); ok || err != nil {
		return res, err
	}

	baseURL := resolveBaseURL("https://github.com/oven-sh/bun/releases/download", mirrorBaseURL)
	tag, normVer := resolveBunVersion(ctx, versionSpec, opts.IncludePrerelease)

	targetOS, targetArch := mapBunTarget()
	filename := fmt.Sprintf("bun-%s-%s.zip", targetOS, targetArch)
	downloadURL := fmt.Sprintf("%s/%s/%s", baseURL, tag, filename)

	return installArchiveTool(ctx, ArchiveInstallConfig{
		ToolName:      "bun",
		VersionSpec:   versionSpec,
		NormalizedVer: normVer,
		DownloadURL:   downloadURL,
		FileName:      filename,
		DoguHome:      doguHome,
		Opts:          opts,
		FindDirs: func(installDir string) (string, []string) {
			return findDirWithBinary(installDir, "bun")
		},
	})
}

func resolveBunVersion(ctx context.Context, versionSpec string, includePrerelease bool) (tag string, normVer string) {
	if versions, err := FetchGitHubReleases(ctx, "oven-sh", "bun"); err == nil && len(versions) > 0 {
		if best, err := SelectBestVersion(versions, versionSpec, includePrerelease); err == nil {
			clean := CleanVersion(best)
			return "bun-v" + clean, "v" + clean
		}
	}

	clean := CleanVersion(versionSpec)
	return "bun-v" + clean, "v" + clean
}

func mapBunTarget() (string, string) {
	osName := "linux"
	switch runtime.GOOS {
	case "darwin":
		osName = "darwin"
	case "windows":
		osName = "windows"
	}

	archName := "x64"
	if runtime.GOARCH == "arm64" {
		archName = "aarch64"
	}

	return osName, archName
}

type NodeTool struct{}

func init() {
	Register(&NodeTool{})
}

func (n *NodeTool) Name() string {
	return "node"
}

func (n *NodeTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if res, ok, err := checkExistingCandidate(doguHome, "node", versionSpec, opts, nil); ok || err != nil {
		return res, err
	}

	baseURL := resolveBaseURL("https://nodejs.org/dist", mirrorBaseURL)
	normVer, err := resolveNodeFullVersion(ctx, baseURL, versionSpec, opts.IncludePrerelease)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve node version '%s': %w", versionSpec, err)
	}

	osPlatform, arch := mapNodePlatform(runtime.GOOS, runtime.GOARCH)

	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}

	filename := fmt.Sprintf("node-%s-%s-%s.%s", normVer, osPlatform, arch, ext)
	downloadURL := fmt.Sprintf("%s/%s/%s", baseURL, normVer, filename)

	return installArchiveTool(ctx, ArchiveInstallConfig{
		ToolName:      "node",
		VersionSpec:   versionSpec,
		NormalizedVer: normVer,
		DownloadURL:   downloadURL,
		FileName:      filename,
		DoguHome:      doguHome,
		Opts:          opts,
		FindDirs: func(installDir string) (string, []string) {
			nodeHome := downloader.FindSubDirWithDir(installDir, "bin")
			binDir := filepath.Join(nodeHome, "bin")
			if runtime.GOOS == "windows" {
				if _, err := filepath.Glob(filepath.Join(nodeHome, "node.exe")); err == nil {
					binDir = nodeHome
				}
			}
			return nodeHome, []string{binDir}
		},
	})
}

func resolveNodeFullVersion(ctx context.Context, baseURL string, spec string, includePrerelease bool) (string, error) {
	clean := spec
	if !strings.HasPrefix(clean, "v") {
		clean = "v" + clean
	}

	indexURL := fmt.Sprintf("%s/index.tab", baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", indexURL, nil)
	if err != nil {
		return clean, nil
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return clean, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return clean, nil
	}

	var versions []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, "\t")
		if len(fields) > 0 {
			ver := fields[0]
			if ver != "" && ver != "version" {
				versions = append(versions, ver)
			}
		}
	}

	if len(versions) > 0 {
		if best, err := SelectBestVersion(versions, spec, includePrerelease); err == nil {
			if !strings.HasPrefix(best, "v") {
				best = "v" + CleanVersion(best)
			}
			return best, nil
		}
	}

	return clean, nil
}

func mapNodePlatform(goos, goarch string) (string, string) {
	platform := goos
	if goos == "darwin" {
		platform = "darwin"
	} else if goos == "windows" {
		platform = "win"
	}

	arch := goarch
	if goarch == "amd64" {
		arch = "x64"
	} else if goarch == "arm64" {
		arch = "arm64"
	}

	return platform, arch
}

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

// YarnTool installs yarn using the shared tool definition.
type YarnTool struct{}

func init() { Register(&YarnTool{}) }

func (*YarnTool) Name() string { return "yarn" }

func (*YarnTool) Install(ctx context.Context, versionSpec, doguHome, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	return yarnDefinition.Install(ctx, versionSpec, doguHome, mirrorBaseURL, opts)
}
