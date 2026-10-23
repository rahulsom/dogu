package tools

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dogu-dev/dogu/pkg/downloader"
	"github.com/dogu-dev/dogu/pkg/env"
)

type NodeTool struct{}

func init() {
	Register(&NodeTool{})
}

func (n *NodeTool) Name() string {
	return "node"
}

func (n *NodeTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if !opts.Force {
		if existingDir, ok := findExistingCandidate(doguHome, "node", versionSpec, opts.IncludePrerelease); ok {
			return buildResultFromDir("node", versionSpec, existingDir), nil
		}
	}

	baseURL := "https://nodejs.org/dist"
	if mirrorBaseURL != "" {
		baseURL = strings.TrimSuffix(mirrorBaseURL, "/")
	}

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

	installDir := filepath.Join(doguHome, "candidates", "node", versionSpec)
	cacheFile := filepath.Join(doguHome, "cache", "node", filename)

	if err := downloader.DownloadAndExtract(ctx, downloadURL, cacheFile, installDir, downloader.WithRewriteRule(opts.RewriteRules)); err != nil {
		return nil, fmt.Errorf("failed to download node from %s: %w", downloadURL, err)
	}

	nodeHome := downloader.FindSubDirWithDir(installDir, "bin")
	binDir := filepath.Join(nodeHome, "bin")
	if runtime.GOOS == "windows" {
		if _, err := filepath.Glob(filepath.Join(nodeHome, "node.exe")); err == nil {
			binDir = nodeHome
		}
	}

	majorVer := env.ExtractMajorVersion(versionSpec)
	envVars := map[string]string{
		env.FormatEnvKey("node", majorVer): nodeHome,
		env.FormatEnvKey("node", ""):        nodeHome,
	}

	return &InstallResult{
		ToolName:      "node",
		RequestedVer:  versionSpec,
		NormalizedVer: normVer,
		InstallDir:    installDir,
		HomeDir:       nodeHome,
		BinDirs:       []string{binDir},
		EnvVars:       envVars,
	}, nil
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
