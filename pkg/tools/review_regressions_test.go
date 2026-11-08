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
	"strings"
	"sync"
	"testing"
	"time"
)

func TestJavaCandidateDistribution(t *testing.T) {
	home := t.TempDir()
	for _, version := range []string{"17-zulu", "17.0.1-zulu", "17.0.2-temurin", "17.0.3-tem", "21.0.1-microsoft", "26-ea-temurin"} {
		if err := os.MkdirAll(filepath.Join(home, "candidates", "java", version, "bin"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		spec, want string
		prerelease bool
	}{
		{"17-zulu", "17.0.1-zulu", false},
		{"17-tem", "17.0.3-tem", false},
		{"17-temurin", "17.0.3-tem", false},
		{"26-ea", "26-ea-temurin", false},
		{"26-temurin", "26-ea-temurin", true},
		{"", "21.0.1-microsoft", false},
	} {
		dir, ok := findExistingCandidate(home, "java", tc.spec, tc.prerelease)
		if !ok || filepath.Base(dir) != tc.want {
			t.Fatalf("spec=%q: %s, %v; want %s", tc.spec, dir, ok, tc.want)
		}
	}
}

func TestJavaBuildMetadataCandidateIsExact(t *testing.T) {
	home := t.TempDir()
	for _, version := range []string{"17.0.2-temurin", "17.0.2-zulu", "21+35-temurin", "21.0.1+12-temurin"} {
		dir := filepath.Join(home, "candidates", "java", version)
		if err := os.MkdirAll(filepath.Join(dir, "bin"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct{ spec, want string }{
		{"21+35-temurin", "21+35-temurin"},
		{"21+35", "21+35-temurin"},
		{"17.0.2-tem", "17.0.2-temurin"},
	} {
		dir, ok := findExistingCandidate(home, "java", tc.spec, false)
		if !ok || filepath.Base(dir) != tc.want {
			t.Fatalf("spec=%q: candidate=%s, %v; want %s", tc.spec, dir, ok, tc.want)
		}
	}
}

func TestJavaUnlistedDistributions(t *testing.T) {
	home := t.TempDir()
	for _, version := range []string{"21.0.1-kona", "21.0.2-mandrel", "21.0.3-zulu_prime", "26-ea-kona"} {
		if err := os.MkdirAll(filepath.Join(home, "candidates", "java", version, "bin"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct{ spec, want string }{
		{"21-kona", "21.0.1-kona"},
		{"21-mandrel", "21.0.2-mandrel"},
		{"21-zulu_prime", "21.0.3-zulu_prime"},
		{"26-ea-kona", "26-ea-kona"},
	} {
		dir, ok := findExistingCandidate(home, "java", tc.spec, true)
		if !ok || filepath.Base(dir) != tc.want {
			t.Fatalf("spec=%q: candidate=%s, %v; want %s", tc.spec, dir, ok, tc.want)
		}
	}
}

func TestExistingCandidateCanBeReusedWithoutWritableLockDirectory(t *testing.T) {
	home := t.TempDir()
	candidatesDir := filepath.Join(home, "candidates", "dummy")
	candidateDir := filepath.Join(candidatesDir, "1.2.3")
	if err := os.MkdirAll(filepath.Join(candidateDir, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chmod(candidatesDir, 0755)
		_ = os.Chmod(filepath.Dir(candidatesDir), 0755)
	}()
	if err := os.Chmod(candidatesDir, 0555); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Dir(candidatesDir), 0555); err != nil {
		t.Fatal(err)
	}
	dir, unlock, ok, err := lockExistingCandidate(t.Context(), home, "dummy", "1.2.3", false)
	if err != nil {
		t.Fatalf("reuse from a read-only candidate directory: %v", err)
	}
	if !ok || dir != candidateDir {
		t.Fatalf("candidate=%s, ok=%v; want %s", dir, ok, candidateDir)
	}
	unlock()
}

func TestLeiningenReuseRepairsMissingLauncher(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "candidates", "leiningen", "2.10.0")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "leiningen-2.10.0-standalone.jar"), []byte("fixture"), 0644); err != nil {
		t.Fatal(err)
	}
	res, err := (&LeiningenTool{}).Install(t.Context(), "2.10.0", home, "", InstallOptions{})
	if err != nil {
		t.Fatalf("reuse incomplete Leiningen candidate: %v", err)
	}
	for _, launcher := range []string{"lein", "lein.bat"} {
		if _, err := os.Stat(filepath.Join(dir, launcher)); err != nil {
			t.Errorf("missing repaired launcher %s: %v", launcher, err)
		}
	}
	if res.InstallDir != dir {
		t.Fatalf("reused install dir=%s; want %s", res.InstallDir, dir)
	}
}

func TestReplacementBackupIsOutsideCandidateSearchRoot(t *testing.T) {
	home := t.TempDir()
	dest := filepath.Join(home, "candidates", "dummy", "1.0.0")
	stage := filepath.Join(home, ".stage")
	if err := os.MkdirAll(dest, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "old"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stage, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "new"), []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := publishCandidate(stage, dest); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(home, "candidates", "dummy"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "1.0.0" {
		t.Fatalf("candidate search root contains unexpected entries: %v", entries)
	}
}

func TestDiagramReuseRequiresCompletionMarker(t *testing.T) {
	home := t.TempDir()
	for _, version := range []string{"1.0.0", "2.0.0"} {
		dir := filepath.Join(home, "candidates", "d2", version)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if version == "1.0.0" {
			if err := os.WriteFile(filepath.Join(dir, ".dogu-complete"), nil, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	dir, ok := findExistingCandidate(home, "d2", "", false)
	if !ok || filepath.Base(dir) != "1.0.0" {
		t.Fatalf("selected incomplete diagram candidate %s, %v", dir, ok)
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

func TestJavaStaticMirrorFallsBackWhenDiscoveryFails(t *testing.T) {
	diagramHTTP(t, func(r *http.Request) (string, int) {
		if r.URL.Host != "api.foojay.io" {
			t.Fatalf("unexpected request: %s", r.URL)
		}
		return "", http.StatusServiceUnavailable
	})
	for _, spec := range []string{"25", "25.0"} {
		url, _, version, dist, err := (&JavaTool{}).resolveDownloadURL(t.Context(), spec, "temurin", "linux", "x64", "https://mirror.test", false)
		if err != nil || version != spec || dist != "temurin" || !strings.HasPrefix(url, "https://mirror.test/java-"+spec+"-temurin-") {
			t.Fatalf("spec=%q: %s, %s, %s, %v", spec, url, version, dist, err)
		}
	}
	if _, _, _, _, err := (&JavaTool{}).resolveDownloadURL(t.Context(), "", "", "linux", "x64", "https://mirror.test", false); err == nil {
		t.Fatal("empty Java spec must not fall back after discovery failure")
	}
}

func TestJavaResolutionSkipsUndownloadablePackages(t *testing.T) {
	diagramHTTP(t, func(r *http.Request) (string, int) {
		if r.URL.Query().Get("directly_downloadable") != "true" {
			t.Errorf("missing directly_downloadable filter: %s", r.URL)
		}
		return `{"result":[
			{"java_version":"26.0.0","distribution":"temurin"},
			{"java_version":"25.0.1","distribution":"temurin","filename":"jdk.tar.gz","links":{"pkg_download_redirect":"https://example.test/jdk.tar.gz"}}]}`, 200
	})
	_, filename, version, _, err := (&JavaTool{}).resolveDownloadURL(t.Context(), "", "", "linux", "x64", "", false)
	if err != nil || version != "25.0.1" || filename != "jdk.tar.gz" {
		t.Fatalf("selected unusable Java package: %s, %s, %v", version, filename, err)
	}
	versions, err := listVersions(t.Context(), "java", "")
	if err != nil || !containsString(versions, "25.0.1-tem") || containsString(versions, "26.0.0-tem") {
		t.Fatalf("listed unusable Java package: %v, %v", versions, err)
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
			fmt.Fprint(w, `<a href="4.0.0-rc-3/">older prerelease</a><a href="4.0.0-rc-4/">prerelease</a>`)
		case "/maven-3/3.9.9/binaries/apache-maven-3.9.9-bin.tar.gz",
			"/maven-4/4.0.0-rc-3/binaries/apache-maven-4.0.0-rc-3-bin.tar.gz",
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
		{"4.0.0-rc-3", "4.0.0-rc-3", false},
		{"4.0.0-rc-4", "4.0.0-rc-4", false},
	} {
		res, err := (&MavenTool{}).Install(t.Context(), tc.spec, t.TempDir(), server.URL+"/maven-3", InstallOptions{IncludePrerelease: tc.prerelease})
		if err != nil || res.NormalizedVer != tc.want {
			t.Fatalf("spec=%q, prerelease=%v: %+v, %v", tc.spec, tc.prerelease, res, err)
		}
	}
}

func TestMavenDirectMirrorLayoutAndScopedMajorLookup(t *testing.T) {
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<a href="4.0.0/">release</a>`)
		case "/4.0.0/binaries/apache-maven-4.0.0-bin.tar.gz":
			_, _ = w.Write(definitionArchive(t, "maven.tar.gz", "maven/bin/mvn", []byte("fixture")))
		default:
			t.Errorf("unexpected direct Maven request: %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer direct.Close()
	res, err := (&MavenTool{}).Install(t.Context(), "", t.TempDir(), direct.URL, InstallOptions{})
	if err != nil || res.NormalizedVer != "4.0.0" {
		t.Fatalf("direct mirror install: %+v, %v", res, err)
	}

	scoped := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<a href="maven-3/">3</a><a href="maven-4/">4</a>`)
		case "/maven-3/":
			fmt.Fprint(w, `<a href="3.9.9/">release</a>`)
		case "/maven-3/3.9.9/binaries/apache-maven-3.9.9-bin.tar.gz":
			_, _ = w.Write(definitionArchive(t, "maven.tar.gz", "maven/bin/mvn", []byte("fixture")))
		default:
			t.Errorf("unrelated Maven index was fetched: %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer scoped.Close()
	res, err = (&MavenTool{}).Install(t.Context(), "3", t.TempDir(), scoped.URL, InstallOptions{})
	if err != nil || res.NormalizedVer != "3.9.9" {
		t.Fatalf("scoped major install: %+v, %v", res, err)
	}
}

func TestPythonReleasePagination(t *testing.T) {
	var firstPage strings.Builder
	firstPage.WriteByte('[')
	for i := 0; i < 100; i++ {
		if i > 0 {
			firstPage.WriteByte(',')
		}
		firstPage.WriteString(`{"tag_name":"20261101","assets":[{"name":"cpython-3.13.0-20261101-`)
		firstPage.WriteString(mapPythonTarget())
		firstPage.WriteString(`-install_only.tar.gz"}]}`)
	}
	firstPage.WriteByte(']')
	diagramHTTP(t, func(r *http.Request) (string, int) {
		if r.URL.Query().Get("page") == "2" {
			return `[{"tag_name":"20250101","assets":[{"name":"cpython-3.12.1-20250101-` + mapPythonTarget() + `-install_only.tar.gz"}]}]`, 200
		}
		return firstPage.String(), 200
	})
	version, tag, _, err := resolvePythonAsset(t.Context(), "3.12.1", false)
	if err != nil || version != "3.12.1" || tag != "20250101" {
		t.Fatalf("paginated Python resolution: %s, %s, %v", version, tag, err)
	}
	versions, err := listVersions(t.Context(), "python", "")
	if err != nil || !containsString(versions, "3.12.1") {
		t.Fatalf("paginated Python listing: %v, %v", versions, err)
	}
}

func TestStableSelectionFormats(t *testing.T) {
	for _, tc := range []struct {
		version string
		rc      bool
	}{
		{"3.0.0-1", true}, {"2.0.0-pre.1", true}, {"21.0.1-microsoft", false},
		{"21.0.3-zulu_prime", false},
		{"25.0.1-temurin", false}, {"25-ea", true}, {"25-ea-microsoft", true},
		{"v0.0.0-20240101000000-cdef12345678", false},
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

func TestSharedInstallerKeepsPreviousCandidateOnSetupFailure(t *testing.T) {
	for _, binaryInstall := range []bool{false, true} {
		t.Run(fmt.Sprintf("binary=%v", binaryInstall), func(t *testing.T) {
			home := t.TempDir()
			installDir := filepath.Join(home, "candidates", "dummy", "1.0.0")
			if err := os.MkdirAll(installDir, 0755); err != nil {
				t.Fatal(err)
			}
			oldPayload := filepath.Join(installDir, "bin", "tool")
			if binaryInstall {
				oldPayload = filepath.Join(installDir, "tool")
			}
			if err := os.MkdirAll(filepath.Dir(oldPayload), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(oldPayload, []byte("working"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(installDir, ".dogu-complete"), nil, 0644); err != nil {
				t.Fatal(err)
			}
			diagramHTTP(t, func(r *http.Request) (string, int) {
				if binaryInstall {
					return "replacement", 200
				}
				return string(definitionArchive(t, "tool.tar.gz", "bin/tool", []byte("replacement"))), 200
			})
			if binaryInstall {
				_, err := installBinaryTool(t.Context(), BinaryInstallConfig{
					ToolName: "dummy", VersionSpec: "1.0.0", NormalizedVer: "1.0.0",
					DownloadURL: "https://example.test/tool", CacheFileName: "tool", TargetBin: "tool",
					DoguHome: home, Opts: InstallOptions{Force: true},
					PostInstall: func(*InstallResult) error { return fmt.Errorf("setup failed") },
				})
				if err == nil {
					t.Fatal("expected setup failure")
				}
			} else {
				_, err := installArchiveTool(t.Context(), ArchiveInstallConfig{
					ToolName: "dummy", VersionSpec: "1.0.0", NormalizedVer: "1.0.0",
					DownloadURL: "https://example.test/tool.tar.gz", FileName: "tool.tar.gz",
					DoguHome: home, Opts: InstallOptions{Force: true},
					PostInstall: func(*InstallResult) error { return fmt.Errorf("setup failed") },
				})
				if err == nil {
					t.Fatal("expected setup failure")
				}
			}
			got, err := os.ReadFile(oldPayload)
			if err != nil || string(got) != "working" {
				t.Fatalf("previous candidate was not preserved: %q, %v", got, err)
			}
		})
	}
}

func TestSharedInstallerSerializesCandidatePublication(t *testing.T) {
	home := t.TempDir()
	payload := definitionArchive(t, "tool.tar.gz", "bin/tool", []byte("fixture"))
	var mu sync.Mutex
	downloads := 0
	diagramHTTP(t, func(r *http.Request) (string, int) {
		mu.Lock()
		downloads++
		mu.Unlock()
		time.Sleep(40 * time.Millisecond)
		return string(payload), 200
	})
	cfg := ArchiveInstallConfig{
		ToolName: "dummy", VersionSpec: "", NormalizedVer: "1.0.0",
		DownloadURL: "https://example.test/tool.tar.gz", FileName: "tool.tar.gz", DoguHome: home,
	}
	results := make([]*InstallResult, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = installArchiveTool(t.Context(), cfg)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("install %d: %v", i, err)
		}
	}
	if results[0].InstallDir != results[1].InstallDir || downloads != 1 {
		t.Fatalf("installs were not serialized: dirs=%s,%s downloads=%d", results[0].InstallDir, results[1].InstallDir, downloads)
	}
	root, err := os.Stat(results[0].InstallDir)
	if err != nil || root.Mode().Perm() != 0755 {
		t.Fatalf("candidate permissions: %v, %v", root, err)
	}
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
