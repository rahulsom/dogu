package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIHelpAndToolsList(t *testing.T) {
	tempDir := t.TempDir()
	doguBin := filepath.Join(tempDir, "dogu")

	buildCmd := exec.Command("go", "build", "-o", doguBin, "github.com/dogu-dev/dogu/cmd/dogu")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build dogu binary: %v", err)
	}

	cmd := exec.Command(doguBin, "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dogu --help failed: %v", err)
	}

	output := string(out)
	if !strings.Contains(output, "Usage: dogu") {
		t.Errorf("expected Usage in --help output, got: %s", output)
	}
	if !strings.Contains(output, "Supported tools:") {
		t.Errorf("expected Supported tools list in --help output, got: %s", output)
	}
	for _, tool := range []string{"java", "node", "maven", "npm", "yarn", "pnpm", "bun", "python", "uv"} {
		if !strings.Contains(output, "- "+tool) {
			t.Errorf("expected tool %s in supported tools list, got: %s", tool, output)
		}
	}
}

func TestCandidateReuseAndForce(t *testing.T) {
	tempDir := t.TempDir()
	doguBin := filepath.Join(tempDir, "dogu")

	buildCmd := exec.Command("go", "build", "-o", doguBin, "github.com/dogu-dev/dogu/cmd/dogu")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build dogu binary: %v", err)
	}

	doguHome := filepath.Join(tempDir, "dogu_home")
	node2611Dir := filepath.Join(doguHome, "candidates", "node", "26.11.0", "bin")
	if err := os.MkdirAll(node2611Dir, 0755); err != nil {
		t.Fatalf("failed to create node dir: %v", err)
	}

	mockNodeScript := []byte("#!/bin/sh\necho 'v26.11.0'\n")
	if err := os.WriteFile(filepath.Join(node2611Dir, "node"), mockNodeScript, 0755); err != nil {
		t.Fatalf("failed to write mock node: %v", err)
	}

	// Request major version --node 26 without force -> should reuse 26.11.0
	cmd := exec.Command(doguBin, "--node", "26", "--", "node", "--version")
	cmd.Env = append(os.Environ(), "DOGU_HOME="+doguHome)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dogu failed to execute: %v, output: %s", err, string(out))
	}

	if !strings.Contains(string(out), "v26.11.0") {
		t.Errorf("expected reused v26.11.0 output, got: %s", string(out))
	}
}
