package tools

import (
	"context"
	"errors"
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
		env.FormatEnvKey(toolName, ""):       homeDir,
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
	cacheFile := filepath.Join(cfg.DoguHome, "cache", cfg.ToolName, dirName, cfg.FileName)
	if err := os.MkdirAll(cfg.DoguHome, 0755); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(cfg.DoguHome, ".dogu-install-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)

	if err := downloader.DownloadAndExtract(ctx, cfg.DownloadURL, cacheFile, stage, downloader.WithRewriteRule(cfg.Opts.RewriteRules)); err != nil {
		if cleanupErr := os.Remove(cacheFile); cleanupErr != nil && !os.IsNotExist(cleanupErr) {
			err = errors.Join(err, fmt.Errorf("remove failed archive cache: %w", cleanupErr))
		}
		return nil, fmt.Errorf("failed to download %s: %w", cfg.ToolName, err)
	}
	if err := publishCandidate(stage, installDir); err != nil {
		return nil, err
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
			return nil, errors.Join(err, os.RemoveAll(installDir))
		}
	}

	if err := completeCandidate(installDir); err != nil {
		return nil, errors.Join(err, os.RemoveAll(installDir))
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
	cacheFile := filepath.Join(cfg.DoguHome, "cache", cfg.ToolName, dirName, cfg.CacheFileName)
	if err := os.MkdirAll(cfg.DoguHome, 0755); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(cfg.DoguHome, ".dogu-install-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)

	if err := downloader.SaveSingleBinary(ctx, cfg.DownloadURL, cacheFile, stage, cfg.TargetBin, downloader.WithRewriteRule(cfg.Opts.RewriteRules)); err != nil {
		return nil, fmt.Errorf("failed to download %s: %w", cfg.ToolName, err)
	}
	if err := publishCandidate(stage, installDir); err != nil {
		return nil, err
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
			return nil, errors.Join(err, os.RemoveAll(installDir))
		}
	}

	if err := completeCandidate(installDir); err != nil {
		return nil, errors.Join(err, os.RemoveAll(installDir))
	}

	return res, nil
}

func publishCandidate(stage, dest string) error {
	if err := os.WriteFile(filepath.Join(stage, ".dogu-incomplete"), nil, 0644); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	return os.Rename(stage, dest)
}

func completeCandidate(dir string) error {
	if err := os.WriteFile(filepath.Join(dir, ".dogu-complete"), nil, 0644); err != nil {
		return err
	}
	return os.Remove(filepath.Join(dir, ".dogu-incomplete"))
}

func candidateComplete(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, ".dogu-incomplete")); err == nil {
		return false
	}
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}

func findDirWithBinary(installDir, binName string) (string, []string) {
	if fi, err := os.Stat(filepath.Join(installDir, binName)); err == nil && !fi.IsDir() {
		return installDir, []string{installDir}
	}
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

	version := CleanVersion(versionSpec)
	if toolName == "java" {
		version, _ = parseJavaSpec(version)
	}
	if versionSpec != "" && (!partialVersionRegex.MatchString(version) || IsRC(versionSpec)) {
		exactDir := filepath.Join(candidatesDir, versionSpec)
		if fi, err := os.Stat(exactDir); err == nil && fi.IsDir() {
			if candidateComplete(exactDir) {
				return exactDir, true
			}
		}
	}

	specIsRC := IsRC(versionSpec)
	var matchingDirs []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if toolName == "java" {
			_, requestedDist := parseJavaSpec(versionSpec)
			_, candidateDist := parseJavaSpec(name)
			if requestedDist != "" && requestedDist != candidateDist {
				continue
			}
		}
		if MatchVersion(name, versionSpec) {
			if !includePrerelease && !specIsRC && IsRC(name) {
				continue
			}
			dirPath := filepath.Join(candidatesDir, name)
			if candidateComplete(dirPath) {
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

	normalizedVer := filepath.Base(installDir)
	if toolName == "bun" || toolName == "uv" {
		binary := toolName
		if runtime.GOOS == "windows" {
			binary += ".exe"
		}
		homeDir, _ = findDirWithBinary(installDir, binary)
		binDir = homeDir
	}
	majorVer := env.ExtractMajorVersion(normalizedVer)
	envVars := map[string]string{
		env.FormatEnvKey(toolName, majorVer): homeDir,
		env.FormatEnvKey(toolName, ""):       homeDir,
	}

	return &InstallResult{
		ToolName:      toolName,
		RequestedVer:  versionSpec,
		NormalizedVer: normalizedVer,
		InstallDir:    installDir,
		HomeDir:       homeDir,
		BinDirs:       []string{binDir},
		EnvVars:       envVars,
	}
}
