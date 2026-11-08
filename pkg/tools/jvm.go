package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/dogu-dev/dogu/pkg/downloader"
	"github.com/dogu-dev/dogu/pkg/env"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

type ClojureTool struct{}

func init() {
	Register(&ClojureTool{})
	Register(&LeiningenTool{})
}

func (*ClojureTool) Name() string { return "clojure" }

func fetchClojureVersions(ctx context.Context, baseURL string) ([]string, error) {
	data, err := fetchReleaseData(ctx, strings.TrimRight(baseURL, "/")+"/")
	if err != nil {
		return nil, err
	}
	matches := regexp.MustCompile(`clojure-tools-([0-9]+(?:\.[0-9]+)+(?:-[A-Za-z0-9.-]+)?)\.tar\.gz`).FindAllSubmatch(data, -1)
	var versions []string
	for _, match := range matches {
		versions = append(versions, string(match[1]))
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("no Clojure CLI versions found at %s", baseURL)
	}
	return versions, nil
}

func (*ClojureTool) Install(ctx context.Context, spec, home, mirror string, opts InstallOptions) (*InstallResult, error) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("Clojure CLI installation currently supports Linux and macOS")
	}
	if res, ok, err := checkExistingCandidate(home, "clojure", spec, opts, func(res *InstallResult) error {
		if err := validateClojureLaunchers(res.InstallDir); err != nil {
			return err
		}
		res.NormalizedVer = filepath.Base(res.InstallDir)
		res.HomeDir = res.InstallDir
		res.BinDirs = []string{filepath.Join(res.InstallDir, "bin")}
		res.EnvVars = buildEnvVars("clojure", res.NormalizedVer, res.HomeDir, clojureDefinition.homeEnv, nil)
		return nil
	}); ok || err != nil {
		return res, err
	}

	baseURL := resolveBaseURL(clojureDefinition.baseURL, mirror)
	versions, err := clojureDefinition.source.fetch(ctx, baseURL, mirror)
	if err != nil {
		return nil, fmt.Errorf("resolve clojure version: %w", err)
	}
	version, err := SelectBestVersion(versions, spec, opts.IncludePrerelease)
	if err != nil {
		return nil, err
	}
	version = CleanVersion(version)
	_, path, _, cacheName, _ := clojureDefinition.expand(version, runtime.GOOS, runtime.GOARCH)

	work, err := os.MkdirTemp("", "dogu-clojure-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)
	if err := downloader.DownloadAndExtract(ctx, baseURL+"/"+path, filepath.Join(home, "cache", "clojure", cacheName), work, downloader.WithRewriteRule(opts.RewriteRules)); err != nil {
		return nil, fmt.Errorf("failed to download clojure: %w", err)
	}
	installer, err := findClojureInstaller(work)
	if err != nil {
		return nil, err
	}

	installDir, err := filepath.Abs(filepath.Join(home, "candidates", "clojure", version))
	if err != nil {
		return nil, err
	}
	if opts.Force {
		if err := os.RemoveAll(installDir); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(installDir), 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return nil, err
	}
	installed := false
	defer func() {
		if !installed {
			_ = os.RemoveAll(installDir)
		}
	}()

	cmd := exec.CommandContext(ctx, "sh", installer, installDir)
	cmd.Dir = filepath.Dir(installer)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Clojure CLI setup failed: %w\n%s", err, output)
	}
	if err := validateClojureLaunchers(installDir); err != nil {
		return nil, err
	}
	installed = true

	res := &InstallResult{
		ToolName:      "clojure",
		RequestedVer:  spec,
		NormalizedVer: version,
		InstallDir:    installDir,
		HomeDir:       installDir,
		BinDirs:       []string{filepath.Join(installDir, "bin")},
		EnvVars:       buildEnvVars("clojure", version, installDir, clojureDefinition.homeEnv, nil),
	}
	return res, nil
}

func validateClojureLaunchers(installDir string) error {
	for _, launcher := range []string{"clojure", "clj"} {
		if _, err := os.Stat(filepath.Join(installDir, "bin", launcher)); err != nil {
			return fmt.Errorf("Clojure CLI installation is missing its %s launcher: %w", launcher, err)
		}
	}
	return nil
}

func findClojureInstaller(root string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			path := filepath.Join(root, entry.Name(), "install.sh")
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}
	path := filepath.Join(root, "install.sh")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("Clojure CLI archive %s does not contain install.sh", filepath.Base(root))
}

type LeiningenTool struct{}

func (*LeiningenTool) Name() string { return "leiningen" }

func (*LeiningenTool) Install(ctx context.Context, spec, home, mirror string, opts InstallOptions) (*InstallResult, error) {
	if !opts.Force {
		if dir, ok := findExistingCandidate(home, "leiningen", spec, opts.IncludePrerelease); ok {
			launcher := filepath.Join(dir, "lein")
			if runtime.GOOS == "windows" {
				launcher += ".bat"
			}
			if _, err := os.Stat(launcher); err == nil {
				res := buildResultFromDir("leiningen", spec, dir)
				res.NormalizedVer = filepath.Base(dir)
				res.HomeDir = dir
				res.BinDirs = []string{dir}
				res.EnvVars = buildEnvVars("leiningen", res.NormalizedVer, dir, leiningenDefinition.homeEnv, nil)
				return res, nil
			}
		}
	}

	version, err := leiningenDefinition.resolveVersion(ctx, spec, mirror, opts.IncludePrerelease)
	if err != nil {
		return nil, err
	}
	asset, path, _, cacheName, _ := leiningenDefinition.expand(version, runtime.GOOS, runtime.GOARCH)
	jarName := filepath.Base(asset)
	return installBinaryTool(ctx, BinaryInstallConfig{
		ToolName:      "leiningen",
		VersionSpec:   spec,
		NormalizedVer: version,
		DownloadURL:   resolveBaseURL(leiningenDefinition.baseURL, mirror) + "/" + path,
		CacheFileName: cacheName,
		TargetBin:     jarName,
		DoguHome:      home,
		Opts:          opts,
		ExtraHomeEnv:  leiningenDefinition.homeEnv,
		PostInstall: func(res *InstallResult) error {
			script := fmt.Sprintf("#!/bin/sh\nDIR=$(CDPATH= cd -- \"$(dirname -- \"$0\")\" && pwd)\nexec \"${LEIN_JAVA_CMD:-java}\" ${LEIN_JVM_OPTS:-} -Dfile.encoding=UTF-8 -Dmaven.wagon.http.ssl.easy=false -Dleiningen.original.working.directory=\"$PWD\" -Dleiningen.script=\"$0\" -classpath \"$DIR/%s\" clojure.main -m leiningen.core.main \"$@\"\n", jarName)
			if err := os.WriteFile(filepath.Join(res.InstallDir, "lein"), []byte(script), 0755); err != nil {
				return err
			}
			batch := fmt.Sprintf("@echo off\r\njava %%LEIN_JVM_OPTS%% -Dfile.encoding=UTF-8 -Dmaven.wagon.http.ssl.easy=false -Dleiningen.original.working.directory=\"%%CD%%\" -Dleiningen.script=\"%%~f0\" -classpath \"%%~dp0%s\" clojure.main -m leiningen.core.main %%*\r\n", jarName)
			if err := os.WriteFile(filepath.Join(res.InstallDir, "lein.bat"), []byte(batch), 0644); err != nil {
				return err
			}
			return nil
		},
	})
}

// GradleTool installs gradle using the shared tool definition.
type GradleTool struct{}

func init() { Register(&GradleTool{}) }

func (*GradleTool) Name() string { return "gradle" }

func (*GradleTool) Install(ctx context.Context, versionSpec, doguHome, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	return gradleDefinition.Install(ctx, versionSpec, doguHome, mirrorBaseURL, opts)
}

type GroovyTool struct{}

func init()                      { Register(&GroovyTool{}) }
func (*GroovyTool) Name() string { return "groovy" }
func (*GroovyTool) Install(ctx context.Context, spec, home, mirror string, opts InstallOptions) (*InstallResult, error) {
	d := groovyDefinition
	return d.Install(ctx, spec, home, mirror, opts)
}

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
	EarlyAccess         bool   `json:"early_access"`
	Links               struct {
		PkgDownloadRedirect string `json:"pkg_download_redirect"`
	} `json:"links"`
}

type discoResponse struct {
	Result  []discoPackage `json:"result"`
	Message string         `json:"message"`
}

func (j *JavaTool) Install(ctx context.Context, versionSpec string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	if !opts.Force {
		if existingDir, ok := findExistingCandidate(doguHome, "java", versionSpec, opts.IncludePrerelease); ok {
			return buildJavaResultFromDir(versionSpec, existingDir), nil
		}
	}

	ver, dist := parseJavaSpec(versionSpec)

	osName := mapOS(runtime.GOOS)
	archName := mapArch(runtime.GOARCH)

	downloadURL, filename, normVersion, resolvedDist, err := j.resolveDownloadURL(ctx, ver, dist, osName, archName, mirrorBaseURL, opts.IncludePrerelease)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve java version '%s': %w", versionSpec, err)
	}

	candidateDirName := normVersion
	if resolvedDist != "" {
		candidateDirName = normVersion + "-" + resolvedDist
	}

	return installArchiveTool(ctx, ArchiveInstallConfig{
		ToolName:         "java",
		VersionSpec:      versionSpec,
		NormalizedVer:    normVersion,
		CandidateDirName: candidateDirName,
		DownloadURL:      downloadURL,
		FileName:         filename,
		DoguHome:         doguHome,
		Opts:             opts,
		FindDirs: func(installDir string) (string, []string) {
			jdkHome := findJDKHome(installDir)
			return jdkHome, []string{filepath.Join(jdkHome, "bin")}
		},
	})
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

func (j *JavaTool) resolveDownloadURL(ctx context.Context, ver string, dist string, osName string, archName string, mirrorBaseURL string, includePrerelease bool) (downloadURL string, filename string, normVersion string, resolvedDist string, err error) {
	if mirrorBaseURL != "" && !strings.Contains(mirrorBaseURL, "foojay") {
		if ver == "" {
			_, _, ver, dist, err = j.resolveDownloadURL(ctx, ver, dist, osName, archName, "", includePrerelease)
			if err != nil {
				return "", "", "", "", err
			}
		}
		mirrorBaseURL = strings.TrimSuffix(mirrorBaseURL, "/")
		distSuffix := ""
		if dist != "" {
			distSuffix = "-" + dist
		}
		fn := fmt.Sprintf("java-%s%s-%s-%s.tar.gz", ver, distSuffix, osName, archName)
		url := fmt.Sprintf("%s/%s", mirrorBaseURL, fn)
		return url, fn, ver, dist, nil
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
	if ver != "" {
		v.Set("version", ver)
	}
	v.Set("operating_system", osName)
	v.Set("architecture", archName)
	v.Set("archive_type", archiveType)
	v.Set("package_type", "jdk")

	specIsRC := IsRC(ver)
	if !includePrerelease && !specIsRC {
		v.Set("early_access", "false")
	}

	if dist != "" {
		v.Set("distribution", dist)
	}

	reqURL := fmt.Sprintf("%s?%s", endpoint, v.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return "", "", "", "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", "", "", fmt.Errorf("foojay disco api returned status %s", resp.Status)
	}

	var discoResp discoResponse
	if err := json.NewDecoder(resp.Body).Decode(&discoResp); err != nil {
		return "", "", "", "", err
	}

	var validPkgs []discoPackage
	for _, p := range discoResp.Result {
		if !includePrerelease && !specIsRC {
			if p.EarlyAccess || IsRC(p.JavaVersion) {
				continue
			}
		}
		validPkgs = append(validPkgs, p)
	}

	if len(validPkgs) == 0 {
		return "", "", "", "", fmt.Errorf("no java package found for version=%s, dist=%s, os=%s, arch=%s", ver, dist, osName, archName)
	}

	pkg := validPkgs[0]
	for _, candidate := range validPkgs[1:] {
		if CompareVersions(candidate.JavaVersion, pkg.JavaVersion) > 0 {
			pkg = candidate
		}
	}
	pkgDist := strings.ToLower(pkg.Distribution)
	if pkgDist == "" {
		pkgDist = dist
	}
	redirectURL := pkg.Links.PkgDownloadRedirect
	if mirrorBaseURL != "" && !strings.Contains(mirrorBaseURL, "foojay") && strings.HasPrefix(redirectURL, "https://") {
		if parsedMirror, err := url.Parse(mirrorBaseURL); err == nil && parsedMirror.Host != "" {
			if parsedRedirect, err := url.Parse(redirectURL); err == nil {
				redirectURL = fmt.Sprintf("%s://%s/%s%s", parsedMirror.Scheme, parsedMirror.Host, parsedRedirect.Host, parsedRedirect.RequestURI())
			}
		}
	}
	return redirectURL, pkg.Filename, pkg.JavaVersion, pkgDist, nil
}

func buildJavaResultFromDir(versionSpec string, installDir string) *InstallResult {
	jdkHome := findJDKHome(installDir)
	binDir := filepath.Join(jdkHome, "bin")
	if _, err := os.Stat(binDir); err != nil {
		binDir = jdkHome
	}

	normalizedVer := filepath.Base(installDir)
	ver, _ := parseJavaSpec(normalizedVer)
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
		NormalizedVer: normalizedVer,
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

type KotlinTool struct{}

func init()                      { Register(&KotlinTool{}) }
func (*KotlinTool) Name() string { return "kotlin" }
func (*KotlinTool) Install(ctx context.Context, spec, home, mirror string, opts InstallOptions) (*InstallResult, error) {
	d := kotlinDefinition
	return d.Install(ctx, spec, home, mirror, opts)
}

// MavenTool installs maven using the shared tool definition.
type MavenTool struct{}

func init() { Register(&MavenTool{}) }

func (*MavenTool) Name() string { return "maven" }

func (*MavenTool) Install(ctx context.Context, versionSpec, doguHome, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	return mavenDefinition.Install(ctx, versionSpec, doguHome, mirrorBaseURL, opts)
}

func resolveMavenVersion(ctx context.Context, versionSpec, baseURL string, includePrerelease bool) (string, error) {
	return mavenDefinition.resolveVersion(ctx, versionSpec, baseURL, includePrerelease)
}

// SBTTool installs sbt using the shared tool definition.
type SBTTool struct{}

func init() { Register(&SBTTool{}) }

func (*SBTTool) Name() string { return "sbt" }

func (*SBTTool) Install(ctx context.Context, versionSpec, doguHome, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	return sbtDefinition.Install(ctx, versionSpec, doguHome, mirrorBaseURL, opts)
}

type ScalaTool struct{}

func init()                     { Register(&ScalaTool{}) }
func (*ScalaTool) Name() string { return "scala" }
func (*ScalaTool) Install(ctx context.Context, spec, home, mirror string, opts InstallOptions) (*InstallResult, error) {
	requestedSpec := spec
	if spec == "" {
		if res, ok, err := checkExistingCandidate(home, "scala", spec, opts, nil); ok || err != nil {
			return res, err
		}
		versions, err := listScalaVersions(ctx)
		if err != nil {
			return nil, err
		}
		spec, err = SelectBestVersion(versions, spec, opts.IncludePrerelease)
		if err != nil {
			return nil, err
		}
		spec = CleanVersion(spec)
	}
	d := scalaDefinition
	if strings.HasPrefix(CleanVersion(spec), "3") {
		d.baseURL = "https://github.com/scala/scala3/releases/download"
		d.source.owner, d.source.repository = "scala", "scala3"
		d.asset, d.downloadPath = "scala3-{version}.zip", "{version}/{asset}"
	}
	res, err := d.Install(ctx, spec, home, mirror, opts)
	if res != nil {
		res.RequestedVer = requestedSpec
	}
	return res, err
}
