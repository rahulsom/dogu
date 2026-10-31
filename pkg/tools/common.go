package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/dogu-dev/dogu/pkg/downloader"
	"github.com/dogu-dev/dogu/pkg/env"
)

type CommonTool struct {
	toolName string
}

func resolveBaseURL(defaultURL, mirrorBaseURL string) string {
	if mirrorBaseURL != "" {
		return strings.TrimSuffix(mirrorBaseURL, "/")
	}
	return defaultURL
}

func checkExistingCandidate(doguHome string, toolName string, versionSpec string, opts InstallOptions, postExisting func(*InstallResult) error) (*InstallResult, bool, error) {
	if !opts.Force {
		if existingDir, ok := findExistingCandidate(doguHome, toolName, versionSpec, opts.IncludePrerelease); ok {
			res := buildResultFromDir(toolName, versionSpec, existingDir)
			if postExisting != nil {
				if err := postExisting(res); err != nil {
					return nil, false, err
				}
			}
			return res, true, nil
		}
	}
	return nil, false, nil
}

func buildEnvVars(toolName string, versionForMajor string, homeDir string, extraHomeEnv []string, extraEnv map[string]string) map[string]string {
	majorVer := env.ExtractMajorVersion(versionForMajor)
	envVars := map[string]string{
		env.FormatEnvKey(toolName, majorVer): homeDir,
		env.FormatEnvKey(toolName, ""):        homeDir,
	}
	for _, homeKey := range extraHomeEnv {
		envVars[homeKey] = homeDir
	}
	for k, v := range extraEnv {
		envVars[k] = v
	}
	return envVars
}

// ArchiveInstallConfig defines configuration for downloading and extracting an archive tool.
type ArchiveInstallConfig struct {
	ToolName         string
	VersionSpec      string
	NormalizedVer    string
	CandidateDirName string // Defaults to NormalizedVer if empty
	DownloadURL      string
	FileName         string
	DoguHome         string
	Opts             InstallOptions
	ExtraHomeEnv     []string
	ExtraEnv         map[string]string
	FindDirs         func(installDir string) (homeDir string, binDirs []string)
	PostInstall      func(result *InstallResult) error
}

func installArchiveTool(ctx context.Context, cfg ArchiveInstallConfig) (*InstallResult, error) {
	dirName := cfg.CandidateDirName
	if dirName == "" {
		dirName = cfg.NormalizedVer
	}
	installDir := filepath.Join(cfg.DoguHome, "candidates", cfg.ToolName, dirName)
	cacheFile := filepath.Join(cfg.DoguHome, "cache", cfg.ToolName, cfg.FileName)

	if err := downloader.DownloadAndExtract(ctx, cfg.DownloadURL, cacheFile, installDir, downloader.WithRewriteRule(cfg.Opts.RewriteRules)); err != nil {
		return nil, fmt.Errorf("failed to download %s: %w", cfg.ToolName, err)
	}

	var homeDir string
	var binDirs []string
	if cfg.FindDirs != nil {
		homeDir, binDirs = cfg.FindDirs(installDir)
	} else {
		homeDir = downloader.FindSubDirWithDir(installDir, "bin")
		binDirs = []string{filepath.Join(homeDir, "bin")}
	}

	res := &InstallResult{
		ToolName:      cfg.ToolName,
		RequestedVer:  cfg.VersionSpec,
		NormalizedVer: dirName,
		InstallDir:    installDir,
		HomeDir:       homeDir,
		BinDirs:       binDirs,
		EnvVars:       buildEnvVars(cfg.ToolName, cfg.NormalizedVer, homeDir, cfg.ExtraHomeEnv, cfg.ExtraEnv),
	}

	if cfg.PostInstall != nil {
		if err := cfg.PostInstall(res); err != nil {
			return nil, err
		}
	}

	return res, nil
}

// BinaryInstallConfig defines configuration for downloading a single-binary tool.
type BinaryInstallConfig struct {
	ToolName         string
	VersionSpec      string
	NormalizedVer    string
	CandidateDirName string // Defaults to NormalizedVer if empty
	DownloadURL      string
	CacheFileName    string
	TargetBin        string
	DoguHome         string
	Opts             InstallOptions
	ExtraHomeEnv     []string
	ExtraEnv         map[string]string
	PostInstall      func(result *InstallResult) error
}

func installBinaryTool(ctx context.Context, cfg BinaryInstallConfig) (*InstallResult, error) {
	dirName := cfg.CandidateDirName
	if dirName == "" {
		dirName = cfg.NormalizedVer
	}
	installDir := filepath.Join(cfg.DoguHome, "candidates", cfg.ToolName, dirName)
	cacheFile := filepath.Join(cfg.DoguHome, "cache", cfg.ToolName, cfg.CacheFileName)

	if err := downloader.SaveSingleBinary(ctx, cfg.DownloadURL, cacheFile, installDir, cfg.TargetBin, downloader.WithRewriteRule(cfg.Opts.RewriteRules)); err != nil {
		return nil, fmt.Errorf("failed to download %s: %w", cfg.ToolName, err)
	}

	res := &InstallResult{
		ToolName:      cfg.ToolName,
		RequestedVer:  cfg.VersionSpec,
		NormalizedVer: dirName,
		InstallDir:    installDir,
		HomeDir:       installDir,
		BinDirs:       []string{installDir},
		EnvVars:       buildEnvVars(cfg.ToolName, cfg.NormalizedVer, installDir, cfg.ExtraHomeEnv, cfg.ExtraEnv),
	}

	if cfg.PostInstall != nil {
		if err := cfg.PostInstall(res); err != nil {
			return nil, err
		}
	}

	return res, nil
}

func findDirWithBinary(installDir, binName string) (string, []string) {
	home := downloader.FindSubDirWithDir(installDir, binName)
	if fi, err := filepath.Glob(filepath.Join(installDir, "*", binName)); err == nil && len(fi) > 0 {
		home = filepath.Dir(fi[0])
	}
	return home, []string{home}
}

func mapTargetTriple(linuxVariant string) (arch string, sys string) {
	arch = "x86_64"
	if runtime.GOARCH == "arm64" {
		arch = "aarch64"
	}
	sys = "unknown-linux-" + linuxVariant
	switch runtime.GOOS {
	case "darwin":
		sys = "apple-darwin"
	case "windows":
		sys = "pc-windows-msvc"
	}
	return arch, sys
}

func findExistingCandidate(doguHome string, toolName string, versionSpec string, includePrerelease bool) (string, bool) {
	log.Debug("Searching for existing candidate", "tool", toolName, "spec", versionSpec)
	candidatesDir := filepath.Join(doguHome, "candidates", toolName)
	entries, err := os.ReadDir(candidatesDir)
	if err != nil {
		return "", false
	}

	exactDir := filepath.Join(candidatesDir, versionSpec)
	if fi, err := os.Stat(exactDir); err == nil && fi.IsDir() {
		if subEntries, _ := os.ReadDir(exactDir); len(subEntries) > 0 {
			log.Debug("Found candidate directory", "tool", toolName, "dir", exactDir)
			return exactDir, true
		}
	}

	specIsRC := IsRC(versionSpec)
	var matchingDirs []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if MatchVersion(name, versionSpec) {
			if !includePrerelease && !specIsRC && IsRC(name) {
				continue
			}
			dirPath := filepath.Join(candidatesDir, name)
			if subEntries, _ := os.ReadDir(dirPath); len(subEntries) > 0 {
				matchingDirs = append(matchingDirs, name)
			}
		}
	}

	if len(matchingDirs) == 0 {
		return "", false
	}

	sort.Slice(matchingDirs, func(i, j int) bool {
		return CompareVersions(matchingDirs[i], matchingDirs[j]) < 0
	})

	bestDir := filepath.Join(candidatesDir, matchingDirs[len(matchingDirs)-1])
	log.Debug("Found matching candidate directory", "tool", toolName, "dir", bestDir)
	return bestDir, true
}

func buildResultFromDir(toolName string, versionSpec string, installDir string) *InstallResult {
	binDir := filepath.Join(installDir, "bin")
	homeDir := installDir

	if fi, err := os.Stat(binDir); err == nil && fi.IsDir() {
		homeDir = installDir
	} else {
		homeDir = downloader.FindSubDirWithDir(installDir, "bin")
		binDir = filepath.Join(homeDir, "bin")
		if _, err := os.Stat(binDir); err != nil {
			binDir = homeDir
		}
	}

	majorVer := env.ExtractMajorVersion(versionSpec)
	envVars := map[string]string{
		env.FormatEnvKey(toolName, majorVer): homeDir,
		env.FormatEnvKey(toolName, ""):        homeDir,
	}

	return &InstallResult{
		ToolName:      toolName,
		RequestedVer:  versionSpec,
		NormalizedVer: versionSpec,
		InstallDir:    installDir,
		HomeDir:       homeDir,
		BinDirs:       []string{binDir},
		EnvVars:       envVars,
	}
}
