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
)

type PantsTool struct{}

func init() {
	Register(&PantsTool{})
}

func (p *PantsTool) Name() string {
	return "pants"
}

func (p *PantsTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if res, ok, err := checkExistingCandidate(doguHome, "pants", versionSpec, opts, nil); ok || err != nil {
		return res, err
	}

	baseURL := resolveBaseURL("https://github.com/pantsbuild/pants/releases/download", mirrorBaseURL)
	normVer := resolvePantsVersion(ctx, versionSpec, opts.IncludePrerelease)

	osName, archName := mapPantsTarget()
	targetBin := "pants"
	if osName == "windows" {
		targetBin = "pants.exe"
	}

	downloadURL := resolvePantsDownloadURL(ctx, baseURL, normVer, osName, archName)
	cacheFileName := filepath.Base(downloadURL)

	return installBinaryTool(ctx, BinaryInstallConfig{
		ToolName:      "pants",
		VersionSpec:   versionSpec,
		NormalizedVer: normVer,
		DownloadURL:   downloadURL,
		CacheFileName: cacheFileName,
		TargetBin:     targetBin,
		DoguHome:      doguHome,
		Opts:          opts,
		ExtraHomeEnv:  []string{"PANTS_HOME"},
	})
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
