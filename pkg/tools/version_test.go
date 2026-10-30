package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"v1.2.3", "1.2.3"},
		{"bun-v1.1.0", "1.1.0"},
		{"apache-maven-3.9.6", "3.9.6"},
		{"apache-ant-1.10.14", "1.10.14"},
		{"gradle-8.10.2", "8.10.2"},
		{"release_2.33.1", "2.33.1"},
		{"cpython-3.12.1", "3.12.1"},
		{"yarn-v1.22.19", "1.22.19"},
		{"17.0.2+7", "17.0.2"},
		{"21", "21"},
	}

	for _, tc := range tests {
		got := CleanVersion(tc.input)
		if got != tc.expected {
			t.Errorf("CleanVersion(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestMatchVersion(t *testing.T) {
	tests := []struct {
		candidate string
		spec      string
		expected  bool
	}{
		{"1.1.0", "1", true},
		{"bun-v1.1.0", "1", true},
		{"bun-v1.1.0", "1.1", true},
		{"bun-v1.1.0", "1.1.0", true},
		{"bun-v1.1.0", "1.2", false},
		{"3.9.6", "3", true},
		{"3.9.6", "3.9", true},
		{"3.9.6", "3.9.6", true},
		{"3.90.0", "3.9", false},
		{"10.2.5", "10", true},
		{"10.2.5", "10.2", true},
		{"10.2.5", "2", false},
		{"17.0.2", "17-tem", true},
		{"17.0.2-temurin", "17", true},
	}

	for _, tc := range tests {
		got := MatchVersion(tc.candidate, tc.spec)
		if got != tc.expected {
			t.Errorf("MatchVersion(%q, %q) = %v; want %v", tc.candidate, tc.spec, got, tc.expected)
		}
	}
}

func TestIsRC(t *testing.T) {
	tests := []struct {
		version  string
		expected bool
	}{
		{"7.7.1rc2", true},
		{"7.7.0", false},
		{"8.11-rc-1", true},
		{"8.10.2", false},
		{"4.0.0-rc-1", true},
		{"4.0.0-alpha-1", true},
		{"3.9.6", false},
		{"1.10.0-RC1", true},
		{"1.10.10", false},
		{"2.23.0rc0", true},
		{"2.23.0a0", true},
		{"2.33.1", false},
		{"3.13.0a1", true},
		{"3.11.7", false},
		{"v23.0.0-rc.1", true},
		{"v22.11.0", false},
		{"21-ea", true},
		{"21", false},
		{"17.0.2+7", false},
	}

	for _, tc := range tests {
		got := IsRC(tc.version)
		if got != tc.expected {
			t.Errorf("IsRC(%q) = %v; want %v", tc.version, got, tc.expected)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int // -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
	}{
		{"1.1.0", "1.2.0", -1},
		{"1.10.0", "1.9.0", 1},
		{"3.9.16", "3.9.2", 1},
		{"v20.18.0", "20.18.0", 0},
		{"1.22.19", "1.22.22", -1},
		{"0.1.15", "0.2.0", -1},
		{"7.7.0", "7.7.1rc2", -1},
		{"7.7.1", "7.7.1rc2", 1},
		{"7.7.1rc1", "7.7.1rc2", -1},
	}

	for _, tc := range tests {
		got := CompareVersions(tc.v1, tc.v2)
		if got != tc.expected {
			t.Errorf("CompareVersions(%q, %q) = %d; want %d", tc.v1, tc.v2, got, tc.expected)
		}
	}
}

func TestResolveUVVersion(t *testing.T) {
	normVer := resolveUVVersion(t.Context(), "0.1", false)
	if normVer == "0.1" {
		t.Errorf("expected uv 0.1 to resolve to full version, got %q", normVer)
	}
}

func TestResolvePythonVersion(t *testing.T) {
	normVer := resolvePythonVersion(t.Context(), "3.12", false)
	if normVer != "3.12.1" {
		t.Errorf("expected python 3.12 to resolve to 3.12.1, got %q", normVer)
	}

	normVer3 := resolvePythonVersion(t.Context(), "3", false)
	if normVer3 == "3" {
		t.Errorf("expected python 3 to resolve to full version, got %q", normVer3)
	}
}

func TestResolveNPMToolVersion(t *testing.T) {
	for _, tool := range []string{"npm", "pnpm", "yarn"} {
		normVer := resolveNPMToolVersion(t.Context(), tool, "1", "", false)
		if normVer == "1" {
			t.Errorf("expected %s normVer to be resolved to full version, got %q", tool, normVer)
		}
	}
}

func TestResolveMavenVersion(t *testing.T) {
	normVer := resolveMavenVersion(t.Context(), "3", "https://archive.apache.org/dist/maven/maven-3", false)
	if normVer == "3" {
		t.Errorf("expected normVer to be resolved to full maven version, got %q", normVer)
	}
}

func TestFetchGradleVersions(t *testing.T) {
	versions, err := FetchGradleVersions(t.Context(), "")
	if err != nil {
		t.Fatalf("FetchGradleVersions failed: %v", err)
	}
	if len(versions) == 0 {
		t.Errorf("expected gradle versions to be non-empty")
	}
}

func TestFetchApacheAntVersions(t *testing.T) {
	versions, err := FetchApacheAntVersions(t.Context(), "")
	if err != nil {
		t.Fatalf("FetchApacheAntVersions failed: %v", err)
	}
	if len(versions) == 0 {
		t.Errorf("expected apache ant versions to be non-empty")
	}
}

func TestResolveBunVersion(t *testing.T) {
	tag, normVer := resolveBunVersion(t.Context(), "1", false)
	if tag == "bun-v1" {
		t.Errorf("expected tag to be resolved to full version tag, got %q", tag)
	}
	if normVer == "v1" {
		t.Errorf("expected normVer to be resolved to full version, got %q", normVer)
	}
}

func TestSelectBestVersion(t *testing.T) {
	versions := []string{"1.0.0", "1.1.0", "1.1.42", "1.2.0", "2.0.0"}

	got, err := SelectBestVersion(versions, "1.1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.1.42" {
		t.Errorf("SelectBestVersion(..., '1.1') = %q; want %q", got, "1.1.42")
	}

	got, err = SelectBestVersion(versions, "1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.2.0" {
		t.Errorf("SelectBestVersion(..., '1') = %q; want %q", got, "1.2.0")
	}

	_, err = SelectBestVersion(versions, "3", false)
	if err == nil {
		t.Errorf("expected error for non-matching spec '3', got nil")
	}
}

func TestSelectBestVersionWithPrereleases(t *testing.T) {
	bazelVersions := []string{"7.7.0", "7.7.1rc2"}

	// Default: includePrerelease = false -> should avoid 7.7.1rc2 and select 7.7.0
	got, err := SelectBestVersion(bazelVersions, "7", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "7.7.0" {
		t.Errorf("SelectBestVersion(bazelVersions, '7', false) = %q; want %q", got, "7.7.0")
	}

	// Option enabled: includePrerelease = true -> should include 7.7.1rc2 and select 7.7.1rc2
	got, err = SelectBestVersion(bazelVersions, "7", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "7.7.1rc2" {
		t.Errorf("SelectBestVersion(bazelVersions, '7', true) = %q; want %q", got, "7.7.1rc2")
	}

	// Explicit RC spec: even with includePrerelease = false, explicit spec should select 7.7.1rc2
	got, err = SelectBestVersion(bazelVersions, "7.7.1rc2", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "7.7.1rc2" {
		t.Errorf("SelectBestVersion(bazelVersions, '7.7.1rc2', false) = %q; want %q", got, "7.7.1rc2")
	}
}

func TestInstallCandidateDirUsesFullyResolvedVersion(t *testing.T) {
	// Test that installing a tool with a partial version spec creates candidate directory with fully resolved version
	doguHome := t.TempDir()

	tool, ok := Get("ant")
	if !ok {
		t.Fatalf("ant tool not registered")
	}

	result, err := tool.Install(t.Context(), "1.10", doguHome, "", InstallOptions{})
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	if result.NormalizedVer == "1.10" {
		t.Fatalf("expected NormalizedVer to be resolved to full version, got %q", result.NormalizedVer)
	}

	expectedInstallDir := filepath.Join(doguHome, "candidates", "ant", result.NormalizedVer)
	if result.InstallDir != expectedInstallDir {
		t.Errorf("InstallDir = %q; want %q", result.InstallDir, expectedInstallDir)
	}

	if filepath.Base(result.InstallDir) == "1.10" {
		t.Errorf("InstallDir base should be fully resolved version (%q), but got partial spec '1.10'", result.NormalizedVer)
	}
}

func TestJavaCandidateDirIncludesVendorSuffix(t *testing.T) {
	j := &JavaTool{}
	url, fn, normVer, resolvedDist, err := j.resolveDownloadURL(t.Context(), "17", "temurin", "macos", "x64", "", false)
	if err != nil {
		t.Fatalf("resolveDownloadURL failed: %v", err)
	}
	if url == "" || fn == "" {
		t.Errorf("expected non-empty url and filename")
	}
	if normVer == "17" {
		t.Errorf("expected normVer to be resolved to full version, got %q", normVer)
	}
	if resolvedDist != "temurin" {
		t.Errorf("expected resolvedDist to be temurin, got %q", resolvedDist)
	}
}

func TestFindJDKHome(t *testing.T) {
	// 1. Standard layout: baseDir/bin
	tmpDir1 := t.TempDir()
	binDir1 := filepath.Join(tmpDir1, "bin")
	if err := os.MkdirAll(binDir1, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}
	if got := findJDKHome(tmpDir1); got != tmpDir1 {
		t.Errorf("findJDKHome(standard) = %q; want %q", got, tmpDir1)
	}

	// 2. macOS top-level layout: baseDir/Contents/Home/bin
	tmpDir2 := t.TempDir()
	binDir2 := filepath.Join(tmpDir2, "Contents", "Home", "bin")
	if err := os.MkdirAll(binDir2, 0755); err != nil {
		t.Fatalf("failed to create mac home bin dir: %v", err)
	}
	wantMacHome2 := filepath.Join(tmpDir2, "Contents", "Home")
	if got := findJDKHome(tmpDir2); got != wantMacHome2 {
		t.Errorf("findJDKHome(mac top-level) = %q; want %q", got, wantMacHome2)
	}

	// 3. macOS nested layout: baseDir/amazon-corretto-8.jdk/Contents/Home/bin
	tmpDir3 := t.TempDir()
	binDir3 := filepath.Join(tmpDir3, "amazon-corretto-8.jdk", "Contents", "Home", "bin")
	if err := os.MkdirAll(binDir3, 0755); err != nil {
		t.Fatalf("failed to create mac nested jdk bin dir: %v", err)
	}
	wantMacHome3 := filepath.Join(tmpDir3, "amazon-corretto-8.jdk", "Contents", "Home")
	if got := findJDKHome(tmpDir3); got != wantMacHome3 {
		t.Errorf("findJDKHome(mac nested) = %q; want %q", got, wantMacHome3)
	}

	// 4. Nested standard layout: baseDir/jdk-17.0.1/bin
	tmpDir4 := t.TempDir()
	binDir4 := filepath.Join(tmpDir4, "jdk-17.0.1", "bin")
	if err := os.MkdirAll(binDir4, 0755); err != nil {
		t.Fatalf("failed to create nested standard bin dir: %v", err)
	}
	wantSub4 := filepath.Join(tmpDir4, "jdk-17.0.1")
	if got := findJDKHome(tmpDir4); got != wantSub4 {
		t.Errorf("findJDKHome(nested standard) = %q; want %q", got, wantSub4)
	}
}
