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

func checkExistingCandidate(ctx context.Context, doguHome string, toolName string, versionSpec string, opts InstallOptions, postExisting func(*InstallResult) error) (*InstallResult, bool, error) {
	if !opts.Force {
		if existingDir, unlock, ok, err := lockExistingCandidate(ctx, doguHome, toolName, versionSpec, opts.IncludePrerelease); err != nil {
			return nil, false, err
		} else if ok {
			defer unlock()
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
	unlock, err := lockCandidate(ctx, cfg.DoguHome, cfg.ToolName, dirName)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if !cfg.Opts.Force && candidateComplete(installDir) {
		return archiveInstallResult(cfg, dirName, installDir), nil
	}
	if err := os.MkdirAll(cfg.DoguHome, 0755); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(cfg.DoguHome, ".dogu-install-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	if err := os.Chmod(stage, 0755); err != nil {
		return nil, err
	}

	if err := downloader.DownloadAndExtract(ctx, cfg.DownloadURL, cacheFile, stage, downloader.WithRewriteRule(cfg.Opts.RewriteRules)); err != nil {
		if cleanupErr := os.Remove(cacheFile); cleanupErr != nil && !os.IsNotExist(cleanupErr) {
			err = errors.Join(err, fmt.Errorf("remove failed archive cache: %w", cleanupErr))
		}
		return nil, fmt.Errorf("failed to download %s: %w", cfg.ToolName, err)
	}

	res := archiveInstallResult(cfg, dirName, stage)

	if cfg.PostInstall != nil {
		if err := cfg.PostInstall(res); err != nil {
			return nil, err
		}
	}

	if err := publishCandidate(stage, installDir); err != nil {
		return nil, err
	}
	relocateInstallResult(res, stage, installDir)
	return res, nil
}

func archiveInstallResult(cfg ArchiveInstallConfig, dirName, installDir string) *InstallResult {
	var homeDir string
	var binDirs []string
	if cfg.FindDirs != nil {
		homeDir, binDirs = cfg.FindDirs(installDir)
	} else {
		homeDir = downloader.FindSubDirWithDir(installDir, "bin")
		binDirs = []string{filepath.Join(homeDir, "bin")}
	}
	return &InstallResult{
		ToolName:      cfg.ToolName,
		RequestedVer:  cfg.VersionSpec,
		NormalizedVer: dirName,
		InstallDir:    installDir,
		HomeDir:       homeDir,
		BinDirs:       binDirs,
		EnvVars:       buildEnvVars(cfg.ToolName, cfg.NormalizedVer, homeDir, cfg.ExtraHomeEnv, cfg.ExtraEnv),
	}
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
	unlock, err := lockCandidate(ctx, cfg.DoguHome, cfg.ToolName, dirName)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if !cfg.Opts.Force && candidateComplete(installDir) {
		return binaryInstallResult(cfg, dirName, installDir), nil
	}
	if err := os.MkdirAll(cfg.DoguHome, 0755); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(cfg.DoguHome, ".dogu-install-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	if err := os.Chmod(stage, 0755); err != nil {
		return nil, err
	}

	if err := downloader.SaveSingleBinary(ctx, cfg.DownloadURL, cacheFile, stage, cfg.TargetBin, downloader.WithRewriteRule(cfg.Opts.RewriteRules)); err != nil {
		return nil, fmt.Errorf("failed to download %s: %w", cfg.ToolName, err)
	}

	res := binaryInstallResult(cfg, dirName, stage)

	if cfg.PostInstall != nil {
		if err := cfg.PostInstall(res); err != nil {
			return nil, err
		}
	}

	if err := publishCandidate(stage, installDir); err != nil {
		return nil, err
	}
	relocateInstallResult(res, stage, installDir)
	return res, nil
}

func binaryInstallResult(cfg BinaryInstallConfig, dirName, installDir string) *InstallResult {
	return &InstallResult{
		ToolName:      cfg.ToolName,
		RequestedVer:  cfg.VersionSpec,
		NormalizedVer: dirName,
		InstallDir:    installDir,
		HomeDir:       installDir,
		BinDirs:       []string{installDir},
		EnvVars:       buildEnvVars(cfg.ToolName, cfg.NormalizedVer, installDir, cfg.ExtraHomeEnv, cfg.ExtraEnv),
	}
}

func relocateInstallResult(res *InstallResult, from, to string) {
	relocate := func(path string) string {
		if path == from {
			return to
		}
		if strings.HasPrefix(path, from+string(os.PathSeparator)) {
			return filepath.Join(to, strings.TrimPrefix(path, from+string(os.PathSeparator)))
		}
		return path
	}
	res.InstallDir = relocate(res.InstallDir)
	res.HomeDir = relocate(res.HomeDir)
	for i, dir := range res.BinDirs {
		res.BinDirs[i] = relocate(dir)
	}
	for key, value := range res.EnvVars {
		res.EnvVars[key] = relocate(value)
	}
}

func publishCandidate(stage, dest string) error {
	if err := os.WriteFile(filepath.Join(stage, ".dogu-incomplete"), nil, 0644); err != nil {
		return err
	}
	if err := os.Chmod(stage, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	backup, err := os.MkdirTemp(filepath.Dir(dest), ".dogu-backup-")
	if err != nil {
		return err
	}
	if err := os.Remove(backup); err != nil {
		return err
	}
	hadPrevious := false
	if _, err := os.Lstat(dest); err == nil {
		if err := os.Rename(dest, backup); err != nil {
			return err
		}
		hadPrevious = true
	} else if !os.IsNotExist(err) {
		return err
	}
	restore := func() error {
		removeErr := os.RemoveAll(dest)
		if hadPrevious {
			return errors.Join(removeErr, os.Rename(backup, dest))
		}
		return removeErr
	}
	if err := os.Rename(stage, dest); err != nil {
		return errors.Join(err, restore())
	}
	if err := completeCandidate(dest); err != nil {
		return errors.Join(err, restore())
	}
	if hadPrevious {
		if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("remove previous candidate backup: %w", err)
		}
	}
	return nil
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
	if _, isDiagram := diagramDefinition(filepath.Base(filepath.Dir(dir))); isDiagram {
		if _, err := os.Stat(filepath.Join(dir, ".dogu-complete")); err != nil {
			return false
		}
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
	hasBuildMetadata := strings.Contains(versionSpec, "+")
	if toolName == "java" {
		version, _ = parseJavaSpec(version)
	}
	if versionSpec != "" && (hasBuildMetadata || !partialVersionRegex.MatchString(version) || IsRC(versionSpec)) {
		exactDir := filepath.Join(candidatesDir, versionSpec)
		if fi, err := os.Stat(exactDir); err == nil && fi.IsDir() {
			if candidateComplete(exactDir) {
				return exactDir, true
			}
		}
	}
	if hasBuildMetadata {
		return "", false
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
			if requestedDist != "" && normalizeJavaDistribution(requestedDist) != normalizeJavaDistribution(candidateDist) {
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

func lockCandidate(ctx context.Context, doguHome, toolName, dirName string) (func(), error) {
	candidatesDir := filepath.Join(doguHome, "candidates", toolName)
	if err := os.MkdirAll(candidatesDir, 0755); err != nil {
		return nil, err
	}
	lockPath := filepath.Join(candidatesDir, "."+filepath.Base(dirName)+".lock")
	return acquireFileLock(ctx, lockPath)
}

func lockExistingCandidate(ctx context.Context, doguHome, toolName, versionSpec string, includePrerelease bool) (string, func(), bool, error) {
	for {
		dir, ok := findExistingCandidate(doguHome, toolName, versionSpec, includePrerelease)
		if !ok {
			return "", nil, false, nil
		}
		unlock, err := lockCandidate(ctx, doguHome, toolName, filepath.Base(dir))
		if err != nil {
			return "", nil, false, err
		}
		current, found := findExistingCandidate(doguHome, toolName, versionSpec, includePrerelease)
		if !found {
			unlock()
			return "", nil, false, nil
		}
		if current != dir {
			unlock()
			continue
		}
		return dir, unlock, true, nil
	}
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
