package tools

import (
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
	}

	for _, tc := range tests {
		got := CompareVersions(tc.v1, tc.v2)
		if got != tc.expected {
			t.Errorf("CompareVersions(%q, %q) = %d; want %d", tc.v1, tc.v2, got, tc.expected)
		}
	}
}

func TestResolveUVVersion(t *testing.T) {
	normVer := resolveUVVersion(t.Context(), "0.1")
	if normVer == "0.1" {
		t.Errorf("expected uv 0.1 to resolve to full version, got %q", normVer)
	}
}

func TestResolvePythonVersion(t *testing.T) {
	normVer := resolvePythonVersion(t.Context(), "3.12")
	if normVer != "3.12.1" {
		t.Errorf("expected python 3.12 to resolve to 3.12.1, got %q", normVer)
	}

	normVer3 := resolvePythonVersion(t.Context(), "3")
	if normVer3 == "3" {
		t.Errorf("expected python 3 to resolve to full version, got %q", normVer3)
	}
}

func TestResolveNPMToolVersion(t *testing.T) {
	for _, tool := range []string{"npm", "pnpm", "yarn"} {
		normVer := resolveNPMToolVersion(t.Context(), tool, "1", "")
		if normVer == "1" {
			t.Errorf("expected %s normVer to be resolved to full version, got %q", tool, normVer)
		}
	}
}

func TestResolveMavenVersion(t *testing.T) {
	normVer := resolveMavenVersion(t.Context(), "3", "https://archive.apache.org/dist/maven/maven-3")
	if normVer == "3" {
		t.Errorf("expected normVer to be resolved to full maven version, got %q", normVer)
	}
}

func TestResolveBunVersion(t *testing.T) {
	tag, normVer := resolveBunVersion(t.Context(), "1")
	if tag == "bun-v1" {
		t.Errorf("expected tag to be resolved to full version tag, got %q", tag)
	}
	if normVer == "v1" {
		t.Errorf("expected normVer to be resolved to full version, got %q", normVer)
	}
}

func TestSelectBestVersion(t *testing.T) {
	versions := []string{"1.0.0", "1.1.0", "1.1.42", "1.2.0", "2.0.0"}

	got, err := SelectBestVersion(versions, "1.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.1.42" {
		t.Errorf("SelectBestVersion(..., '1.1') = %q; want %q", got, "1.1.42")
	}

	got, err = SelectBestVersion(versions, "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.2.0" {
		t.Errorf("SelectBestVersion(..., '1') = %q; want %q", got, "1.2.0")
	}

	_, err = SelectBestVersion(versions, "3")
	if err == nil {
		t.Errorf("expected error for non-matching spec '3', got nil")
	}
}
