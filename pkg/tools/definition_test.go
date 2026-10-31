package tools

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestCatalogAssets(t *testing.T) {
	for _, tc := range []struct {
		d                                    toolDefinition
		os, arch, asset, path, binary, cache string
	}{
		{antDefinition, "linux", "amd64", "apache-ant-1.2.3-bin.tar.gz", "apache-ant-1.2.3-bin.tar.gz", "", "apache-ant-1.2.3-bin.tar.gz"},
		{mavenDefinition, "darwin", "arm64", "apache-maven-1.2.3-bin.tar.gz", "1.2.3/binaries/apache-maven-1.2.3-bin.tar.gz", "", "apache-maven-1.2.3-bin.tar.gz"},
		{gradleDefinition, "windows", "amd64", "gradle-1.2.3-bin.zip", "gradle-1.2.3-bin.zip", "", "gradle-1.2.3-bin.zip"},
		{sbtDefinition, "linux", "amd64", "sbt-1.2.3.tgz", "v1.2.3/sbt-1.2.3.tgz", "", "sbt-1.2.3.tgz"},
		{sbtDefinition, "windows", "arm64", "sbt-1.2.3.zip", "v1.2.3/sbt-1.2.3.zip", "", "sbt-1.2.3.zip"},
		{yarnDefinition, "darwin", "arm64", "yarn-v1.2.3.tar.gz", "v1.2.3/yarn-v1.2.3.tar.gz", "", "yarn-v1.2.3.tar.gz"},
		{bazelDefinition, "linux", "amd64", "bazel-1.2.3-linux-x86_64", "1.2.3/bazel-1.2.3-linux-x86_64", "bazel", "bazel-1.2.3-linux-x86_64"},
		{bazelDefinition, "darwin", "arm64", "bazel-1.2.3-darwin-arm64", "1.2.3/bazel-1.2.3-darwin-arm64", "bazel", "bazel-1.2.3-darwin-arm64"},
		{bazelDefinition, "windows", "arm64", "bazel-1.2.3-windows-arm64.exe", "1.2.3/bazel-1.2.3-windows-arm64.exe", "bazel.exe", "bazel-1.2.3-windows-arm64.exe"},
		{buck2Definition, "linux", "arm64", "buck2-aarch64-unknown-linux-gnu.zst", "1.2.3/buck2-aarch64-unknown-linux-gnu.zst", "buck2", "buck2-aarch64-unknown-linux-gnu.zst"},
		{buck2Definition, "darwin", "amd64", "buck2-x86_64-apple-darwin.zst", "1.2.3/buck2-x86_64-apple-darwin.zst", "buck2", "buck2-x86_64-apple-darwin.zst"},
		{buck2Definition, "windows", "arm64", "buck2-aarch64-pc-windows-msvc.exe.zst", "1.2.3/buck2-aarch64-pc-windows-msvc.exe.zst", "buck2.exe", "buck2-aarch64-pc-windows-msvc.exe.zst"},
		{millDefinition, "darwin", "arm64", "1.2.3", "1.2.3/1.2.3", "mill", "mill-1.2.3-mill"},
		{millDefinition, "windows", "amd64", "1.2.3.bat", "1.2.3/1.2.3.bat", "mill.bat", "mill-1.2.3-mill.bat"},
	} {
		t.Run(tc.d.name+"/"+tc.os+"/"+tc.arch, func(t *testing.T) {
			asset, path, binary, cache, _ := tc.d.expand("1.2.3", tc.os, tc.arch)
			if asset != tc.asset || path != tc.path || binary != tc.binary || cache != tc.cache {
				t.Fatalf("got %q %q %q %q; want %q %q %q %q", asset, path, binary, cache, tc.asset, tc.path, tc.binary, tc.cache)
			}
		})
	}
}

type definitionTransport func(*http.Request) (*http.Response, error)

func (f definitionTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Redirect only version-discovery requests that cannot use a mirror (GitHub).
func localGitHubVersions(t *testing.T, server *httptest.Server) *int {
	t.Helper()
	requests := 0
	original := http.DefaultTransport
	http.DefaultTransport = definitionTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.URL.Host == "api.github.com" {
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`[{"tag_name":"v1.2.3","assets":[{}]}]`)), Request: r}, nil
		}
		if r.URL.Host != strings.TrimPrefix(server.URL, "http://") {
			return nil, fmt.Errorf("unexpected external request: %s", r.URL)
		}
		return original.RoundTrip(r)
	})
	t.Cleanup(func() { http.DefaultTransport = original })
	return &requests
}

func TestDefinitionInstall(t *testing.T) {
	for _, d := range []toolDefinition{antDefinition, mavenDefinition, gradleDefinition, sbtDefinition, yarnDefinition, bazelDefinition, buck2Definition, millDefinition} {
		t.Run(d.name, func(t *testing.T) {
			asset, path, binary, _, _ := d.expand("1.2.3", runtime.GOOS, runtime.GOARCH)
			payload := []byte("tool executable\n")
			if binary == "" {
				payload = definitionArchive(t, asset, d.name+"/bin/tool", payload)
			}
			if strings.HasSuffix(asset, ".zst") {
				encoder, err := zstd.NewWriter(nil)
				if err != nil {
					t.Fatal(err)
				}
				payload = encoder.EncodeAll(payload, nil)
				encoder.Close()
			}
			downloads := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/mirror/"+path {
					downloads++
					_, _ = w.Write(payload)
					return
				}
				switch r.URL.Path {
				case "/mirror/":
					_, _ = io.WriteString(w, `apache-ant-1.2.3-bin.tar.gz <a href="1.2.3/">1.2.3</a>`)
				case "/mirror/versions/all":
					_, _ = io.WriteString(w, `[{"version":"1.2.3"}]`)
				case "/mirror/yarn":
					_, _ = io.WriteString(w, `{"versions":{"1.2.3":{}}}`)
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			requests := localGitHubVersions(t, server)
			home := t.TempDir()
			tool, err := FindTool(d.name)
			if err != nil {
				t.Fatal(err)
			}
			fresh, err := tool.Install(t.Context(), "1.2", home, server.URL+"/mirror/", InstallOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if fresh.NormalizedVer != "1.2.3" || fresh.RequestedVer != "1.2" {
				t.Fatalf("unexpected result: %+v", fresh)
			}
			for _, key := range d.homeEnv {
				if fresh.EnvVars[key] != fresh.HomeDir {
					t.Errorf("missing %s: %+v", key, fresh.EnvVars)
				}
			}
			if d.name == "mill" && fresh.EnvVars["MILL_VERSION"] != "1.2.3" {
				t.Fatal("missing MILL_VERSION")
			}
			installedBinary := filepath.Join(fresh.BinDirs[0], "tool")
			if binary != "" {
				installedBinary = filepath.Join(fresh.HomeDir, binary)
			}
			if got, err := os.ReadFile(installedBinary); err != nil || string(got) != "tool executable\n" {
				t.Fatalf("installed payload: %q, %v", got, err)
			}
			beforeCached := *requests
			server.Close() // Cached installs must not need network access.
			cached, err := tool.Install(t.Context(), "1", home, server.URL+"/mirror", InstallOptions{})
			if err != nil {
				t.Fatal(err)
			}
			fresh.RequestedVer = "1"
			if !reflect.DeepEqual(fresh, cached) {
				t.Fatalf("fresh and cached differ:\n%+v\n%+v", fresh, cached)
			}
			if *requests != beforeCached {
				t.Fatal("cached install made network requests")
			}
			// Force bypasses the candidate and attempts version discovery again.
			_, err = tool.Install(t.Context(), "1.2.3", home, server.URL+"/mirror", InstallOptions{Force: true})
			if err != nil {
				t.Fatal(err)
			}
			if *requests == beforeCached {
				t.Fatal("force did not attempt version discovery")
			}

			if downloads != 1 {
				t.Fatalf("downloads = %d, want 1", downloads)
			}
		})
	}
}

func definitionArchive(t *testing.T, asset, name string, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	if strings.HasSuffix(asset, ".zip") {
		w := zip.NewWriter(&buf)
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(payload); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
	} else {
		gz := gzip.NewWriter(&buf)
		w := tar.NewWriter(gz)
		if err := w.WriteHeader(&tar.Header{Name: name, Mode: 0755, Size: int64(len(payload))}); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(payload); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return buf.Bytes()
}

func TestDefinitionVersionSelection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"versions":{"1.2.1":{},"1.2.3":{},"1.3.0-rc.1":{}}}`)
	}))
	defer server.Close()
	d := yarnDefinition
	for _, tc := range []struct {
		spec       string
		prerelease bool
		want       string
	}{
		{"1.2", false, "1.2.3"}, {"1", false, "1.2.3"}, {"1", true, "1.3.0-rc.1"}, {"v9.8.7", false, "9.8.7"},
	} {
		if got := d.resolveVersion(context.Background(), tc.spec, server.URL, tc.prerelease); got != tc.want {
			t.Errorf("resolve(%q, %v) = %q, want %q", tc.spec, tc.prerelease, got, tc.want)
		}
	}
	server.Close()
	if got := d.resolveVersion(t.Context(), "v9.8.7", server.URL, false); got != "9.8.7" {
		t.Fatalf("fallback = %q", got)
	}
}
