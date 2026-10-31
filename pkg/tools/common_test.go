package tools

import (
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
