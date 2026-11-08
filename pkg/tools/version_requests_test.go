package tools

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dogu-dev/dogu/pkg/env"
)

func TestAllToolsReuseMajorAndLatest(t *testing.T) {
	old := http.DefaultTransport
	http.DefaultTransport = definitionTransport(func(r *http.Request) (*http.Response, error) {
		t.Errorf("local reuse made a network request: %s", r.URL)
		return nil, fmt.Errorf("network disabled")
	})
	t.Cleanup(func() { http.DefaultTransport = old })
	for _, name := range List() {
		t.Run(name, func(t *testing.T) {
			if _, diagram := diagramDefinition(name); diagram && runtime.GOOS == "windows" {
				t.Skip("Unix-only diagram provider")
			}
			if name == "elm" && runtime.GOARCH != "amd64" && !(runtime.GOOS == "darwin" && runtime.GOARCH == "arm64") {
				t.Skip("unsupported Elm architecture")
			}
			if runtime.GOOS == "windows" && (name == "clojure" || name == "rust" || name == "ghc" || name == "r" || name == "ruby" || name == "lua") {
				t.Skip("Unix-only provider")
			}
			home := t.TempDir()
			for _, version := range []string{"1", "1.2.3", "1.10.0", "2.0.0", "3.0.0-rc1"} {
				dir := filepath.Join(home, "candidates", name, version)
				if err := os.MkdirAll(filepath.Join(dir, "bin"), 0755); err != nil {
					t.Fatal(err)
				}
				for _, file := range []string{"bin/tool", "bin/clojure", "bin/clj", "lein", "lein.bat", ".dogu-complete"} {
					if err := os.WriteFile(filepath.Join(dir, file), []byte("fixture"), 0755); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := os.MkdirAll(filepath.Join(home, "candidates", name, "9.0.0"), 0755); err != nil {
				t.Fatal(err)
			}
			tool, _ := Get(name)
			for _, tc := range []struct {
				spec       string
				prerelease bool
				want       string
			}{
				{"1", false, "1.10.0"},
				{"", false, "2.0.0"},
				{"", true, "3.0.0-rc1"},
			} {
				res, err := tool.Install(t.Context(), tc.spec, home, "", InstallOptions{IncludePrerelease: tc.prerelease})
				if err != nil {
					t.Fatal(err)
				}
				if filepath.Base(res.InstallDir) != tc.want || res.NormalizedVer != tc.want || res.RequestedVer != tc.spec {
					t.Fatalf("spec=%q prerelease=%v: %+v; want %s", tc.spec, tc.prerelease, res, tc.want)
				}
				if res.EnvVars[env.FormatEnvKey(name, env.ExtractMajorVersion(tc.want))] != res.HomeDir {
					t.Fatalf("missing selected major's environment: %v", res.EnvVars)
				}
			}
		})
	}
}

func TestLatestSelection(t *testing.T) {
	for _, prerelease := range []bool{false, true} {
		want := "v2.10.0"
		if prerelease {
			want = "v3.0.0-rc1"
		}
		got, err := SelectBestVersion([]string{"v2.2.0", "v3.0.0-rc1", "", "v2.10.0", "v1.0.0"}, "", prerelease)
		if err != nil || got != want {
			t.Fatalf("latest(prerelease=%v) = %q, %v; want %q", prerelease, got, err, want)
		}
	}
	for _, versions := range [][]string{nil, {""}, {"3.0.0-rc1"}} {
		if _, err := SelectBestVersion(versions, "", false); err == nil {
			t.Fatalf("expected failure for %v", versions)
		}
	}
}

func TestSpecializedMajorAndLatestInstalls(t *testing.T) {
	for _, name := range []string{"java", "node", "python", "bun", "npm", "pnpm", "uv", "pants", "scala"} {
		t.Run(name, func(t *testing.T) {
			version, major := "2.10.0", "2"
			if name == "java" {
				version, major = "25.0.2", "25"
			} else if name == "python" {
				version, major = "3.14.2", "3"
			} else if name == "scala" {
				version, major = "3.10.0", "3"
			}
			asset, path, binary := "", "", ""
			mirror := "https://downloads.example.test"
			switch name {
			case "java":
				asset, path = "jdk.tar.gz", "/jdk.tar.gz"
				mirror = ""
			case "node":
				osName, arch := mapNodePlatform(runtime.GOOS, runtime.GOARCH)
				ext := "tar.gz"
				if runtime.GOOS == "windows" {
					ext = "zip"
				}
				asset = fmt.Sprintf("node-v%s-%s-%s.%s", version, osName, arch, ext)
				path = "/v" + version + "/" + asset
			case "python":
				asset = fmt.Sprintf("cpython-%s-20261101-%s-install_only.tar.gz", version, mapPythonTarget())
				path = "/20261101/" + asset
			case "bun":
				osName, arch := mapBunTarget()
				asset = fmt.Sprintf("bun-%s-%s.zip", osName, arch)
				path = "/bun-v" + version + "/" + asset
			case "npm":
				asset, path = "npm-"+version+".tgz", "/npm-"+version+".tgz"
			case "pnpm":
				binary = "pnpm"
				asset = fmt.Sprintf("pnpm-%s-%s", mapPNPMOS(), mapPNPMArch())
				if runtime.GOOS == "windows" {
					asset += ".exe"
					binary += ".exe"
				}
				path = "/v" + version + "/" + asset
			case "uv":
				asset = "uv-" + mapUVTarget() + ".tar.gz"
				if runtime.GOOS == "windows" {
					asset = "uv-" + mapUVTarget() + ".zip"
				}
				path = "/" + version + "/" + asset
			case "pants":
				osName, arch := mapPantsTarget()
				asset = "pants-" + osName + "_" + arch
				path, binary = "/release_"+version+"/"+asset, "pants"
				if runtime.GOOS == "windows" {
					binary += ".exe"
				}
			case "scala":
				asset = "scala3-" + version + ".zip"
				path = "/" + version + "/" + asset
			}
			payload := []byte("fixture")
			archiveBinary := "tool"
			if name == "bun" || name == "uv" {
				archiveBinary = name
				if runtime.GOOS == "windows" {
					archiveBinary += ".exe"
				}
			}
			if runtime.GOOS == "windows" && name == "node" {
				archiveBinary = "node.exe"
			}
			if runtime.GOOS == "windows" && name == "python" {
				archiveBinary = "python.exe"
			}
			if binary == "" {
				archivePath := "distribution/bin/" + archiveBinary
				if name == "bun" || name == "uv" {
					archivePath = "distribution/" + archiveBinary
				}
				if runtime.GOOS == "windows" && name == "node" {
					_, nodeArch := mapNodePlatform("windows", runtime.GOARCH)
					archivePath = fmt.Sprintf("node-v%s-win-%s/node.exe", version, nodeArch)
				} else if runtime.GOOS == "windows" && name == "python" {
					archivePath = "python/python.exe"
				}
				payload = definitionArchive(t, asset, archivePath, payload)
			}
			downloads, discoveries := 0, 0
			diagramHTTP(t, func(r *http.Request) (string, int) {
				if r.URL.Path == path && (r.URL.Host == "downloads.example.test" || name == "java") {
					downloads++
					return string(payload), 200
				}
				discoveries++
				switch {
				case name == "java" && r.URL.Host == "api.foojay.io":
					if got := r.URL.Query().Get("version"); got != "" && got != major {
						t.Errorf("unexpected version query: %q", got)
					}
					return fmt.Sprintf(`{"result":[
		{"java_version":"25.0.1","distribution":"temurin"},
		{"java_version":"26-ea","early_access":true},
		{"java_version":%q,"distribution":"temurin","filename":"jdk.tar.gz","links":{"pkg_download_redirect":"https://downloads.example.test/jdk.tar.gz"}}]}`, version), 200
				case name == "node" && r.URL.Path == "/index.tab":
					return "version\tdate\nv2.2.0\tdate\nv3.0.0-rc1\tdate\nv" + version + "\tdate\n", 200
				case name == "python" && r.URL.Host == "api.github.com":
					return fmt.Sprintf(`[{"tag_name":"20261101","assets":[{"name":%q},{"name":"cpython-99.0.0-20261101-wrong-target-install_only.tar.gz"},{"name":"cpython-3.15.0rc1-20261101-%s-install_only.tar.gz"}]},{"tag_name":"20261001","assets":[{"name":"cpython-%s-20261001-%s-install_only.tar.gz"}]}]`, asset, mapPythonTarget(), version, mapPythonTarget()), 200
				case name == "npm" || name == "pnpm":
					return fmt.Sprintf(`{"versions":{"2.2.0":{},%q:{},"3.0.0-rc1":{}}}`, version), 200
				case name == "uv" && r.URL.Host == "pypi.org":
					return fmt.Sprintf(`{"releases":{"2.2.0":[],%q:[],"3.0.0-rc1":[]}}`, version), 200
				case name == "pants" && strings.Contains(r.URL.Path, "/releases/tags/"):
					return fmt.Sprintf(`{"assets":[{"name":%q}]}`, asset), 200
				case r.URL.Host == "api.github.com" && strings.Contains(r.URL.Path, "/releases"):
					if r.URL.Query().Get("page") != "1" {
						return `[]`, 200
					}
					tag := "v" + version
					if name == "bun" {
						tag = "bun-v" + version
					} else if name == "pants" {
						tag = "release_" + version
					} else if name == "scala" && strings.Contains(r.URL.Path, "/scala/scala/") {
						tag = "v2.13.16"
					}
					return fmt.Sprintf(`[{"tag_name":%q,"assets":[{}]}]`, tag), 200
				default:
					t.Errorf("unexpected %s request: %s", name, r.URL)
					return "", 404
				}
			})
			tool, _ := Get(name)
			for _, spec := range []string{major, ""} {
				home := t.TempDir()
				res, err := tool.Install(t.Context(), spec, home, mirror, InstallOptions{})
				if err != nil {
					t.Fatalf("spec=%q: %v", spec, err)
				}
				want := version
				if name == "java" {
					want += "-temurin"
				} else if name == "node" || name == "bun" {
					want = "v" + want
				}
				if filepath.Base(res.InstallDir) != want || res.RequestedVer != spec {
					t.Fatalf("spec=%q: %+v; want %s", spec, res, want)
				}
				if res.EnvVars[env.FormatEnvKey(name, major)] != res.HomeDir {
					t.Fatalf("wrong version environment: %+v", res)
				}
				if name == "python" && res.EnvVars["PYTHONHOME"] != res.HomeDir {
					t.Fatalf("wrong Python home: %+v", res)
				}
				file := filepath.Join(res.BinDirs[0], archiveBinary)
				if binary != "" {
					file = filepath.Join(res.BinDirs[0], binary)
				}
				if _, err := os.Stat(file); err != nil {
					t.Fatalf("tool executable not found on PATH: %s: %v", file, err)
				}
				if err := os.WriteFile(file, []byte("broken"), 0755); err != nil {
					t.Fatal(err)
				}
				before := discoveries
				restored, err := tool.Install(t.Context(), "", home, mirror, InstallOptions{Force: true})
				if err != nil {
					t.Fatal(err)
				}
				if discoveries == before || restored.InstallDir != res.InstallDir {
					t.Fatal("force did not discover and replace the resolved candidate")
				}
				got, err := os.ReadFile(file)
				if err != nil || string(got) != "fixture" {
					t.Fatalf("force did not restore payload: %q, %v", got, err)
				}
			}
			if downloads != 2 {
				t.Fatalf("downloads=%d; want two fresh downloads (force reuses the archive cache)", downloads)
			}
		})
	}
}

func TestLatestDiscoveryFailures(t *testing.T) {
	diagramHTTP(t, func(r *http.Request) (string, int) {
		return "", http.StatusNotFound
	})
	for _, name := range List() {
		t.Run(name, func(t *testing.T) {
			tool, _ := Get(name)
			res, err := tool.Install(t.Context(), "", t.TempDir(), "", InstallOptions{})
			if err == nil || res != nil {
				t.Fatalf("discovery failure returned a candidate: %+v, %v", res, err)
			}
		})
	}
}
