package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dogu-dev/dogu/pkg/downloader"
	"github.com/dogu-dev/dogu/pkg/env"
)

type JavaTool struct{}

func init() {
	Register(&JavaTool{})
}

func (j *JavaTool) Name() string {
	return "java"
}

type discoPackage struct {
	ID                  string `json:"id"`
	ArchiveType         string `json:"archive_type"`
	Distribution        string `json:"distribution"`
	MajorVersion        int    `json:"major_version"`
	JavaVersion         string `json:"java_version"`
	DistributionVersion string `json:"distribution_version"`
	Filename            string `json:"filename"`
	Links               struct {
		PkgDownloadRedirect string `json:"pkg_download_redirect"`
	} `json:"links"`
}

type discoResponse struct {
	Result  []discoPackage `json:"result"`
	Message string         `json:"message"`
}

func (j *JavaTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, force bool) (*InstallResult, error) {
	if !force {
		if existingDir, ok := findExistingCandidate(doguHome, "java", versionSpec); ok {
			return buildJavaResultFromDir(versionSpec, existingDir), nil
		}
	}

	ver, dist := parseJavaSpec(versionSpec)

	osName := mapOS(runtime.GOOS)
	archName := mapArch(runtime.GOARCH)

	downloadURL, filename, normVersion, err := j.resolveDownloadURL(ctx, ver, dist, osName, archName, mirrorBaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve java version '%s': %w", versionSpec, err)
	}

	installDir := filepath.Join(doguHome, "candidates", "java", versionSpec)
	cacheFile := filepath.Join(doguHome, "cache", "java", filename)

	if err := downloader.DownloadAndExtract(ctx, downloadURL, cacheFile, installDir); err != nil {
		return nil, fmt.Errorf("failed to download and extract java: %w", err)
	}

	jdkHome := findJDKHome(installDir)
	binDir := filepath.Join(jdkHome, "bin")

	majorVersion := env.ExtractMajorVersion(ver)
	majorKey := env.FormatEnvKey("java", majorVersion)
	javaHomeKey := env.FormatEnvKey("java", "")

	envVars := map[string]string{
		majorKey:    jdkHome,
		javaHomeKey: jdkHome,
	}

	return &InstallResult{
		ToolName:      "java",
		RequestedVer:  versionSpec,
		NormalizedVer: normVersion,
		InstallDir:    installDir,
		HomeDir:       jdkHome,
		BinDirs:       []string{binDir},
		EnvVars:       envVars,
	}, nil
}

func parseJavaSpec(spec string) (version string, distribution string) {
	parts := strings.SplitN(spec, "-", 2)
	version = parts[0]
	if len(parts) > 1 {
		distribution = parts[1]
	}

	if strings.HasSuffix(version, "-tem") || distribution == "tem" || distribution == "temurin" {
		version = strings.TrimSuffix(version, "-tem")
		distribution = "temurin"
	} else if strings.HasSuffix(version, "-zulu") || distribution == "zulu" {
		version = strings.TrimSuffix(version, "-zulu")
		distribution = "zulu"
	}
	return version, distribution
}

func (j *JavaTool) resolveDownloadURL(ctx context.Context, ver string, dist string, osName string, archName string, mirrorBaseURL string) (downloadURL string, filename string, normVersion string, err error) {
	if mirrorBaseURL != "" && !strings.Contains(mirrorBaseURL, "foojay") {
		mirrorBaseURL = strings.TrimSuffix(mirrorBaseURL, "/")
		distSuffix := ""
		if dist != "" {
			distSuffix = "-" + dist
		}
		fn := fmt.Sprintf("java-%s%s-%s-%s.tar.gz", ver, distSuffix, osName, archName)
		url := fmt.Sprintf("%s/%s", mirrorBaseURL, fn)
		return url, fn, ver, nil
	}

	archiveType := "tar.gz"
	if runtime.GOOS == "windows" {
		archiveType = "zip"
	}

	endpoint := "https://api.foojay.io/disco/v3.0/packages"
	if mirrorBaseURL != "" && strings.Contains(mirrorBaseURL, "foojay") {
		endpoint = strings.TrimSuffix(mirrorBaseURL, "/")
	}
	v := url.Values{}
	v.Set("version", ver)
	v.Set("operating_system", osName)
	v.Set("architecture", archName)
	v.Set("archive_type", archiveType)
	v.Set("package_type", "jdk")

	if dist != "" {
		v.Set("distribution", dist)
	}

	reqURL := fmt.Sprintf("%s?%s", endpoint, v.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return "", "", "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("foojay disco api returned status %s", resp.Status)
	}

	var discoResp discoResponse
	if err := json.NewDecoder(resp.Body).Decode(&discoResp); err != nil {
		return "", "", "", err
	}

	if len(discoResp.Result) == 0 {
		return "", "", "", fmt.Errorf("no java package found for version=%s, dist=%s, os=%s, arch=%s", ver, dist, osName, archName)
	}

	pkg := discoResp.Result[0]
	redirectURL := pkg.Links.PkgDownloadRedirect
	if mirrorBaseURL != "" && !strings.Contains(mirrorBaseURL, "foojay") && strings.HasPrefix(redirectURL, "https://") {
		if parsedMirror, err := url.Parse(mirrorBaseURL); err == nil && parsedMirror.Host != "" {
			if parsedRedirect, err := url.Parse(redirectURL); err == nil {
				redirectURL = fmt.Sprintf("%s://%s/%s%s", parsedMirror.Scheme, parsedMirror.Host, parsedRedirect.Host, parsedRedirect.RequestURI())
			}
		}
	}
	return redirectURL, pkg.Filename, pkg.JavaVersion, nil
}

func buildJavaResultFromDir(versionSpec string, installDir string) *InstallResult {
	jdkHome := findJDKHome(installDir)
	binDir := filepath.Join(jdkHome, "bin")
	if _, err := os.Stat(binDir); err != nil {
		binDir = jdkHome
	}

	ver, _ := parseJavaSpec(versionSpec)
	majorVersion := env.ExtractMajorVersion(ver)
	majorKey := env.FormatEnvKey("java", majorVersion)
	javaHomeKey := env.FormatEnvKey("java", "")

	envVars := map[string]string{
		majorKey:    jdkHome,
		javaHomeKey: jdkHome,
	}

	return &InstallResult{
		ToolName:      "java",
		RequestedVer:  versionSpec,
		NormalizedVer: versionSpec,
		InstallDir:    installDir,
		HomeDir:       jdkHome,
		BinDirs:       []string{binDir},
		EnvVars:       envVars,
	}
}

func findJDKHome(baseDir string) string {
	macHome := filepath.Join(baseDir, "Contents", "Home")
	if fi, err := os.Stat(filepath.Join(macHome, "bin")); err == nil && fi.IsDir() {
		return macHome
	}

	if fi, err := os.Stat(filepath.Join(baseDir, "bin")); err == nil && fi.IsDir() {
		return baseDir
	}

	entries, err := os.ReadDir(baseDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				sub := filepath.Join(baseDir, entry.Name())
				macSubHome := filepath.Join(sub, "Contents", "Home")
				if fi, err := os.Stat(filepath.Join(macSubHome, "bin")); err == nil && fi.IsDir() {
					return macSubHome
				}
				if fi, err := os.Stat(filepath.Join(sub, "bin")); err == nil && fi.IsDir() {
					return sub
				}
			}
		}
	}

	return baseDir
}

func mapOS(goos string) string {
	switch goos {
	case "darwin":
		return "macos"
	case "windows":
		return "windows"
	default:
		return "linux"
	}
}

func mapArch(goarch string) string {
	switch goarch {
	case "amd64":
		return "x64"
	case "arm64":
		return "aarch64"
	default:
		return goarch
	}
}
