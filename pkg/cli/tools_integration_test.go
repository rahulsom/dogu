package cli_test

import (
	"encoding/json"
	"os"

	"github.com/dogu-dev/dogu/pkg/config"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolIntegrations(t *testing.T) {
	if os.Getenv("DOGU_INTEGRATION") != "1" {
		t.Skip("set DOGU_INTEGRATION=1 to run live download integration tests")
	}
	cachewURL := getOrStartCachew(t, findRepoRoot(t))
	tempDir := t.TempDir()
	doguBin := filepath.Join(tempDir, "dogu")

	buildCmd := exec.Command("go", "build", "-o", doguBin, "github.com/dogu-dev/dogu/testdata/cachew-dogu")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build dogu binary: %v", err)
	}

	testCases := []struct {
		name           string
		setupFunc      func(doguHome string)
		args           []string
		expectedOutput string
	}{
		{
			name:           "java",
			args:           []string{"--java", "17", "--", "java", "-version"},
			expectedOutput: "17",
		},
		{
			name:           "node",
			args:           []string{"--node", "20", "--", "node", "--version"},
			expectedOutput: "v20",
		},
		{
			name:           "maven",
			args:           []string{"--maven", "3.9.6", "--", "mvn", "-version"},
			expectedOutput: "3.9.6",
		},
		{
			name:           "npm",
			args:           []string{"--node", "20", "--npm", "10.2.5", "--", "npm", "-v"},
			expectedOutput: "10.2.5",
		},
		{
			name:           "yarn",
			args:           []string{"--yarn", "1.22.19", "--", "yarn", "-v"},
			expectedOutput: "1.22.19",
		},
		{
			name:           "pnpm",
			args:           []string{"--pnpm", "8.15.1", "--", "pnpm", "-v"},
			expectedOutput: "8.15.1",
		},
		{
			name:           "bun",
			args:           []string{"--bun", "1.1.0", "--", "bun", "--version"},
			expectedOutput: "1.1.0",
		},
		{
			name:           "python",
			args:           []string{"--python", "3.12.1", "--", "python", "--version"},
			expectedOutput: "3.12.1",
		},
		{
			name:           "uv",
			args:           []string{"--uv", "0.1.15", "--", "uv", "--version"},
			expectedOutput: "0.1.15",
		},
		{
			name:           "bun-partial",
			args:           []string{"--bun", "1", "--", "bun", "--version"},
			expectedOutput: "1.",
		},
		{
			name:           "maven-partial",
			args:           []string{"--maven", "3", "--", "mvn", "-version"},
			expectedOutput: "3.",
		},
		{
			name:           "npm-partial",
			args:           []string{"--node", "20", "--npm", "10", "--", "npm", "-v"},
			expectedOutput: "10.",
		},
		{
			name:           "yarn-partial",
			args:           []string{"--yarn", "1", "--", "yarn", "-v"},
			expectedOutput: "1.",
		},
		{
			name:           "pnpm-partial",
			args:           []string{"--pnpm", "8", "--", "pnpm", "-v"},
			expectedOutput: "8.",
		},
		{
			name:           "python-partial",
			args:           []string{"--python", "3", "--", "python", "--version"},
			expectedOutput: "3.",
		},
		{
			name:           "uv-partial",
			args:           []string{"--uv", "0.1", "--", "uv", "--version"},
			expectedOutput: "0.1.",
		},
		{
			name:           "sbt-partial",
			args:           []string{"--sbt", "1", "--", "sbt", "--version"},
			expectedOutput: "1.",
		},
		{
			name:           "gradle-partial",
			args:           []string{"--gradle", "8", "--", "gradle", "--version"},
			expectedOutput: "8.",
		},
		{
			name: "mill-partial",
			setupFunc: func(doguHome string) {
				mDir := filepath.Join(doguHome, "candidates", "mill", "0.12.5")
				_ = os.MkdirAll(mDir, 0755)
				_ = os.WriteFile(filepath.Join(mDir, "mill"), []byte("#!/bin/sh\necho '0.12.5'\n"), 0755)
			},
			args:           []string{"--mill", "0.12", "--", "mill", "--version"},
			expectedOutput: "0.12.5",
		},
		{
			name:           "bazel-partial",
			args:           []string{"--bazel", "7", "--", "bazel", "--version"},
			expectedOutput: "7.",
		},
		{
			name:           "pants-partial",
			args:           []string{"--pants", "2.33", "--", "pants", "--version"},
			expectedOutput: "2.33.",
		},
		{
			name:           "ant-partial",
			args:           []string{"--ant", "1.10", "--", "ant", "-version"},
			expectedOutput: "1.10.",
		},
		{
			name:           "buck2-partial",
			args:           []string{"--buck2", "2026-10", "--", "buck2", "--version"},
			expectedOutput: "buck2",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			doguHome := t.TempDir()
			cfg, err := config.DefaultConfig()
			if err != nil {
				t.Fatal(err)
			}
			for tool, upstream := range cfg.BaseURLs {
				cfg.BaseURLs[tool] = cachewURL + "/" + strings.TrimPrefix(upstream, "https://")
			}
			data, err := json.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(doguHome, "config.json"), data, 0644); err != nil {
				t.Fatal(err)
			}

			_ = os.WriteFile(filepath.Join(doguHome, "BUILDROOT"), []byte(""), 0644)
			if tc.setupFunc != nil {
				tc.setupFunc(doguHome)
			}
			cmd := exec.Command(doguBin, tc.args...)
			cmd.Dir = doguHome
			cmd.Env = append(os.Environ(), "DOGU_HOME="+doguHome, "CACHEW_URL="+cachewURL, "DOGU_CONFIGURATION=")

			out, err := cmd.CombinedOutput()
			output := string(out)

			if err != nil {
				t.Fatalf("dogu %s failed: %v, output: %s", strings.Join(tc.args, " "), err, output)
			}

			if !strings.Contains(output, tc.expectedOutput) {
				t.Errorf("expected output to contain %q, got: %s", tc.expectedOutput, output)
			}
		})
	}
}
