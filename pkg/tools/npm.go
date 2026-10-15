package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dogu-dev/dogu/pkg/downloader"
	"github.com/dogu-dev/dogu/pkg/env"
)

type NPMTool struct{}

func init() {
	Register(&NPMTool{})
}

func (n *NPMTool) Name() string {
	return "npm"
}

func (n *NPMTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, force bool) (*InstallResult, error) {
	if !force {
		if existingDir, ok := findExistingCandidate(doguHome, "npm", versionSpec); ok {
			res := buildResultFromDir("npm", versionSpec, existingDir)
			for _, b := range res.BinDirs {
				ensureNPMWrappers(b)
			}
			return res, nil
		}
	}

	normVer := strings.TrimPrefix(versionSpec, "v")

	baseURL := "https://registry.npmjs.org/npm/-"
	if mirrorBaseURL != "" {
		baseURL = strings.TrimSuffix(mirrorBaseURL, "/")
	}

	filename := fmt.Sprintf("npm-%s.tgz", normVer)
	downloadURL := fmt.Sprintf("%s/%s", baseURL, filename)

	installDir := filepath.Join(doguHome, "candidates", "npm", versionSpec)
	cacheFile := filepath.Join(doguHome, "cache", "npm", filename)

	if err := downloader.DownloadAndExtract(ctx, downloadURL, cacheFile, installDir); err != nil {
		return nil, fmt.Errorf("failed to download npm: %w", err)
	}

	npmHome := downloader.FindSubDirWithDir(installDir, "bin")
	binDir := filepath.Join(npmHome, "bin")
	ensureNPMWrappers(binDir)

	majorVer := env.ExtractMajorVersion(normVer)
	envVars := map[string]string{
		env.FormatEnvKey("npm", majorVer): npmHome,
		env.FormatEnvKey("npm", ""):        npmHome,
	}

	return &InstallResult{
		ToolName:      "npm",
		RequestedVer:  versionSpec,
		NormalizedVer: normVer,
		InstallDir:    installDir,
		HomeDir:       npmHome,
		BinDirs:       []string{binDir},
		EnvVars:       envVars,
	}, nil
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
