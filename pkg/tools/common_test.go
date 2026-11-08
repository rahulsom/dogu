package tools

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveBaseURL(t *testing.T) {
	defaultURL := "https://example.com/dist"
	if got := resolveBaseURL(defaultURL, ""); got != defaultURL {
		t.Errorf("resolveBaseURL('', default) = %q, want %q", got, defaultURL)
	}

	mirror := "https://mirror.example.com/dist/"
	want := "https://mirror.example.com/dist"
	if got := resolveBaseURL(defaultURL, mirror); got != want {
		t.Errorf("resolveBaseURL(mirror, default) = %q, want %q", got, want)
	}
}

func TestBuildEnvVars(t *testing.T) {
	homeDir := "/test/home/tool"
	envVars := buildEnvVars("mytool", "1.2.3", homeDir, []string{"MYTOOL_HOME"}, map[string]string{"CUSTOM_VAR": "val"})

	if envVars["MYTOOL_HOME"] != homeDir {
		t.Errorf("expected MYTOOL_HOME = %q, got %q", homeDir, envVars["MYTOOL_HOME"])
	}
	if envVars["CUSTOM_VAR"] != "val" {
		t.Errorf("expected CUSTOM_VAR = 'val', got %q", envVars["CUSTOM_VAR"])
	}
	if envVars["MYTOOL_1_HOME"] != homeDir {
		t.Errorf("expected MYTOOL_1_HOME = %q, got %q", homeDir, envVars["MYTOOL_1_HOME"])
	}
}

func TestMapTargetTriple(t *testing.T) {
	arch, sys := mapTargetTriple("gnu")
	if arch == "" || sys == "" {
		t.Errorf("expected non-empty arch and sys, got arch=%q, sys=%q", arch, sys)
	}

	switch runtime.GOOS {
	case "darwin":
		if sys != "apple-darwin" {
			t.Errorf("darwin expected apple-darwin, got %q", sys)
		}
	case "windows":
		if sys != "pc-windows-msvc" {
			t.Errorf("windows expected pc-windows-msvc, got %q", sys)
		}
	default:
		if sys != "unknown-linux-gnu" {
			t.Errorf("linux expected unknown-linux-gnu, got %q", sys)
		}
	}
}

func TestCheckExistingCandidate(t *testing.T) {
	doguHome := t.TempDir()
	candidateDir := filepath.Join(doguHome, "candidates", "dummy", "1.0.0", "bin")
	if err := os.MkdirAll(candidateDir, 0755); err != nil {
		t.Fatalf("failed to create dummy candidate dir: %v", err)
	}

	dummyBin := filepath.Join(candidateDir, "dummy")
	if err := os.WriteFile(dummyBin, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to write dummy bin: %v", err)
	}

	// Hit existing candidate
	res, ok, err := checkExistingCandidate(doguHome, "dummy", "1.0", InstallOptions{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected existing candidate to be found")
	}
	if res.ToolName != "dummy" {
		t.Errorf("expected toolName = dummy, got %s", res.ToolName)
	}

	// Force mode ignores existing
	res, ok, err = checkExistingCandidate(doguHome, "dummy", "1.0", InstallOptions{Force: true}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok || res != nil {
		t.Errorf("expected ok=false with Force=true")
	}
}

func TestExistingCandidateExactAndPartial(t *testing.T) {
	home := t.TempDir()
	for _, version := range []string{"1", "1.2", "1.2.3", "1.10.0", "2.0.0-rc1", "2.0.0"} {
		if err := os.MkdirAll(filepath.Join(home, "candidates", "dummy", version, "bin"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct{ spec, want string }{
		{"1", "1.10.0"},
		{"1.2", "1.2.3"},
		{"1.2.3", "1.2.3"},
		{"2.0.0-rc1", "2.0.0-rc1"},
		{"", "2.0.0"},
	} {
		dir, ok := findExistingCandidate(home, "dummy", tc.spec, false)
		if !ok || filepath.Base(dir) != tc.want {
			t.Fatalf("spec=%q: %s, %v; want %s", tc.spec, dir, ok, tc.want)
		}
	}
}

func TestCachedAssetsAreVersionScoped(t *testing.T) {
	for _, binary := range []bool{false, true} {
		t.Run(fmt.Sprintf("binary=%v", binary), func(t *testing.T) {
			home := t.TempDir()
			filename := "tool.tar.gz"
			if binary {
				filename = "tool"
			}
			downloads := 0
			diagramHTTP(t, func(r *http.Request) (string, int) {
				downloads++
				version := filepath.Base(filepath.Dir(r.URL.Path))
				payload := []byte(version)
				if !binary {
					payload = definitionArchive(t, filename, "tool/bin/tool", payload)
				}
				return string(payload), 200
			})
			for _, version := range []string{"1.0.0", "2.0.0"} {
				url := "https://downloads.example.test/" + version + "/" + filename
				var res *InstallResult
				var err error
				if binary {
					res, err = installBinaryTool(t.Context(), BinaryInstallConfig{
						ToolName: "dummy", NormalizedVer: version, DoguHome: home,
						DownloadURL: url, CacheFileName: filename, TargetBin: "tool",
					})
				} else {
					res, err = installArchiveTool(t.Context(), ArchiveInstallConfig{
						ToolName: "dummy", NormalizedVer: version, DoguHome: home,
						DownloadURL: url, FileName: filename,
					})
				}
				if err != nil {
					t.Fatal(err)
				}
				got, err := os.ReadFile(filepath.Join(res.BinDirs[0], "tool"))
				if err != nil || string(got) != version {
					t.Fatalf("installed %s payload = %q, %v", version, got, err)
				}
			}
			if downloads != 2 {
				t.Fatalf("downloads=%d; distinct versions must not share cached payloads", downloads)
			}
		})
	}
}
