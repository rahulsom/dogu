package tools

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestJavaCandidateDistribution(t *testing.T) {
	home := t.TempDir()
	for _, version := range []string{"17-zulu", "17.0.1-zulu", "17.0.2-temurin", "17.0.3-tem", "21.0.1-microsoft"} {
		if err := os.MkdirAll(filepath.Join(home, "candidates", "java", version, "bin"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct{ spec, want string }{
		{"17-zulu", "17.0.1-zulu"},
		{"17-tem", "17.0.3-tem"},
		{"17-temurin", "17.0.3-tem"},
		{"", "21.0.1-microsoft"},
	} {
		dir, ok := findExistingCandidate(home, "java", tc.spec, false)
		if !ok || filepath.Base(dir) != tc.want {
			t.Fatalf("spec=%q: %s, %v; want %s", tc.spec, dir, ok, tc.want)
		}
	}
}

func TestNestedExecutableReuse(t *testing.T) {
	for _, name := range []string{"bun", "uv"} {
		for _, nested := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/nested=%v", name, nested), func(t *testing.T) {
				home := t.TempDir()
				dir := filepath.Join(home, "candidates", name, "1.2.3")
				binDir := dir
				if nested {
					binDir = filepath.Join(dir, "distribution")
				}
				if err := os.MkdirAll(binDir, 0755); err != nil {
					t.Fatal(err)
				}
				binary := name
				if runtime.GOOS == "windows" {
					binary += ".exe"
				}
				if err := os.WriteFile(filepath.Join(binDir, binary), []byte("fixture"), 0755); err != nil {
					t.Fatal(err)
				}
				tool, _ := Get(name)
				res, err := tool.Install(t.Context(), "", home, "", InstallOptions{})
				if err != nil || res.HomeDir != binDir || len(res.BinDirs) != 1 || res.BinDirs[0] != binDir {
					t.Fatalf("reuse lost executable directory: %+v, %v", res, err)
				}
			})
		}
	}
}

func TestPythonFallbackAfterSelectionFailure(t *testing.T) {
	diagramHTTP(t, func(r *http.Request) (string, int) {
		return `[{"tag_name":"20261101","assets":[{"name":"cpython-9.0.0-20261101-wrong-target-install_only.tar.gz"}]}]`, 200
	})
	for _, spec := range []string{"3", "3.12", "3.12.1"} {
		version, tag, filename, err := resolvePythonAsset(t.Context(), spec, false)
		if err != nil || version != "3.12.1" || tag != "20240107" || filename != "cpython-3.12.1+20240107-"+mapPythonTarget()+"-install_only.tar.gz" {
			t.Fatalf("spec=%s: %s, %s, %s, %v", spec, version, tag, filename, err)
		}
	}
	if _, _, _, err := resolvePythonAsset(t.Context(), "", false); err == nil {
		t.Fatal("empty spec must not use the legacy fallback")
	}
}

func TestJavaStaticMirrorPartialResolution(t *testing.T) {
	discoveries := 0
	diagramHTTP(t, func(r *http.Request) (string, int) {
		discoveries++
		if r.URL.Host != "api.foojay.io" {
			t.Fatalf("unexpected discovery: %s", r.URL)
		}
		return `{"result":[{"java_version":"25.0.2","distribution":"temurin","filename":"jdk.tar.gz","links":{"pkg_download_redirect":"https://example.test/jdk.tar.gz"}}]}`, 200
	})
	for _, spec := range []string{"", "25", "25.0", "25.0.2"} {
		url, _, version, dist, err := (&JavaTool{}).resolveDownloadURL(t.Context(), spec, "", "linux", "x64", "https://mirror.test", false)
		wantDist := "temurin"
		if spec == "25.0.2" {
			wantDist = ""
		}
		suffix := ""
		if wantDist != "" {
			suffix = "-" + wantDist
		}
		if err != nil || version != "25.0.2" || dist != wantDist || url != "https://mirror.test/java-25.0.2"+suffix+"-linux-x64.tar.gz" {
			t.Fatalf("spec=%q: %s, %s, %s, %v", spec, url, version, dist, err)
		}
	}
	if discoveries != 3 {
		t.Fatalf("discoveries=%d; exact requests should not need discovery", discoveries)
	}
}

func TestMavenMajorArchives(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<a href="maven-3/">3</a><a href="maven-4/">4</a>`)
		case "/maven-3/":
			fmt.Fprint(w, `<a href="3.9.9/">release</a>`)
		case "/maven-4/":
			fmt.Fprint(w, `<a href="4.0.0-rc-4/">prerelease</a>`)
		case "/maven-3/3.9.9/binaries/apache-maven-3.9.9-bin.tar.gz",
			"/maven-4/4.0.0-rc-4/binaries/apache-maven-4.0.0-rc-4-bin.tar.gz":
			_, _ = w.Write(definitionArchive(t, "maven.tar.gz", "maven/bin/mvn", []byte("fixture")))
		default:
			t.Errorf("unexpected Maven request: %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	for _, tc := range []struct {
		spec, want string
		prerelease bool
	}{
		{"", "3.9.9", false},
		{"", "4.0.0-rc-4", true},
		{"4", "4.0.0-rc-4", true},
		{"4.0.0-rc-4", "4.0.0-rc-4", false},
	} {
		res, err := (&MavenTool{}).Install(t.Context(), tc.spec, t.TempDir(), server.URL+"/maven-3", InstallOptions{IncludePrerelease: tc.prerelease})
		if err != nil || res.NormalizedVer != tc.want {
			t.Fatalf("spec=%q, prerelease=%v: %+v, %v", tc.spec, tc.prerelease, res, err)
		}
	}
}

func TestStableSelectionFormats(t *testing.T) {
	for _, tc := range []struct {
		version string
		rc      bool
	}{
		{"3.0.0-1", true}, {"2.0.0-pre.1", true}, {"21.0.1-microsoft", false},
		{"25.0.1-temurin", false}, {"25-ea", true}, {"25-ea-microsoft", true},
	} {
		if got := IsRC(tc.version); got != tc.rc {
			t.Fatalf("IsRC(%q)=%v; want %v", tc.version, got, tc.rc)
		}
	}
	best, err := SelectBestVersion([]string{"21.0.1-microsoft", "25.0.0-1", "26.0.0-pre.1"}, "", false)
	if err != nil || best != "21.0.1-microsoft" {
		t.Fatalf("stable selection=%s, %v", best, err)
	}
}

func TestFailedExtractionIsNotReusable(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "bin/tool", Mode: 0755, Size: 3}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("bad")); err != nil {
		t.Fatal(err)
	}
	if err := tw.WriteHeader(&tar.Header{Name: "bin/incomplete", Mode: 0755, Size: 100}); err != nil {
		t.Fatal(err)
	}
	// Close the compression stream with a deliberately truncated second entry.
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	payload := buf.Bytes()
	diagramHTTP(t, func(r *http.Request) (string, int) {
		return string(payload), 200
	})
	home := t.TempDir()
	cfg := ArchiveInstallConfig{
		ToolName: "dummy", VersionSpec: "", NormalizedVer: "9.0.0", DoguHome: home,
		DownloadURL: "https://example.test/tool.tar.gz", FileName: "tool.tar.gz",
	}
	if _, err := installArchiveTool(t.Context(), cfg); err == nil {
		t.Fatal("expected extraction failure")
	}
	if _, ok := findExistingCandidate(home, "dummy", "", false); ok {
		t.Fatal("incomplete extracted candidate is reusable")
	}
	if _, err := os.Stat(filepath.Join(home, "candidates", "dummy", "9.0.0")); !os.IsNotExist(err) {
		t.Fatalf("failed candidate still exists: %v", err)
	}
	payload = definitionArchive(t, "tool.tar.gz", "bin/tool", []byte("valid"))
	if _, err := installArchiveTool(t.Context(), cfg); err != nil {
		t.Fatalf("retry reused a broken archive: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(home, "candidates", "dummy", "9.0.0")); err != nil {
		t.Fatal(err)
	}
	// Failed setup is also ineligible, including while its hook is running.
	cfg.NormalizedVer = "10.0.0"
	cfg.PostInstall = func(res *InstallResult) error {
		if _, ok := findExistingCandidate(home, "dummy", "", false); ok {
			t.Error("candidate became reusable before setup completed")
		}
		return fmt.Errorf("setup failed")
	}
	if _, err := installArchiveTool(t.Context(), cfg); err == nil {
		t.Fatal("expected setup failure")
	}
	if _, ok := findExistingCandidate(home, "dummy", "", false); ok {
		t.Fatal("failed setup candidate is reusable")
	}
}
