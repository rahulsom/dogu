package tools

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func sourceLanguageIndex(name string) string {
	if name == "ruby" {
		return "name\turl\tsha256\n" +
			"ruby-3.4.1\thttps://cache.ruby-lang.org/pub/ruby/3.4/ruby-3.4.1.tar.gz\thash\n" +
			"ruby-3.4.2\thttps://cache.ruby-lang.org/pub/ruby/3.4/ruby-3.4.2.tar.gz\thash\n" +
			"ruby-3.4.3-preview1\thttps://cache.ruby-lang.org/pub/ruby/3.4/ruby-3.4.3-preview1.tar.gz\thash\n"
	}
	return `<a href="lua-3.4.1.tar.gz">old</a>
<A HREF="lua-3.4.2.tar.gz">stable</A>
<a href="lua-3.4.2.tar.gz">duplicate</a>
<a href="lua-3.4.3-beta1.tar.gz">beta</a>
<a href="lua-3.4.9.tar.gz.asc">signature only</a>`
}

func TestSourceLanguageVersions(t *testing.T) {
	for _, name := range []string{"ruby", "lua"} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expected := "/"
				if name == "ruby" {
					expected = "/index.txt"
				}
				if r.URL.Path != expected {
					t.Errorf("unexpected index path %s", r.URL.Path)
				}
				fmt.Fprint(w, sourceLanguageIndex(name))
			}))
			defer server.Close()
			got, err := ListVersions(t.Context(), name, server.URL+"/")
			if err != nil {
				t.Fatal(err)
			}
			pre := "3.4.3-beta1"
			if name == "ruby" {
				pre = "3.4.3-preview1"
			}
			if want := []string{pre, "3.4.2", "3.4.1"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("versions %v, want %v", got, want)
			}
		})
	}
}

func TestSourceLanguageInstall(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("Unix source builds")
	}
	for _, name := range []string{"ruby", "lua"} {
		t.Run(name, func(t *testing.T) {
			target := "linux"
			if runtime.GOOS == "darwin" {
				target = "macosx"
			}
			makefile := fmt.Sprintf(`%s:
	@printf built > built
install:
	@test -f built
	@mkdir -p "$(INSTALL_TOP)/bin"
	@printf '%s' > "$(INSTALL_TOP)/bin/%s"
`, target, name, name)
			entry := "source/Makefile"
			script := makefile
			if name == "ruby" {
				entry = "source/configure"
				script = `#!/bin/sh
test "$2" = "--disable-install-doc" || exit 1
printf 'INSTALL_TOP := %s\n' "${1#--prefix=}" > Makefile
cat >> Makefile <<'MAKE'
` + makefile + "MAKE\n"
			}
			payload := definitionArchive(t, "source.tar.gz", entry, []byte(script))
			downloads := 0
			fail := false
			version := "3.4.2"
			failureEntry, failureData := "source/Makefile", "linux macosx:\n\t@false\n"
			if name == "ruby" {
				failureEntry, failureData = "source/configure", "#!/bin/sh\nexit 1\n"
			}
			failurePayload := definitionArchive(t, "source.tar.gz", failureEntry, []byte(failureData))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, ".tar.gz") {
					path := "/lua-" + version + ".tar.gz"
					if name == "ruby" {
						path = "/3.4/ruby-" + version + ".tar.gz"
					}
					if r.URL.Path != path {
						t.Errorf("download path %s, want %s", r.URL.Path, path)
					}
					downloads++
					if fail {
						w.Write(failurePayload)
					} else {
						w.Write(payload)
					}
					return
				}
				fmt.Fprint(w, sourceLanguageIndex(name))
			}))
			defer server.Close()
			home := t.TempDir()
			tool, err := FindTool(name)
			if err != nil {
				t.Fatal(err)
			}
			res, err := tool.Install(t.Context(), "", home, server.URL, InstallOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if res.NormalizedVer != "3.4.2" {
				t.Fatalf("version %s", res.NormalizedVer)
			}
			data, err := os.ReadFile(filepath.Join(res.BinDirs[0], name))
			if err != nil || string(data) != name {
				t.Fatalf("installed payload %q: %v", data, err)
			}
			cached, err := tool.Install(t.Context(), "3", home, "http://127.0.0.1:1", InstallOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if cached.NormalizedVer != res.NormalizedVer || cached.HomeDir != res.HomeDir ||
				!reflect.DeepEqual(cached.EnvVars, res.EnvVars) || downloads != 1 {
				t.Fatalf("cache mismatch: %+v vs %+v, downloads %d", cached, res, downloads)
			}
			for _, key := range []string{strings.ToUpper(name) + "_HOME", strings.ToUpper(name) + "_3_HOME"} {
				if res.EnvVars[key] != res.HomeDir {
					t.Errorf("%s = %q", key, res.EnvVars[key])
				}
			}
			binary := filepath.Join(res.BinDirs[0], name)
			if err := os.WriteFile(binary, []byte("modified"), 0755); err != nil {
				t.Fatal(err)
			}
			if _, err := tool.Install(t.Context(), "3.4.2", home, server.URL, InstallOptions{Force: true}); err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(binary); err != nil || string(data) != name {
				t.Fatalf("force did not rebuild: %q, %v", data, err)
			}
			version = "3.4.3-beta1"
			if name == "ruby" {
				version = "3.4.3-preview1"
			}
			pre, err := tool.Install(t.Context(), "3.4", t.TempDir(), server.URL, InstallOptions{IncludePrerelease: true})
			if err != nil || pre.NormalizedVer != version {
				t.Fatalf("prerelease: %+v, %v", pre, err)
			}
			version = "3.4.2"
			fail = true
			failedHome := t.TempDir()
			if _, err := tool.Install(t.Context(), "3.4.2", failedHome, server.URL, InstallOptions{}); err == nil {
				t.Fatal("expected setup failure")
			}
			if _, ok := findExistingCandidate(failedHome, name, "3", false); ok {
				t.Fatal("failed build left a reusable candidate")
			}
		})
	}
}

func TestSourceLanguageDiscoveryErrors(t *testing.T) {
	for _, name := range []string{"ruby", "lua"} {
		for _, status := range []int{http.StatusNotFound, http.StatusOK} {
			t.Run(fmt.Sprintf("%s/%d", name, status), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(status)
					fmt.Fprint(w, "no releases")
				}))
				defer server.Close()
				if _, err := ListVersions(t.Context(), name, server.URL); err == nil {
					t.Fatal("expected discovery error")
				}
				tool, _ := FindTool(name)
				home := t.TempDir()
				if _, err := tool.Install(t.Context(), "3.4", home, server.URL, InstallOptions{}); err == nil {
					t.Fatal("expected install error")
				}
				if _, ok := findExistingCandidate(home, name, "3", false); ok {
					t.Fatal("discovery error left a candidate")
				}
			})
		}
	}
}
