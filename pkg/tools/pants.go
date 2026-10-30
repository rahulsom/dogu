package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dogu-dev/dogu/pkg/downloader"
	"github.com/dogu-dev/dogu/pkg/env"
)

type PantsTool struct{}

func init() {
	Register(&PantsTool{})
}

func (p *PantsTool) Name() string {
	return "pants"
}

func (p *PantsTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if !opts.Force {
		if existingDir, ok := findExistingCandidate(doguHome, "pants", versionSpec, opts.IncludePrerelease); ok {
			return buildResultFromDir("pants", versionSpec, existingDir), nil
		}
	}

	baseURL := "https://github.com/pantsbuild/pants/releases/download"
	if mirrorBaseURL != "" {
		baseURL = strings.TrimSuffix(mirrorBaseURL, "/")
	}

	normVer := resolvePantsVersion(ctx, versionSpec, opts.IncludePrerelease)

	osName, archName := mapPantsTarget()
	targetBin := "pants"
	if osName == "windows" {
		targetBin = "pants.exe"
	}

	downloadURL := resolvePantsDownloadURL(ctx, baseURL, normVer, osName, archName)

	cacheFileName := filepath.Base(downloadURL)
	installDir := filepath.Join(doguHome, "candidates", "pants", normVer)
	cacheFile := filepath.Join(doguHome, "cache", "pants", cacheFileName)

	if err := downloader.SaveSingleBinary(ctx, downloadURL, cacheFile, installDir, targetBin, downloader.WithRewriteRule(opts.RewriteRules)); err != nil {
		return nil, fmt.Errorf("failed to download pants: %w", err)
	}

	majorVer := env.ExtractMajorVersion(normVer)
	envVars := map[string]string{
		env.FormatEnvKey("pants", majorVer): installDir,
		env.FormatEnvKey("pants", ""):        installDir,
		"PANTS_HOME":                         installDir,
	}

	return &InstallResult{
		ToolName:      "pants",
		RequestedVer:  versionSpec,
		NormalizedVer: normVer,
		InstallDir:    installDir,
		HomeDir:       installDir,
		BinDirs:       []string{installDir},
		EnvVars:       envVars,
	}, nil
}

func resolvePantsVersion(ctx context.Context, versionSpec string, includePrerelease bool) string {
	if versions, err := FetchGitHubReleases(ctx, "pantsbuild", "pants"); err == nil && len(versions) > 0 {
		if best, err := SelectBestVersion(versions, versionSpec, includePrerelease); err == nil {
			return CleanVersion(best)
		}
	}
	if versions, err := FetchPyPIVersions(ctx, "pantsbuild.pants"); err == nil && len(versions) > 0 {
		if best, err := SelectBestVersion(versions, versionSpec, includePrerelease); err == nil {
			return CleanVersion(best)
		}
	}
	return CleanVersion(versionSpec)
}

func resolvePantsDownloadURL(ctx context.Context, baseURL string, normVer string, osName string, archName string) string {
	reqURL := fmt.Sprintf("https://api.github.com/repos/pantsbuild/pants/releases/tags/release_%s", normVer)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err == nil {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var releaseData struct {
				Assets []struct {
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
				} `json:"assets"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&releaseData); err == nil {
				targetSuffix := fmt.Sprintf("-%s_%s", osName, archName)
				targetSuffixPex := fmt.Sprintf("-%s_%s.pex", osName, archName)

				for _, a := range releaseData.Assets {
					if strings.HasSuffix(a.Name, targetSuffix) || strings.HasSuffix(a.Name, targetSuffixPex) {
						return fmt.Sprintf("%s/release_%s/%s", baseURL, normVer, a.Name)
					}
				}
			}
		} else if resp != nil {
			resp.Body.Close()
		}
	}

	binName := fmt.Sprintf("pants.%s-cp314-%s_%s", normVer, osName, archName)
	return fmt.Sprintf("%s/release_%s/%s", baseURL, normVer, binName)
}

func mapPantsTarget() (string, string) {
	osName := "linux"
	switch runtime.GOOS {
	case "darwin":
		osName = "darwin"
	case "windows":
		osName = "windows"
	}

	archName := "x86_64"
	if runtime.GOARCH == "arm64" {
		archName = "arm64"
		if runtime.GOOS == "linux" {
			archName = "aarch64"
		}
	}

	return osName, archName
}
