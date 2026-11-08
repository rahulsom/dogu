package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/dogu-dev/dogu/pkg/downloader"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// AntTool installs ant using the shared tool definition.
type AntTool struct{}

func init() { Register(&AntTool{}) }

func (*AntTool) Name() string { return "ant" }

func (*AntTool) Install(ctx context.Context, versionSpec, doguHome, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	return antDefinition.Install(ctx, versionSpec, doguHome, mirrorBaseURL, opts)
}

// BazelTool installs bazel using the shared tool definition.
type BazelTool struct{}

func init() { Register(&BazelTool{}) }

func (*BazelTool) Name() string { return "bazel" }

func (*BazelTool) Install(ctx context.Context, versionSpec, doguHome, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	return bazelDefinition.Install(ctx, versionSpec, doguHome, mirrorBaseURL, opts)
}

// Buck2Tool installs buck2 using the shared tool definition.
type Buck2Tool struct{}

func init() { Register(&Buck2Tool{}) }

func (*Buck2Tool) Name() string { return "buck2" }

func (*Buck2Tool) Install(ctx context.Context, versionSpec, doguHome, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	return buck2Definition.Install(ctx, versionSpec, doguHome, mirrorBaseURL, opts)
}

type ElmTool struct{}

func init()                   { Register(&ElmTool{}) }
func (*ElmTool) Name() string { return "elm" }
func (*ElmTool) Install(ctx context.Context, spec, home, mirror string, opts InstallOptions) (*InstallResult, error) {
	d := elmDefinition
	if runtime.GOARCH != "amd64" && !(runtime.GOOS == "darwin" && runtime.GOARCH == "arm64") {
		return nil, fmt.Errorf("elm supports x86-64 and macOS ARM64 binaries")
	}
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		d.asset = "binary-for-mac-64-bit-ARM.gz"
	}
	return d.Install(ctx, spec, home, mirror, opts)
}

func fetchReleaseData(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release index %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 32<<20))
}

func fetchIndexVersions(ctx context.Context, base string) ([]string, error) {
	data, err := fetchReleaseData(ctx, strings.TrimRight(base, "/")+"/")
	if err != nil {
		return nil, err
	}
	matches := regexp.MustCompile(`href="([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:-[A-Za-z0-9.-]+)?)/?"`).FindAllSubmatch(data, -1)
	var versions []string
	for _, m := range matches {
		versions = append(versions, string(m[1]))
	}
	return versions, nil
}

type GoTool struct{}
type JuliaTool struct{}
type RustTool struct{}
type GHCTool struct{}
type RTool struct{}
type RubyTool struct{}
type LuaTool struct{}

func init() {
	Register(&GoTool{})
	Register(&JuliaTool{})
	Register(&RustTool{})
	Register(&GHCTool{})
	Register(&RTool{})
	Register(&RubyTool{})
	Register(&LuaTool{})
}
func (*GoTool) Name() string    { return "go" }
func (*JuliaTool) Name() string { return "julia" }
func (*RustTool) Name() string  { return "rust" }
func (*GHCTool) Name() string   { return "ghc" }
func (*RTool) Name() string     { return "r" }
func (*RubyTool) Name() string  { return "ruby" }
func (*LuaTool) Name() string   { return "lua" }

// Go and Julia publish machine-readable indexes containing the actual archive
// names. Select only archives for this host rather than guessing release paths.
type languageRelease struct{ version, url, filename string }

func sourceLanguageReleases(ctx context.Context, name, mirror string) ([]languageRelease, error) {
	base, index := "https://www.lua.org/ftp", "/"
	pattern := `(?i:href)="(lua-([0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.-]+)?)\.tar\.gz)"`
	if name == "ruby" {
		base, index = "https://cache.ruby-lang.org/pub/ruby", "/index.txt"
		pattern = `/pub/ruby/([0-9]+\.[0-9]+/ruby-([0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.-]+)?)\.tar\.gz)`
	}
	base = resolveBaseURL(base, mirror)
	data, err := fetchReleaseData(ctx, base+index)
	if err != nil {
		return nil, err
	}
	var releases []languageRelease
	for _, m := range regexp.MustCompile(pattern).FindAllSubmatch(data, -1) {
		path := string(m[1])
		releases = append(releases, languageRelease{string(m[2]), base + "/" + path, filepath.Base(path)})
	}
	return releases, nil
}

func installIndexedLanguage(ctx context.Context, name, spec, home, mirror string, opts InstallOptions) (*InstallResult, error) {
	envKeys := []string(nil)
	if name == "go" {
		envKeys = []string{"GOROOT"}
	}
	if res, ok, err := checkExistingCandidate(home, name, spec, opts, func(r *InstallResult) error {
		r.NormalizedVer = filepath.Base(r.InstallDir)
		r.EnvVars = buildEnvVars(name, r.NormalizedVer, r.HomeDir, envKeys, nil)
		return nil
	}); ok || err != nil {
		return res, err
	}
	var releases []languageRelease
	var err error
	if name == "go" {
		var data []byte
		data, err = fetchReleaseData(ctx, resolveBaseURL("https://go.dev/dl", mirror)+"/?mode=json&include=all")
		if err == nil {
			var index []struct {
				Version string
				Files   []struct{ Filename, OS, Arch, Kind string }
			}
			err = json.Unmarshal(data, &index)
			for _, v := range index {
				for _, f := range v.Files {
					if f.OS == runtime.GOOS && f.Arch == runtime.GOARCH && f.Kind == "archive" {
						releases = append(releases, languageRelease{strings.TrimPrefix(v.Version, "go"), resolveBaseURL("https://go.dev/dl", mirror) + "/" + f.Filename, f.Filename})
					}
				}
			}
		}
	} else {
		var data []byte
		data, err = fetchReleaseData(ctx, resolveBaseURL("https://julialang-s3.julialang.org/bin", mirror)+"/versions.json")
		if err == nil {
			var index map[string]struct {
				Files []struct {
					URL, OS, Arch, Extension string
					Kind                     string
				}
			}
			err = json.Unmarshal(data, &index)
			arch := "x86_64"
			if runtime.GOARCH == "arm64" {
				arch = "aarch64"
			}
			osName := runtime.GOOS
			if osName == "windows" {
				osName = "winnt"
			}
			if osName == "darwin" {
				osName = "mac"
			}
			for v, r := range index {
				for _, f := range r.Files {
					if f.OS == osName && f.Arch == arch && (f.Extension == "tar.gz" || f.Extension == "zip") && !strings.Contains(f.URL, "musl") {
						url := f.URL
						if mirror != "" {
							url = strings.Replace(url, "https://julialang-s3.julialang.org/bin", strings.TrimRight(mirror, "/"), 1)
						}
						releases = append(releases, languageRelease{v, url, filepath.Base(f.URL)})
					}
				}
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", name, err)
	}
	versions := make([]string, 0, len(releases))
	for _, r := range releases {
		versions = append(versions, r.version)
	}
	version, err := SelectBestVersion(versions, spec, opts.IncludePrerelease)
	if err != nil {
		return nil, err
	}
	for _, r := range releases {
		if r.version == version {
			return installArchiveTool(ctx, ArchiveInstallConfig{ToolName: name, VersionSpec: spec, NormalizedVer: version, DownloadURL: r.url, FileName: r.filename, DoguHome: home, Opts: opts, ExtraHomeEnv: envKeys})
		}
	}
	return nil, fmt.Errorf("no %s archive for %s/%s", name, runtime.GOOS, runtime.GOARCH)
}
func (*GoTool) Install(ctx context.Context, spec, home, mirror string, opts InstallOptions) (*InstallResult, error) {
	return installIndexedLanguage(ctx, "go", strings.TrimPrefix(spec, "go"), home, mirror, opts)
}
func (*JuliaTool) Install(ctx context.Context, spec, home, mirror string, opts InstallOptions) (*InstallResult, error) {
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return nil, fmt.Errorf("unsupported Julia architecture %s", runtime.GOARCH)
	}
	return installIndexedLanguage(ctx, "julia", spec, home, mirror, opts)
}

// These distributions require setup, so extraction happens outside candidates.
// Failed setup never leaves an installation that can be mistaken for a cache hit.
func installConfiguredLanguage(ctx context.Context, name, spec, home, mirror string, opts InstallOptions) (res *InstallResult, err error) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("%s installation currently supports Linux and macOS", name)
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return nil, fmt.Errorf("unsupported %s architecture %s", name, runtime.GOARCH)
	}
	if r, ok, e := checkExistingCandidate(home, name, spec, opts, func(r *InstallResult) error {
		r.NormalizedVer = filepath.Base(r.InstallDir)
		r.EnvVars = configuredLanguageEnv(name, r.NormalizedVer, r.HomeDir)
		return nil
	}); ok || e != nil {
		return r, e
	}
	base := "https://static.rust-lang.org/dist"
	if name == "ghc" {
		base = "https://downloads.haskell.org/~ghc"
	}
	if name == "r" {
		base = "https://cran.r-project.org/src/base/R-" + strings.Split(CleanVersion(spec), ".")[0]
	}
	base = resolveBaseURL(base, mirror)
	version := CleanVersion(spec)
	var sourceRelease languageRelease
	if name == "ruby" || name == "lua" {
		releases, e := sourceLanguageReleases(ctx, name, mirror)
		if e != nil {
			return nil, e
		}
		var versions []string
		for _, release := range releases {
			versions = append(versions, release.version)
		}
		version, e = SelectBestVersion(versions, spec, opts.IncludePrerelease)
		if e != nil {
			return nil, e
		}
		for _, release := range releases {
			if release.version == version {
				sourceRelease = release
				break
			}
		}
	} else if name == "rust" {
		versions, e := fetchGitHubReleases(ctx, "rust-lang", "rust", false)
		if e == nil {
			if v, e := SelectBestVersion(versions, spec, opts.IncludePrerelease); e == nil {
				version = CleanVersion(v)
			}
		}
	} else if name == "ghc" {
		versions, e := fetchIndexVersions(ctx, base)
		if e != nil {
			return nil, e
		}
		if v, e := SelectBestVersion(versions, spec, opts.IncludePrerelease); e == nil {
			version = CleanVersion(v)
		}
	} else if name == "r" {
		data, e := fetchReleaseData(ctx, base+"/")
		if e != nil {
			return nil, e
		}
		matches := regexp.MustCompile(`R-([0-9]+\.[0-9]+\.[0-9]+)\.tar\.gz`).FindAllSubmatch(data, -1)
		var versions []string
		for _, m := range matches {
			versions = append(versions, string(m[1]))
		}
		version, e = SelectBestVersion(versions, spec, opts.IncludePrerelease)
		if e != nil {
			return nil, e
		}
	}
	arch := "x86_64"
	if runtime.GOARCH == "arm64" {
		arch = "aarch64"
	}
	filename := "R-" + version + ".tar.gz"
	path := filename
	if name == "rust" {
		target := "unknown-linux-gnu"
		if runtime.GOOS == "darwin" {
			target = "apple-darwin"
		}
		filename = "rust-" + version + "-" + arch + "-" + target + ".tar.gz"
		path = filename
	}
	if name == "ghc" {
		target := "deb10-linux"
		if runtime.GOOS == "darwin" {
			target = "apple-darwin"
		}
		filename = "ghc-" + version + "-" + arch + "-" + target + ".tar.xz"
		path = version + "/" + filename
		// Linux bindist availability changes across GHC releases. Prefer the
		// oldest available Debian baseline to minimize runtime requirements.
		if runtime.GOOS == "linux" {
			data, e := fetchReleaseData(ctx, base+"/"+version+"/")
			if e != nil {
				return nil, e
			}
			for _, distro := range []string{"deb10", "deb11", "deb12", "deb13"} {
				candidate := "ghc-" + version + "-" + arch + "-" + distro + "-linux.tar.xz"
				if strings.Contains(string(data), `href="`+candidate+`"`) {
					filename, path = candidate, version+"/"+candidate
					break
				}
			}
		}
	}
	work, e := os.MkdirTemp("", "dogu-"+name+"-*")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(work)
	downloadURL := base + "/" + path
	if sourceRelease.url != "" {
		downloadURL, filename = sourceRelease.url, sourceRelease.filename
	}
	if e = downloader.DownloadAndExtract(ctx, downloadURL, filepath.Join(home, "cache", name, filename), work, downloader.WithRewriteRule(opts.RewriteRules)); e != nil {
		return nil, e
	}
	entries, e := os.ReadDir(work)
	if e != nil {
		return nil, e
	}
	source := ""
	for _, entry := range entries {
		if entry.IsDir() {
			source = filepath.Join(work, entry.Name())
			break
		}
	}
	if source == "" {
		return nil, fmt.Errorf("%s archive contains no distribution directory", name)
	}
	dest, e := filepath.Abs(filepath.Join(home, "candidates", name, version))
	if e != nil {
		return nil, e
	}
	// Configure uses the final prefix; clean up that prefix if setup fails.
	if opts.Force {
		if e = os.RemoveAll(dest); e != nil {
			return nil, e
		}
	}
	if e = os.MkdirAll(dest, 0755); e != nil {
		return nil, e
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dest)
		}
	}()
	run := func(command string, args ...string) error {
		cmd := exec.CommandContext(ctx, command, args...)
		cmd.Dir = source
		output, e := cmd.CombinedOutput()
		if e != nil {
			return fmt.Errorf("%s setup (%s): %w\n%s", name, command, e, output)
		}
		return nil
	}
	if name == "rust" {
		err = run("sh", "./install.sh", "--prefix="+dest, "--disable-ldconfig")
	} else if name == "lua" {
		target := "linux"
		if runtime.GOOS == "darwin" {
			target = "macosx"
		}
		err = run("make", target)
		if err == nil {
			err = run("make", "install", "INSTALL_TOP="+dest)
		}
	} else {
		args := []string{"./configure", "--prefix=" + dest}
		if name == "r" {
			args = append(args, "--with-x=no")
		}
		if name == "ruby" {
			args = append(args, "--disable-install-doc")
		}
		err = run("sh", args...)
		if err == nil && (name == "r" || name == "ruby") {
			err = run("make")
		}
		if err == nil {
			err = run("make", "install")
		}
	}
	if err != nil {
		return nil, err
	}
	res = buildResultFromDir(name, spec, dest)
	res.NormalizedVer = version
	res.EnvVars = configuredLanguageEnv(name, version, res.HomeDir)
	return res, nil
}
func (*RustTool) Install(ctx context.Context, spec, home, mirror string, opts InstallOptions) (*InstallResult, error) {
	return installConfiguredLanguage(ctx, "rust", spec, home, mirror, opts)
}
func (*GHCTool) Install(ctx context.Context, spec, home, mirror string, opts InstallOptions) (*InstallResult, error) {
	return installConfiguredLanguage(ctx, "ghc", spec, home, mirror, opts)
}
func (*RTool) Install(ctx context.Context, spec, home, mirror string, opts InstallOptions) (*InstallResult, error) {
	return installConfiguredLanguage(ctx, "r", spec, home, mirror, opts)
}
func (*RubyTool) Install(ctx context.Context, spec, home, mirror string, opts InstallOptions) (*InstallResult, error) {
	return installConfiguredLanguage(ctx, "ruby", spec, home, mirror, opts)
}
func (*LuaTool) Install(ctx context.Context, spec, home, mirror string, opts InstallOptions) (*InstallResult, error) {
	return installConfiguredLanguage(ctx, "lua", spec, home, mirror, opts)
}

func configuredLanguageEnv(name, version, home string) map[string]string {
	vars := buildEnvVars(name, version, home, nil, nil)
	if name == "r" {
		vars["R_HOME"] = filepath.Join(home, "lib", "R")
	}
	return vars
}

// MillTool installs mill using the shared tool definition.
type MillTool struct{}

func init() { Register(&MillTool{}) }

func (*MillTool) Name() string { return "mill" }

func (*MillTool) Install(ctx context.Context, versionSpec, doguHome, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	return millDefinition.Install(ctx, versionSpec, doguHome, mirrorBaseURL, opts)
}

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
