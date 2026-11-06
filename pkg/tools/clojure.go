package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/dogu-dev/dogu/pkg/downloader"
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

	version := leiningenDefinition.resolveVersion(ctx, spec, mirror, opts.IncludePrerelease)
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
