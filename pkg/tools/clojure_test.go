package tools

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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

func TestClojureOfficialVersions(t *testing.T) {
	original := http.DefaultTransport
	http.DefaultTransport = definitionTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.github.com" || r.URL.Path != "/repos/clojure/brew-install/releases" {
			t.Fatalf("unexpected discovery URL: %s", r.URL)
		}
		body := `[]`
		if r.URL.Query().Get("page") == "1" {
			body = `[{"tag_name":"1.12.0.1530","assets":[{}]}]`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = original })
	versions, err := ListVersions(t.Context(), "clojure", "")
	if err != nil || !reflect.DeepEqual(versions, []string{"1.12.0.1530"}) {
		t.Fatalf("versions %v: %v", versions, err)
	}
}

func TestClojureInstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Clojure CLI setup script requires a Unix shell")
	}
	installScript := `#!/bin/sh
mkdir -p "$1/bin"
cat > "$1/bin/clojure" <<'LAUNCHER'
#!/bin/sh
install_dir=PREFIX
printf '%s' "$install_dir"
LAUNCHER
cat > "$1/bin/clj" <<'LAUNCHER'
#!/bin/sh
bin_dir=BINDIR
printf '%s' "$bin_dir"
LAUNCHER
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

	home := filepath.Join(t.TempDir(), "dogu home ' PREFIX BINDIR $dollar")
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

	assertLaunchers := func() {
		t.Helper()
		for launcher, want := range map[string]string{"clojure": fresh.InstallDir, "clj": fresh.BinDirs[0]} {
			out, err := exec.Command(filepath.Join(fresh.BinDirs[0], launcher)).CombinedOutput()
			if err != nil || string(out) != want {
				t.Fatalf("%s output %q, want %q: %v", launcher, out, want, err)
			}
		}
	}
	assertLaunchers()
	prepared, err := os.ReadFile(filepath.Join(fresh.BinDirs[0], "clj"))
	if err != nil {
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

	assertLaunchers()
	clj, err := os.ReadFile(filepath.Join(fresh.BinDirs[0], "clj"))
	if err != nil || string(clj) != string(prepared) {
		t.Fatalf("cached launcher changed on reuse: %s (%v)", clj, err)
	}
	// Repair a launcher left unprepared by an older installation.
	if err := os.WriteFile(filepath.Join(fresh.BinDirs[0], "clj"), []byte("#!/bin/sh\nbin_dir=BINDIR\nprintf '%s' \"$bin_dir\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := tool.Install(t.Context(), "1", home, server.URL, InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	assertLaunchers()

	if err := os.WriteFile(filepath.Join(fresh.BinDirs[0], "clojure"), []byte("modified"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := tool.Install(t.Context(), "1.12.0.1530", home, server.URL, InstallOptions{Force: true}); err != nil {
		t.Fatal(err)
	}
	assertLaunchers()
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
