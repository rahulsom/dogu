package tools

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ulikunitz/xz"
)

func TestNewCatalogInstall(t *testing.T) {
	for _, d := range []toolDefinition{groovyDefinition, kotlinDefinition, scalaDefinition, elmDefinition} {
		t.Run(d.name, func(t *testing.T) {
			asset, path, binary, _, _ := d.expand("1.2.3", runtime.GOOS, runtime.GOARCH)
			payload := []byte("executable")
			if binary == "" {
				payload = definitionArchive(t, asset, "distribution/bin/compiler", payload)
			} else {
				var buf bytes.Buffer
				w := gzip.NewWriter(&buf)
				_, _ = w.Write(payload)
				_ = w.Close()
				payload = buf.Bytes()
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/"+path {
					_, _ = w.Write(payload)
					return
				}
				if r.URL.Path == "/" {
					_, _ = io.WriteString(w, `<a href="1.2.3/">release</a>`)
					return
				}
				http.NotFound(w, r)
			}))
			defer server.Close()
			localGitHubVersions(t, server)
			home := t.TempDir()
			res, err := d.Install(t.Context(), "1.2", home, server.URL, InstallOptions{})
			if err != nil {
				t.Fatal(err)
			}
			executable := filepath.Join(res.BinDirs[0], "compiler")
			if binary != "" {
				executable = filepath.Join(res.BinDirs[0], binary)
			}
			got, err := os.ReadFile(executable)
			if err != nil || string(got) != "executable" {
				t.Fatalf("payload %q: %v", got, err)
			}
			cached, err := d.Install(t.Context(), "1", home, server.URL, InstallOptions{})
			if err != nil || cached.NormalizedVer != res.NormalizedVer || cached.HomeDir != res.HomeDir {
				t.Fatalf("cache: %+v %v", cached, err)
			}
		})
	}
}

func TestIndexedLanguages(t *testing.T) {
	for _, name := range []string{"go", "julia"} {
		t.Run(name, func(t *testing.T) {
			downloads := 0
			payload := definitionArchive(t, "release.tar.gz", "release/bin/compiler", []byte("compiler"))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/":
					fmt.Fprintf(w, `[{"version":"go1.2.3","files":[{"filename":"release.tar.gz","os":%q,"arch":%q,"kind":"archive"}]}]`, runtime.GOOS, runtime.GOARCH)
				case "/versions.json":
					arch := "x86_64"
					if runtime.GOARCH == "arm64" {
						arch = "aarch64"
					}
					osName := runtime.GOOS
					if osName == "darwin" {
						osName = "mac"
					}
					fmt.Fprintf(w, `{"1.2.3":{"files":[{"url":"https://julialang-s3.julialang.org/bin/release.tar.gz","os":%q,"arch":%q,"extension":"tar.gz","kind":"archive"}]},"1.9.0":{"files":[{"url":"wrong","os":"wrong","arch":"wrong","extension":"tar.gz"}]}}`, osName, arch)
				case "/release.tar.gz":
					downloads++
					_, _ = w.Write(payload)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			home := t.TempDir()
			tool, _ := Get(name)
			res, err := tool.Install(t.Context(), "1.2", home, server.URL, InstallOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if res.NormalizedVer != "1.2.3" {
				t.Fatalf("version: %s", res.NormalizedVer)
			}
			cached, err := tool.Install(t.Context(), "1", home, server.URL, InstallOptions{})
			if err != nil || cached.NormalizedVer != "1.2.3" {
				t.Fatalf("cache: %+v %v", cached, err)
			}
			if downloads != 1 {
				t.Fatalf("downloads %d", downloads)
			}
			if name == "go" && cached.EnvVars["GOROOT"] != res.HomeDir {
				t.Fatal("missing cached GOROOT")
			}
		})
	}
}

func TestConfiguredLanguages(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix setup scripts")
	}
	for _, name := range []string{"rust", "ghc", "r"} {
		t.Run(name, func(t *testing.T) {
			script := `#!/bin/sh
prefix="${1#--prefix=}"
mkdir -p "$prefix/bin"
printf compiler > "$prefix/bin/compiler"
cat > Makefile <<'MAKE'
all:
	@true
install:
	@true
MAKE
`
			filename := "configure"
			if name == "rust" {
				filename = "install.sh"
			}
			payload := definitionArchive(t, "source.tar.gz", "source/"+filename, []byte(script))
			if name == "ghc" {
				gz, _ := gzip.NewReader(bytes.NewReader(payload))
				tarData, _ := io.ReadAll(gz)
				_ = gz.Close()
				var buf bytes.Buffer
				w, _ := xz.NewWriter(&buf)
				_, _ = w.Write(tarData)
				_ = w.Close()
				payload = buf.Bytes()
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, ".tar.gz") || strings.HasSuffix(r.URL.Path, ".tar.xz") {
					_, _ = w.Write(payload)
					return
				}
				_, _ = io.WriteString(w, `<a href="1.2.3/">release</a> R-1.2.3.tar.gz`)
			}))
			defer server.Close()
			localGitHubVersions(t, server)
			home := t.TempDir()
			tool, _ := Get(name)
			res, err := tool.Install(t.Context(), "1.2", home, server.URL, InstallOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = os.Stat(filepath.Join(res.HomeDir, "bin", "compiler")); err != nil {
				t.Fatal(err)
			}
			cached, err := tool.Install(t.Context(), "1", home, server.URL, InstallOptions{})
			if err != nil || cached.NormalizedVer != "1.2.3" {
				t.Fatalf("cache: %+v %v", cached, err)
			}
			if name == "r" && cached.EnvVars["R_HOME"] != filepath.Join(res.HomeDir, "lib", "R") {
				t.Fatal("incorrect R_HOME")
			}
		})
	}
}

func TestConfiguredFailureDoesNotCacheCandidate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix setup scripts")
	}
	payload := definitionArchive(t, "source.tar.gz", "source/install.sh", []byte("#!/bin/sh\nexit 1\n"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(payload) }))
	defer server.Close()
	localGitHubVersions(t, server)
	home := t.TempDir()
	_, err := (&RustTool{}).Install(t.Context(), "1.2", home, server.URL, InstallOptions{})
	if err == nil {
		t.Fatal("expected setup failure")
	}
	if _, ok := findExistingCandidate(home, "rust", "1", false); ok {
		t.Fatal("failed setup left a reusable candidate")
	}
}

func TestLanguageAssetNames(t *testing.T) {
	for _, tc := range []struct {
		d                               toolDefinition
		os, arch, version, path, binary string
	}{
		{groovyDefinition, "linux", "amd64", "4.0.24", "4.0.24/distribution/apache-groovy-binary-4.0.24.zip", ""},
		{kotlinDefinition, "windows", "arm64", "2.1.0", "v2.1.0/kotlin-compiler-2.1.0.zip", ""},
		{scalaDefinition, "darwin", "arm64", "2.13.16", "v2.13.16/scala-2.13.16.zip", ""},
		{elmDefinition, "windows", "amd64", "0.19.1", "0.19.1/binary-for-windows-64-bit.gz", "elm.exe"},
		{elmDefinition, "linux", "amd64", "0.19.1", "0.19.1/binary-for-linux-64-bit.gz", "elm"},
	} {
		_, path, binary, _, _ := tc.d.expand(tc.version, tc.os, tc.arch)
		if path != tc.path || binary != tc.binary {
			t.Errorf("%s: got %s %s, want %s %s", tc.d.name, path, binary, tc.path, tc.binary)
		}
	}
}
