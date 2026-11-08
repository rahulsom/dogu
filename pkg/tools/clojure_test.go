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

func TestClojureVersions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		fmt.Fprint(w, `<a href="clojure-tools-1.11.1.1413.tar.gz">old</a>
<a href="clojure-tools-1.12.0.1530.tar.gz">stable</a>
<a href="clojure-tools-1.13.0-alpha1.tar.gz">alpha</a>`)
	}))
	defer server.Close()

	versions, err := ListVersions(t.Context(), "clojure", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"1.13.0-alpha1", "1.12.0.1530", "1.11.1.1413"}; !reflect.DeepEqual(versions, want) {
		t.Fatalf("versions %v, want %v", versions, want)
	}
}

func TestClojureInstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Clojure CLI setup script requires a Unix shell")
	}
	installScript := `#!/bin/sh
mkdir -p "$1/bin"
printf '#!/bin/sh\nprintf clojure\n' > "$1/bin/clojure"
printf '#!/bin/sh\nprintf clj\n' > "$1/bin/clj"
chmod +x "$1/bin/clojure" "$1/bin/clj"
`
	payload := definitionArchive(t, "clojure-tools-1.12.0.1530.tar.gz", "clojure-tools/install.sh", []byte(installScript))
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<a href="clojure-tools-1.12.0.1530.tar.gz">stable</a>`)
		case "/clojure-tools-1.12.0.1530.tar.gz":
			_, _ = w.Write(payload)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	home := t.TempDir()
	tool, err := FindTool("clojure")
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := tool.Install(t.Context(), "", home, server.URL, InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if fresh.NormalizedVer != "1.12.0.1530" || fresh.EnvVars["CLOJURE_HOME"] != fresh.HomeDir ||
		fresh.EnvVars["CLOJURE_1_HOME"] != fresh.HomeDir {
		t.Fatalf("unexpected install result: %+v", fresh)
	}
	if _, err := os.Stat(filepath.Join(fresh.BinDirs[0], "clojure")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(fresh.BinDirs[0], "clj")); err != nil {
		t.Fatal(err)
	}

	requestsAfterInstall := requests
	cached, err := tool.Install(t.Context(), "1", home, "http://127.0.0.1:1", InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if cached.NormalizedVer != fresh.NormalizedVer || cached.HomeDir != fresh.HomeDir ||
		!reflect.DeepEqual(cached.EnvVars, fresh.EnvVars) || requests != requestsAfterInstall {
		t.Fatalf("cache mismatch: %+v vs %+v; requests %d -> %d", cached, fresh, requestsAfterInstall, requests)
	}

	if err := os.WriteFile(filepath.Join(fresh.BinDirs[0], "clojure"), []byte("modified"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := tool.Install(t.Context(), "1.12.0.1530", home, server.URL, InstallOptions{Force: true}); err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(filepath.Join(fresh.BinDirs[0], "clojure")); err != nil || !strings.Contains(string(content), "printf clojure") {
		t.Fatalf("force did not reinstall launcher: %q, %v", content, err)
	}
}

func TestLeiningenInstall(t *testing.T) {
	payload := []byte("standalone jar")
	downloads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/1.2.3/leiningen-1.2.3-standalone.jar" {
			t.Errorf("unexpected download path %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		downloads++
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	requests := localGitHubVersions(t, server)

	home := t.TempDir()
	tool, err := FindTool("leiningen")
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := tool.Install(t.Context(), "1.2", home, server.URL, InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if fresh.NormalizedVer != "1.2.3" || fresh.EnvVars["LEIN_HOME"] != fresh.HomeDir ||
		fresh.EnvVars["LEININGEN_1_HOME"] != fresh.HomeDir {
		t.Fatalf("unexpected install result: %+v", fresh)
	}
	jar := filepath.Join(fresh.HomeDir, "leiningen-1.2.3-standalone.jar")
	if content, err := os.ReadFile(jar); err != nil || string(content) != string(payload) {
		t.Fatalf("standalone jar %q: %v", content, err)
	}
	launcher, err := os.ReadFile(filepath.Join(fresh.HomeDir, "lein"))
	if err != nil || !strings.Contains(string(launcher), "clojure.main -m leiningen.core.main") {
		t.Fatalf("Unix launcher %q: %v", launcher, err)
	}
	batch, err := os.ReadFile(filepath.Join(fresh.HomeDir, "lein.bat"))
	if err != nil || !strings.Contains(string(batch), "clojure.main -m leiningen.core.main") {
		t.Fatalf("Windows launcher %q: %v", batch, err)
	}

	requestsAfterInstall := *requests
	cached, err := tool.Install(t.Context(), "1", home, server.URL, InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if cached.NormalizedVer != fresh.NormalizedVer || cached.HomeDir != fresh.HomeDir ||
		!reflect.DeepEqual(cached.EnvVars, fresh.EnvVars) || *requests != requestsAfterInstall {
		t.Fatalf("cache mismatch: %+v vs %+v; requests %d -> %d", cached, fresh, requestsAfterInstall, *requests)
	}
	if downloads != 1 {
		t.Fatalf("downloads = %d, want 1", downloads)
	}
}
