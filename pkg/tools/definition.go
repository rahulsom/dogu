package tools

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dogu-dev/dogu/pkg/downloader"
)

type versionSourceKind uint8

const (
	apacheAntSource versionSourceKind = iota
	apacheMavenSource
	clojureSource
	gradleSource
	githubSource
	npmSource
	indexSource
)

type versionSource struct {
	kind                           versionSourceKind
	owner, repository, packageName string
}

func (s versionSource) fetch(ctx context.Context, baseURL, mirrorURL string) ([]string, error) {
	switch s.kind {
	case apacheAntSource:
		return FetchApacheAntVersions(ctx, baseURL)
	case apacheMavenSource:
		return FetchApacheMavenVersions(ctx, baseURL)
	case clojureSource:
		return fetchClojureVersions(ctx, baseURL)
	case gradleSource:
		return FetchGradleVersions(ctx, mirrorURL)
	case githubSource:
		return FetchGitHubReleases(ctx, s.owner, s.repository)
	case indexSource:
		return fetchIndexVersions(ctx, baseURL)
	case npmSource:
		return FetchNPMVersions(ctx, s.packageName, mirrorURL)
	default:
		panic("unknown tool version source")
	}
}

type platformMapping struct {
	arch, arm64            string
	linux, darwin, windows string
}

type toolDefinition struct {
	name, baseURL               string
	source                      versionSource
	asset, downloadPath         string
	extension, windowsExtension string
	// A nonempty binary selects single-binary installation; otherwise use an archive.
	binary, windowsSuffix, cacheName string
	platform                         platformMapping
	homeEnv                          []string
	extraEnv                         map[string]string
}

func (d toolDefinition) resolveVersion(ctx context.Context, spec, mirrorURL string, prerelease bool) (string, error) {
	versions, err := d.source.fetch(ctx, resolveBaseURL(d.baseURL, mirrorURL), mirrorURL)
	return resolveDiscoveredVersion(versions, err, spec, prerelease)
}

// expand returns the naming rules for a specific platform without consulting the
// host, so all supported platforms can be checked in tests.
func (d toolDefinition) expand(version, goos, goarch string) (asset, path, binary, cache string, extraEnv map[string]string) {
	osName := d.platform.linux
	ext, suffix := d.extension, ""
	switch goos {
	case "darwin":
		osName = d.platform.darwin
	case "windows":
		osName, suffix = d.platform.windows, d.windowsSuffix
		if d.windowsExtension != "" {
			ext = d.windowsExtension
		}
	}
	arch := d.platform.arch
	if goarch == "arm64" {
		arch = d.platform.arm64
	}
	binary = d.binary
	if binary != "" {
		binary += suffix
	}
	replace := strings.NewReplacer("{version}", version, "{major}", strings.Split(version, ".")[0], "{os}", osName, "{arch}", arch, "{ext}", ext, "{suffix}", suffix, "{binary}", binary)
	asset = replace.Replace(d.asset)
	path = strings.ReplaceAll(replace.Replace(d.downloadPath), "{asset}", asset)
	cache = asset
	if d.cacheName != "" {
		cache = replace.Replace(d.cacheName)
	}
	extraEnv = make(map[string]string, len(d.extraEnv))
	for k, v := range d.extraEnv {
		extraEnv[k] = replace.Replace(v)
	}
	return
}

func (d toolDefinition) Install(ctx context.Context, spec, doguHome, mirrorURL string, opts InstallOptions) (*InstallResult, error) {
	if !opts.Force {
		if dir, unlock, ok, err := lockExistingCandidate(ctx, doguHome, d.name, spec, opts.IncludePrerelease); err != nil {
			return nil, err
		} else if ok {
			defer unlock()
			version := filepath.Base(dir)
			_, _, _, _, extraEnv := d.expand(version, runtime.GOOS, runtime.GOARCH)
			res := buildResultFromDir(d.name, spec, dir)
			if d.binary != "" {
				res.HomeDir, res.BinDirs = dir, []string{dir}
			} else {
				res.HomeDir = downloader.FindSubDirWithDir(dir, "bin")
				res.BinDirs = []string{filepath.Join(res.HomeDir, "bin")}
			}
			res.NormalizedVer = version
			res.EnvVars = buildEnvVars(d.name, version, res.HomeDir, d.homeEnv, extraEnv)
			return res, nil
		}
	}
	var version string
	var mavenArchive string
	var mavenDirect bool
	var err error
	if d.name == "maven" {
		baseURL := resolveBaseURL(d.baseURL, mirrorURL)
		versions, archive, direct, fetchErr := fetchApacheMavenRelease(ctx, baseURL, spec)
		mavenArchive, mavenDirect = archive, direct
		version, err = resolveDiscoveredVersion(versions, fetchErr, spec, opts.IncludePrerelease)
	} else {
		version, err = d.resolveVersion(ctx, spec, mirrorURL, opts.IncludePrerelease)
	}
	if err != nil {
		return nil, err
	}
	asset, path, binary, cache, extraEnv := d.expand(version, runtime.GOOS, runtime.GOARCH)
	base := resolveBaseURL(d.baseURL, mirrorURL)
	if d.name == "maven" {
		if !mavenDirect {
			mavenArchive = strings.TrimRight(mavenArchive, "/") + "/maven-" + mavenSpecMajor(version)
		}
		path = mavenArchivePath(mavenArchive, version, asset)
	}
	url := base + "/" + path
	if d.name == "maven" {
		url = path
	}
	if d.binary != "" {
		return installBinaryTool(ctx, BinaryInstallConfig{
			ToolName: d.name, VersionSpec: spec, NormalizedVer: version,
			DownloadURL: url, CacheFileName: cache, TargetBin: binary,
			DoguHome: doguHome, Opts: opts, ExtraHomeEnv: d.homeEnv, ExtraEnv: extraEnv,
		})
	}
	return installArchiveTool(ctx, ArchiveInstallConfig{
		ToolName: d.name, VersionSpec: spec, NormalizedVer: version,
		DownloadURL: url, FileName: asset, DoguHome: doguHome, Opts: opts,
		ExtraHomeEnv: d.homeEnv, ExtraEnv: extraEnv,
	})
}
