package tools

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/charmbracelet/log"
	"github.com/dogu-dev/dogu/pkg/downloader"
	"github.com/dogu-dev/dogu/pkg/env"
)

type CommonTool struct {
	toolName string
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
