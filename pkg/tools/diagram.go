package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/dogu-dev/dogu/pkg/downloader"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// Package-based tools are installed into private prefixes, never globally.
// Runtimes and build prerequisites must already be on the installer's PATH.
type diagramTool struct{ name, kind, pkg, base string }

var diagramTools = []diagramTool{
	{"asciitosvg", "go", "github.com/asciitosvg/asciitosvg/cmd/a2s", "https://proxy.golang.org/github.com/asciitosvg/asciitosvg"},
	{"barby", "gem", "barby", "https://rubygems.org"},
	{"rqrcode", "gem", "rqrcode", "https://rubygems.org"},
	{"bytefield-svg", "npm", "bytefield-svg", "https://registry.npmjs.org"},
	{"d2", "github", "d2lang/d2", "https://github.com/d2lang/d2/releases/download"},
	{"dbml-renderer", "npm", "@softwaretechnik/dbml-renderer", "https://registry.npmjs.org"},
	{"ditaa", "github", "stathissideris/ditaa", "https://github.com/stathissideris/ditaa/releases/download"},
	{"erd", "cabal", "erd", "https://hackage.haskell.org"},
	{"gnuplot", "source", "", "https://sourceforge.net/projects/gnuplot/files/gnuplot"},
	{"graphviz", "source", "", "https://graphviz.org/download/source"},
	{"lilypond", "gitlab", "lilypond/lilypond", "https://gitlab.com/lilypond/lilypond/-/releases"},
	{"mermaid-cli", "npm", "@mermaid-js/mermaid-cli", "https://registry.npmjs.org"},
	{"penrose", "npm", "@penrose/roger", "https://registry.npmjs.org"},
	{"plantuml", "github", "plantuml/plantuml", "https://github.com/plantuml/plantuml/releases/download"},
	{"js-sequence-diagrams", "browser", "bramp/js-sequence-diagrams", "https://github.com/bramp/js-sequence-diagrams/archive/refs/tags"},
	{"jsyntrax", "github", "atp-mipt/jsyntrax", "https://github.com/atp-mipt/jsyntrax/releases/download"},
	{"vega-lite", "npm", "vega-lite", "https://registry.npmjs.org"},
	{"vega", "npm", "vega-cli", "https://registry.npmjs.org"},
}

func init() {
	for _, d := range diagramTools {
		Register(d)
	}
}
func (d diagramTool) Name() string { return d.name }
func diagramDefinition(name string) (diagramTool, bool) {
	for _, d := range diagramTools {
		if d.name == name {
			return d, true
		}
	}
	return diagramTool{}, false
}

type diagramRelease struct {
	Assets []struct {
		Name string
	} `json:"assets"`
}

func (d diagramTool) versions(ctx context.Context, mirror string) ([]string, error) {
	base := resolveBaseURL(d.base, mirror)
	if d.kind == "npm" {
		return FetchNPMVersions(ctx, d.pkg, base)
	}
	if d.kind == "browser" {
		data, err := fetchReleaseData(ctx, "https://api.github.com/repos/"+d.pkg+"/tags?per_page=100")
		if err != nil {
			return nil, err
		}
		var tags []struct{ Name string }
		if err := json.Unmarshal(data, &tags); err != nil {
			return nil, err
		}
		var versions []string
		for _, tag := range tags {
			versions = append(versions, tag.Name)
		}
		return versions, nil
	}
	if d.kind == "github" {
		return fetchGitHubReleases(ctx, strings.Split(d.pkg, "/")[0], strings.Split(d.pkg, "/")[1], true)
	}
	endpoint := base + "/"
	switch d.kind {
	case "gem":
		endpoint = base + "/api/v1/versions/" + d.pkg + ".json"
	case "go":
		endpoint = base + "/@v/list"
	case "gitlab":
		endpoint = "https://gitlab.com/api/v4/projects/" + url.PathEscape(d.pkg) + "/releases?per_page=100"
	case "cabal":
		endpoint = base + "/package/erd/preferred.json"
	}
	data, err := fetchReleaseData(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	var versions []string
	switch d.kind {
	case "gem":
		var entries []struct{ Number string }
		if err := json.Unmarshal(data, &entries); err != nil {
			return nil, err
		}
		for _, e := range entries {
			versions = append(versions, e.Number)
		}
	case "go":
		versions = strings.Fields(string(data))
		if len(versions) == 0 {
			data, err = fetchReleaseData(ctx, base+"/@latest")
			if err != nil {
				return nil, err
			}
			var entry struct{ Version string }
			if err := json.Unmarshal(data, &entry); err != nil {
				return nil, err
			}
			versions = append(versions, entry.Version)
		}
	case "gitlab":
		var entries []struct {
			TagName string `json:"tag_name"`
		}
		if err := json.Unmarshal(data, &entries); err != nil {
			return nil, err
		}
		for _, e := range entries {
			versions = append(versions, e.TagName)
		}
	case "cabal":
		var entries map[string][]string
		if err := json.Unmarshal(data, &entries); err != nil {
			return nil, err
		}
		versions = append(entries["normal-version"], entries["deprecated-version"]...)
	default:
		pattern := `graphviz-([0-9]+\.[0-9]+\.[0-9]+)\.tar\.gz`
		if d.name == "gnuplot" {
			pattern = `/gnuplot/([0-9]+\.[0-9]+(?:\.[0-9]+)?)/`
		}
		for _, m := range regexp.MustCompile(pattern).FindAllSubmatch(data, -1) {
			versions = append(versions, string(m[1]))
		}
	}
	return versions, nil
}

func (d diagramTool) result(spec, version, dest string) *InstallResult {
	r := buildResultFromDir(d.name, spec, dest)
	r.NormalizedVer = version
	r.EnvVars = buildEnvVars(d.name, version, r.HomeDir, nil, nil)
	if d.kind == "npm" {
		r.HomeDir = dest
		r.BinDirs = []string{filepath.Join(dest, "node_modules", ".bin")}
		r.EnvVars = buildEnvVars(d.name, version, dest, nil, map[string]string{"NODE_PATH": filepath.Join(dest, "node_modules")})
	}
	if d.kind == "gem" {
		r.EnvVars["GEM_PATH"] = dest
	}
	return r
}

func (d diagramTool) Install(ctx context.Context, spec, home, mirror string, opts InstallOptions) (res *InstallResult, err error) {
	// These installers use POSIX build tools and launchers. Reject unsupported hosts explicitly.
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("%s installation currently supports Linux and macOS", d.name)
	}
	if !opts.Force {
		if dest, ok := findExistingCandidate(home, d.name, spec, opts.IncludePrerelease); ok {
			if _, e := os.Stat(filepath.Join(dest, ".dogu-complete")); e == nil {
				return d.result(spec, filepath.Base(dest), dest), nil
			}
		}
	}
	versions, err := d.versions(ctx, mirror)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", d.name, err)
	}
	tag, err := SelectBestVersion(versions, spec, opts.IncludePrerelease)
	if err != nil {
		return nil, err
	}
	version := CleanVersion(tag)
	if !regexp.MustCompile(`^[0-9][0-9A-Za-z._+-]*$`).MatchString(version) {
		return nil, fmt.Errorf("invalid %s release version %q", d.name, version)
	}
	dest, err := filepath.Abs(filepath.Join(home, "candidates", d.name, version))
	if err != nil {
		return nil, err
	}
	if err = os.RemoveAll(dest); err != nil {
		return nil, err
	}
	if err = os.MkdirAll(dest, 0755); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dest)
		}
	}()
	base := resolveBaseURL(d.base, mirror)
	rewrite := func(s string) string {
		if opts.RewriteRules != nil {
			return opts.RewriteRules.RewriteURL(s)
		}
		return s
	}
	run := func(dir string, env []string, name string, args ...string) error {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), env...)
		output, e := cmd.CombinedOutput()
		if e != nil {
			return fmt.Errorf("%s setup requires %s and its prerequisites: %w\n%s", d.name, name, e, output)
		}
		return nil
	}
	switch d.kind {
	case "npm":
		err = run(dest, nil, "npm", "install", "--prefix", dest, "--no-audit", "--no-fund", "--registry", rewrite(base), d.pkg+"@"+version)
	case "gem":
		err = run(dest, nil, "gem", "install", d.pkg, "--version", version, "--install-dir", dest, "--bindir", filepath.Join(dest, "bin"), "--no-document", "--clear-sources", "--source", rewrite(base))
	case "go":
		err = run(dest, []string{"GOBIN=" + filepath.Join(dest, "bin"), "GOPROXY=" + rewrite(strings.TrimSuffix(base, "/github.com/asciitosvg/asciitosvg"))}, "go", "install", d.pkg+"@"+tag)
	case "cabal":
		// Cabal's repository is configured by the caller; the prefix remains private.
		err = run(dest, nil, "cabal", "install", "erd-"+version, "--installdir="+filepath.Join(dest, "bin"), "--install-method=copy", "--overwrite-policy=always")
	case "source":
		err = d.installSource(ctx, version, dest, home, base, opts, run)
	case "browser":
		filename := tag + ".tar.gz"
		err = downloader.DownloadAndExtract(ctx, base+"/"+filename, filepath.Join(home, "cache", d.name, filename), dest, downloader.WithRewriteRule(opts.RewriteRules))
	default:
		var asset, download string
		asset, download, err = d.asset(ctx, tag, version, base, runtime.GOOS, runtime.GOARCH)
		if err != nil {
			return nil, err
		}
		cache := filepath.Join(home, "cache", d.name, version+"-"+asset)
		if strings.HasSuffix(asset, ".jar") {
			err = downloader.SaveSingleBinary(ctx, download, cache, dest, asset, downloader.WithRewriteRule(opts.RewriteRules))
			if err == nil {
				err = writeDiagramJarLauncher(dest, d.name, asset)
			}
		} else {
			err = downloader.DownloadAndExtract(ctx, download, cache, dest, downloader.WithRewriteRule(opts.RewriteRules))
		}
	}
	if err != nil {
		return nil, err
	}
	if err = os.WriteFile(filepath.Join(dest, ".dogu-complete"), []byte(version), 0644); err != nil {
		return nil, err
	}
	return d.result(spec, version, dest), nil
}

func writeDiagramJarLauncher(dest, name, jar string) error {
	script := fmt.Sprintf("#!/bin/sh\nDIR=$(CDPATH= cd -- \"$(dirname -- \"$0\")\" && pwd)\nexec java -jar \"$DIR/%s\" \"$@\"\n", jar)
	return os.WriteFile(filepath.Join(dest, name), []byte(script), 0755)
}

func (d diagramTool) asset(ctx context.Context, tag, version, base, goos, goarch string) (string, string, error) {
	if d.kind == "gitlab" {
		arch := "x86_64"
		if goarch == "arm64" {
			arch = "arm64"
		}
		if (goos == "linux" && goarch != "amd64") || (goarch != "amd64" && goarch != "arm64") {
			return "", "", fmt.Errorf("no LilyPond binary for %s/%s", goos, goarch)
		}
		name := fmt.Sprintf("lilypond-%s-%s-%s.tar.gz", version, goos, arch)
		return name, base + "/" + tag + "/downloads/" + name, nil
	}
	data, err := fetchReleaseData(ctx, "https://api.github.com/repos/"+d.pkg+"/releases/tags/"+url.PathEscape(tag))
	if err != nil {
		return "", "", err
	}
	var release diagramRelease
	if err = json.Unmarshal(data, &release); err != nil {
		return "", "", err
	}
	for _, a := range release.Assets {
		if diagramAssetMatches(d.name, a.Name, version, goos, goarch) {
			return a.Name, base + "/" + tag + "/" + a.Name, nil
		}
	}
	return "", "", fmt.Errorf("no %s release asset for %s/%s at %s", d.name, goos, goarch, tag)
}

func diagramAssetMatches(tool, name, version, goos, goarch string) bool {
	switch tool {
	case "plantuml":
		return name == "plantuml-"+version+".jar" || name == "plantuml.jar"
	case "ditaa":
		return name == "ditaa-"+version+"-standalone.jar"
	case "jsyntrax":
		return name == "jsyntrax-"+version+"-syntrax.zip"
	case "d2":
		if goos == "darwin" {
			goos = "macos"
		}
		return name == fmt.Sprintf("d2-v%s-%s-%s.tar.gz", version, goos, goarch)
	}
	return false
}

func (d diagramTool) installSource(ctx context.Context, version, dest, home, base string, opts InstallOptions, run func(string, []string, string, ...string) error) error {
	filename := d.name + "-" + version + ".tar.gz"
	download := fmt.Sprintf("https://gitlab.com/api/v4/projects/4207231/packages/generic/graphviz-releases/%s/%s", version, filename)
	if d.name == "gnuplot" {
		download = base + "/" + version + "/" + filename + "/download"
	} else if base != d.base {
		download = base + "/" + filename
	}
	work, err := os.MkdirTemp("", "dogu-"+d.name+"-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	if err = downloader.DownloadAndExtract(ctx, download, filepath.Join(home, "cache", d.name, filename), work, downloader.WithRewriteRule(opts.RewriteRules)); err != nil {
		return err
	}
	source := ""
	err = filepath.WalkDir(work, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && entry.Name() == "configure" {
			source = filepath.Dir(path)
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return err
	}
	if source == "" {
		return fmt.Errorf("%s archive has no configure script", d.name)
	}
	if err = run(source, nil, "sh", "./configure", "--prefix="+dest); err != nil {
		return err
	}
	if err = run(source, nil, "make"); err != nil {
		return err
	}
	return run(source, nil, "make", "install")
}
