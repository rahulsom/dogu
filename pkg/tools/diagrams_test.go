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
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// All discovery and download traffic stays in memory; no upstream releases or
// package managers are needed to exercise installation and failure cleanup.
func diagramHTTP(t *testing.T, handler func(*http.Request) (string, int)) {
	t.Helper()
	old := http.DefaultTransport
	http.DefaultTransport = definitionTransport(func(r *http.Request) (*http.Response, error) {
		body, status := handler(r)
		return &http.Response{StatusCode: status, Status: fmt.Sprintf("%d", status), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = old })
}

func TestDiagramVersions(t *testing.T) {
	for _, d := range diagramTools {
		t.Run(d.name, func(t *testing.T) {
			if _, ok := Get(d.name); !ok {
				t.Fatal("provider not registered")
			}
			diagramHTTP(t, func(r *http.Request) (string, int) {
				switch d.kind {
				case "npm":
					if r.URL.Path != "/"+d.pkg {
						t.Errorf("wrong npm package: %s", r.URL.Path)
					}
					return `{"versions":{"1.2.3":{},"1.3.0":{},"1.4.0-rc1":{}}}`, 200
				case "browser":
					return `[{"name":"v1.2.3"},{"name":"v1.3.0"}]`, 200
				case "gem":
					return `[{"number":"1.2.3"},{"number":"1.3.0"},{"number":"1.4.0-rc1"}]`, 200
				case "go":
					return "v1.2.3\nv1.3.0\nv1.4.0-rc1\n", 200
				case "gitlab":
					if r.URL.Path != "/api/v4/projects/18695663/releases" {
						t.Errorf("wrong LilyPond project: %s", r.URL)
					}
					return `[{"tag_name":"v1.2.3"},{"tag_name":"v1.3.0"}]`, 200
				case "cabal":
					return `{"normal-version":["1.2.3","1.3.0","1.4.0-rc1"]}`, 200
				case "source":
					if d.name == "gnuplot" {
						return `<a href="/projects/gnuplot/files/gnuplot/1.2.3/">1.2.3</a><a href="/gnuplot/1.3.0/">1.3.0</a>`, 200
					}
					return `graphviz-1.2.3.tar.gz graphviz-1.3.0.tar.gz graphviz-1.3.0.tar.gz`, 200
				default:
					if strings.Contains(r.URL.RawQuery, "page=2") {
						return `[]`, 200
					}
					return `[{"tag_name":"v1.2.3","assets":[{}]},{"tag_name":"v1.3.0","assets":[{}]}]`, 200
				}
			})
			got, err := ListVersions(context.Background(), d.name, "")
			if err != nil {
				t.Fatal(err)
			}
			if len(got) < 2 || CleanVersion(got[len(got)-1]) != "1.2.3" {
				t.Fatalf("bad sorted versions: %v", got)
			}
		})
	}
}

func TestDiagramPackageInstallation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX installers")
	}
	for _, name := range []string{"bytefield-svg", "barby", "asciitosvg", "erd"} {
		t.Run(name, func(t *testing.T) {
			d, _ := diagramDefinition(name)
			bin := t.TempDir()
			log := filepath.Join(bin, "args")
			command := map[string]string{"npm": "npm", "gem": "gem", "go": "go", "cabal": "cabal"}[d.kind]
			// The fake package manager records its arguments and materializes a payload.
			script := "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$DIAGRAM_LOG\"\n[ \"$DIAGRAM_FAIL\" != 1 ] || exit 1\nmkdir -p node_modules/.bin bin gems\necho payload > bin/tool\n"
			if err := os.WriteFile(filepath.Join(bin, command), []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("DIAGRAM_LOG", log)
			diagramHTTP(t, func(r *http.Request) (string, int) {
				switch d.kind {
				case "npm":
					return `{"versions":{"1.2.3":{},"1.2.4":{},"1.3.0-rc1":{}}}`, 200
				case "gem":
					return `[{"number":"1.2.3"},{"number":"1.2.4"}]`, 200
				case "go":
					return "v1.2.3\nv1.2.4\n", 200
				default:
					return `{"normal-version":["1.2.3","1.2.4"]}`, 200
				}
			})
			home := t.TempDir()
			r, err := d.Install(context.Background(), "", home, "", InstallOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if r.NormalizedVer != "1.2.4" {
				t.Fatal(r.NormalizedVer)
			}
			args, _ := os.ReadFile(log)
			if !strings.Contains(string(args), "1.2.4") {
				t.Fatalf("unresolved install: %s", args)
			}
			reused, err := d.Install(context.Background(), "1", home, "", InstallOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(r.EnvVars, reused.EnvVars) || !reflect.DeepEqual(r.BinDirs, reused.BinDirs) {
				t.Fatalf("reuse changed metadata: %#v %#v", r, reused)
			}
			again, _ := os.ReadFile(log)
			if string(again) != string(args) {
				t.Fatal("reused candidate ran setup")
			}
			t.Setenv("DIAGRAM_FAIL", "1")
			if _, err := d.Install(context.Background(), "1.2", home, "", InstallOptions{Force: true}); err == nil {
				t.Fatal("expected setup failure")
			}
			if _, err := os.Stat(r.InstallDir); !os.IsNotExist(err) {
				t.Fatal("failed candidate remains cached")
			}
		})
	}
}

func TestDiagramJarInstallation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX launcher")
	}
	for _, name := range []string{"plantuml", "ditaa"} {
		t.Run(name, func(t *testing.T) {
			d, _ := diagramDefinition(name)
			asset := "plantuml-1.2.3.jar"
			if name == "ditaa" {
				asset = "ditaa-1.2.3-standalone.jar"
			}
			count := 0
			diagramHTTP(t, func(r *http.Request) (string, int) {
				count++
				switch {
				case strings.Contains(r.URL.Path, "/releases/tags/"):
					return fmt.Sprintf(`{"assets":[{"name":%q}]}`, asset), 200
				case r.URL.Host == "api.github.com":
					return `[{"tag_name":"v1.2.3","assets":[{}]}]`, 200
				default:
					return "jar fixture", 200
				}
			})
			home := t.TempDir()
			r, err := d.Install(context.Background(), "1.2", home, "", InstallOptions{})
			if err != nil {
				t.Fatal(err)
			}
			script, err := os.ReadFile(filepath.Join(r.InstallDir, name))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(script), `exec java -jar "$DIR/`+asset+`" "$@"`) {
				t.Fatal(string(script))
			}
			before := count
			if _, err = d.Install(context.Background(), "1", home, "", InstallOptions{}); err != nil {
				t.Fatal(err)
			}
			if count != before {
				t.Fatal("reused jar made HTTP requests")
			}
		})
	}
}

func TestDiagramSourceBuild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX source build")
	}
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	configure := []byte("#!/bin/sh\n[ \"$2\" = --disable-swig ] || exit 3\necho fixture\n")
	if err := tw.WriteHeader(&tar.Header{Name: "graphviz-1.2.3/configure", Mode: 0755, Size: int64(len(configure))}); err != nil {
		t.Fatal(err)
	}
	tw.Write(configure)
	tw.Close()
	gz.Close()
	diagramHTTP(t, func(r *http.Request) (string, int) {
		if strings.HasSuffix(r.URL.Path, ".tar.gz") {
			return b.String(), 200
		}
		return "graphviz-1.2.3.tar.gz", 200
	})
	bin := t.TempDir()
	// configure is found inside the source directory, and installation uses the
	// final prefix, rather than mistaking extracted sources for installed tools.
	script := "#!/bin/sh\n[ -f configure ] || exit 2\n"
	os.WriteFile(filepath.Join(bin, "make"), []byte(script), 0755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	d, _ := diagramDefinition("graphviz")
	if _, err := d.Install(context.Background(), "1", t.TempDir(), "", InstallOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestDiagramAssetNames(t *testing.T) {
	for _, tc := range []struct{ name, asset, os, arch string }{
		{"d2", "d2-v1.2.3-linux-amd64.tar.gz", "linux", "amd64"},
		{"d2", "d2-v1.2.3-macos-arm64.tar.gz", "darwin", "arm64"},
		{"ditaa", "ditaa-1.2.3-standalone.jar", "linux", "arm64"},
		{"plantuml", "plantuml-1.2.3.jar", "darwin", "amd64"},
		{"jsyntrax", "jsyntrax-1.2.3-syntrax.zip", "linux", "amd64"},
	} {
		if !diagramAssetMatches(tc.name, tc.asset, "1.2.3", tc.os, tc.arch) {
			t.Fatal(tc)
		}
	}
	d, _ := diagramDefinition("lilypond")
	asset, _, err := d.asset(context.Background(), "v1.2.3", "1.2.3", d.base, "darwin", "arm64")
	if err != nil || asset != "lilypond-1.2.3-darwin-arm64.tar.gz" {
		t.Fatalf("%s %v", asset, err)
	}
	if _, _, err = d.asset(context.Background(), "v1.2.3", "1.2.3", d.base, "linux", "arm64"); err == nil {
		t.Fatal("unsupported host accepted")
	}
}

func TestDiagramArchiveInstallation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX installers")
	}
	for _, name := range []string{"d2", "jsyntrax", "lilypond"} {
		t.Run(name, func(t *testing.T) {
			d, _ := diagramDefinition(name)
			asset, entry := "jsyntrax-1.2.3-syntrax.zip", "jsyntrax-1.2.3/bin/syntrax"
			switch name {
			case "d2":
				osName := runtime.GOOS
				if osName == "darwin" {
					osName = "macos"
				}
				asset = fmt.Sprintf("d2-v1.2.3-%s-%s.tar.gz", osName, runtime.GOARCH)
				entry = "d2-v1.2.3/bin/d2"
			case "lilypond":
				var err error
				asset, _, err = d.asset(context.Background(), "v1.2.3", "1.2.3", d.base, runtime.GOOS, runtime.GOARCH)
				if err != nil {
					t.Skip(err)
				}
				entry = "lilypond-1.2.3/bin/lilypond"
			}
			payload := definitionArchive(t, asset, entry, []byte("fixture"))
			if name == "jsyntrax" {
				var archive bytes.Buffer
				zw := zip.NewWriter(&archive)
				header := &zip.FileHeader{Name: entry}
				header.SetMode(0644)
				writer, err := zw.CreateHeader(header)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := writer.Write([]byte("fixture")); err != nil {
					t.Fatal(err)
				}
				if err := zw.Close(); err != nil {
					t.Fatal(err)
				}
				payload = archive.Bytes()
			}
			diagramHTTP(t, func(r *http.Request) (string, int) {
				switch {
				case strings.Contains(r.URL.Path, "/releases/tags/"):
					return fmt.Sprintf(`{"assets":[{"name":%q}]}`, asset), 200
				case strings.Contains(r.URL.Path, "/releases") && (r.URL.Host == "api.github.com" || strings.Contains(r.URL.Path, "/api/v4/")):
					return `[{"tag_name":"v1.2.3","assets":[{}]}]`, 200
				default:
					return string(payload), 200
				}
			})
			home := t.TempDir()
			r, err := d.Install(context.Background(), "1", home, "", InstallOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = os.Stat(filepath.Join(r.InstallDir, entry)); err != nil {
				t.Fatal(err)
			}
			if name == "jsyntrax" {
				info, err := os.Stat(filepath.Join(r.InstallDir, entry))
				if err != nil || info.Mode()&0111 == 0 {
					t.Fatalf("launcher is not executable: %v", err)
				}
				if err := os.Chmod(filepath.Join(r.InstallDir, entry), 0644); err != nil {
					t.Fatal(err)
				}
			}
			reused, err := d.Install(context.Background(), "1.2", home, "", InstallOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if name == "jsyntrax" {
				info, err := os.Stat(filepath.Join(r.InstallDir, entry))
				if err != nil || info.Mode()&0111 == 0 {
					t.Fatalf("cached launcher is not executable: %v", err)
				}
			}
			if !reflect.DeepEqual(r.BinDirs, reused.BinDirs) || !reflect.DeepEqual(r.EnvVars, reused.EnvVars) {
				t.Fatal("cached archive metadata changed")
			}
		})
	}
}

func TestDiagramDiscoveryErrors(t *testing.T) {
	for _, name := range []string{"barby", "asciitosvg", "erd", "graphviz", "lilypond"} {
		t.Run(name, func(t *testing.T) {
			d, _ := diagramDefinition(name)
			diagramHTTP(t, func(*http.Request) (string, int) { return "unavailable", 503 })
			if _, err := d.Install(context.Background(), "1", t.TempDir(), "", InstallOptions{}); err == nil {
				t.Fatal("discovery failure ignored")
			}
		})
	}
}

func TestAsciiToSVGPseudoVersion(t *testing.T) {
	d, _ := diagramDefinition("asciitosvg")
	diagramHTTP(t, func(r *http.Request) (string, int) {
		if strings.HasSuffix(r.URL.Path, "/@v/list") {
			return "", 200
		}
		return `{"Version":"v0.0.0-20240101000000-cdef12345678"}`, 200
	})
	got, err := d.versions(context.Background(), "")
	if err != nil || len(got) != 1 || !strings.HasPrefix(got[0], "v0.0.0-") {
		t.Fatalf("%v %v", got, err)
	}
}

func TestERDParserCompatibility(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX package manager")
	}
	for _, tc := range []struct {
		name, version string
		badSource     bool
	}{
		{"released source", "0.2.1.0", false}, {"other release", "1.2.4", false}, {"unexpected source", "0.2.1.0", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin := t.TempDir()
			log := filepath.Join(bin, "args")
			script := `#!/bin/sh
printf '%s\n' "$@" >> "$ERD_ARGS"
case "$1" in
 unpack)
  mkdir -p erd-0.2.1.0/src/Text/Parsec/Erd
  if [ "$ERD_BAD_SOURCE" = 1 ]; then
   echo 'import Data.Other' > erd-0.2.1.0/src/Text/Parsec/Erd/Parser.hs
  else
   printf 'module Parser where\nimport           Data.Text.Lazy\n' > erd-0.2.1.0/src/Text/Parsec/Erd/Parser.hs
  fi
  ;;
 install)
  if [ "$2" = . ]; then
   grep -q '^import           Data.Text.Lazy hiding (elem)$' src/Text/Parsec/Erd/Parser.hs || exit 3
   pwd > "$ERD_SOURCE_DIR"
  fi
  bindir=
  for arg in "$@"; do
   case "$arg" in --installdir=*) bindir="${arg#--installdir=}" ;; esac
  done
  [ -n "$bindir" ] || exit 4
  mkdir -p "$bindir"
  printf fixture > "$bindir/erd"
  ;;
esac
`
			if err := os.WriteFile(filepath.Join(bin, "cabal"), []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("ERD_ARGS", log)
			sourceLog := filepath.Join(bin, "source")
			t.Setenv("ERD_SOURCE_DIR", sourceLog)
			if tc.badSource {
				t.Setenv("ERD_BAD_SOURCE", "1")
			} else {
				t.Setenv("ERD_BAD_SOURCE", "0")
			}
			diagramHTTP(t, func(*http.Request) (string, int) { return fmt.Sprintf(`{"normal-version":[%q]}`, tc.version), 200 })
			d, _ := diagramDefinition("erd")
			home := t.TempDir()
			result, err := d.Install(t.Context(), tc.version, home, "", InstallOptions{})
			if tc.badSource {
				if err == nil {
					t.Fatal("unexpected source was accepted")
				}
				if _, err := os.Stat(filepath.Join(home, "candidates", "erd", tc.version)); !os.IsNotExist(err) {
					t.Fatalf("failed candidate retained: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			binary, err := os.ReadFile(filepath.Join(result.BinDirs[0], "erd"))
			if err != nil || string(binary) != "fixture" {
				t.Fatalf("missing private executable: %q (%v)", binary, err)
			}
			args, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			patched := tc.version == "0.2.1.0"
			if strings.Contains(string(args), "unpack\n") != patched || strings.Contains(string(args), "install\n.\n") != patched {
				t.Fatalf("incorrect version-specific setup: %s", args)
			}
			if patched {
				source, err := os.ReadFile(sourceLog)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(strings.TrimSpace(string(source))); !os.IsNotExist(err) {
					t.Fatalf("temporary source retained: %v", err)
				}
			}
			if _, err := d.Install(t.Context(), tc.version, home, "", InstallOptions{}); err != nil {
				t.Fatal(err)
			}
			reusedArgs, err := os.ReadFile(log)
			if err != nil || string(reusedArgs) != string(args) {
				t.Fatalf("reuse ran setup: %s (%v)", reusedArgs, err)
			}
			if _, err := d.Install(t.Context(), tc.version, home, "", InstallOptions{Force: true}); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(result.InstallDir, ".dogu-complete")); err != nil {
				t.Fatal(err)
			}
		})
	}
}
