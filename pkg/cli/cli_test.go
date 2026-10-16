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

	// Test Java candidate reuse with nested macOS JDK layout
	java8Dir := filepath.Join(doguHome, "candidates", "java", "8", "amazon-corretto-8.jdk", "Contents", "Home", "bin")
	if err := os.MkdirAll(java8Dir, 0755); err != nil {
		t.Fatalf("failed to create java mac dir: %v", err)
	}
	mockJavaScript := []byte("#!/bin/sh\necho 'openjdk version 1.8.0_402'\n")
	if err := os.WriteFile(filepath.Join(java8Dir, "java"), mockJavaScript, 0755); err != nil {
		t.Fatalf("failed to write mock java: %v", err)
	}

	javaCmd := exec.Command(doguBin, "--java", "8", "--", "java", "-version")
	javaCmd.Env = append(os.Environ(), "DOGU_HOME="+doguHome)

	javaOut, err := javaCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dogu java failed to execute: %v, output: %s", err, string(javaOut))
	}

	if !strings.Contains(string(javaOut), "openjdk version 1.8.0_402") {
		t.Errorf("expected reused java output 'openjdk version 1.8.0_402', got: %s", string(javaOut))
	}
}

func TestParallelToolInstallation(t *testing.T) {
	tempDir := t.TempDir()
	doguBin := filepath.Join(tempDir, "dogu")

	buildCmd := exec.Command("go", "build", "-o", doguBin, "github.com/dogu-dev/dogu/cmd/dogu")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build dogu binary: %v", err)
	}

	doguHome := filepath.Join(tempDir, "dogu_home")

	// Set up pre-populated candidates for yarn and pnpm so no external network calls are needed
	pnpmDir := filepath.Join(doguHome, "candidates", "pnpm", "8.15.1")
	if err := os.MkdirAll(pnpmDir, 0755); err != nil {
		t.Fatalf("failed to create pnpm dir: %v", err)
	}
	mockPnpmScript := []byte("#!/bin/sh\necho '8.15.1'\n")
	if err := os.WriteFile(filepath.Join(pnpmDir, "pnpm"), mockPnpmScript, 0755); err != nil {
		t.Fatalf("failed to write mock pnpm: %v", err)
	}

	yarnDir := filepath.Join(doguHome, "candidates", "yarn", "1.22.19", "bin")
	if err := os.MkdirAll(yarnDir, 0755); err != nil {
		t.Fatalf("failed to create yarn dir: %v", err)
	}
	mockYarnScript := []byte("#!/bin/sh\necho '1.22.19'\n")
	if err := os.WriteFile(filepath.Join(yarnDir, "yarn"), mockYarnScript, 0755); err != nil {
		t.Fatalf("failed to write mock yarn: %v", err)
	}

	// Request both tools in parallel: --pnpm 8.15.1 --yarn 1.22.19 -- sh -c "pnpm; yarn"
	cmd := exec.Command(doguBin, "--pnpm", "8.15.1", "--yarn", "1.22.19", "--", "sh", "-c", "pnpm && yarn")
	cmd.Env = append(os.Environ(), "DOGU_HOME="+doguHome)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dogu failed to execute parallel tools: %v, output: %s", err, string(out))
	}

	output := string(out)
	if !strings.Contains(output, "8.15.1") || !strings.Contains(output, "1.22.19") {
		t.Errorf("expected output to contain both 8.15.1 and 1.22.19, got: %s", output)
	}
}

func TestVerboseMode(t *testing.T) {
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

	for _, verboseFlag := range []string{"-v", "--verbose"} {
		cmd := exec.Command(doguBin, verboseFlag, "--node", "26", "--", "node", "--version")
		cmd.Env = append(os.Environ(), "DOGU_HOME="+doguHome)

		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("dogu %s failed: %v, output: %s", verboseFlag, err, string(out))
		}

		output := string(out)
		if !strings.Contains(output, "DEBU") && !strings.Contains(output, "DEBUG") {
			t.Errorf("expected DEBUG logs in verbose output for %s, got: %s", verboseFlag, output)
		}
		if !strings.Contains(output, "v26.11.0") {
			t.Errorf("expected sub-command output v26.11.0, got: %s", output)
		}
	}
}
