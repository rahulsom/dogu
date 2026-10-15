package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolIntegrations(t *testing.T) {
	tempDir := t.TempDir()
	doguBin := filepath.Join(tempDir, "dogu")

	buildCmd := exec.Command("go", "build", "-o", doguBin, "github.com/dogu-dev/dogu/cmd/dogu")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build dogu binary: %v", err)
	}

	testCases := []struct {
		name           string
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
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			doguHome := t.TempDir()
			cmd := exec.Command(doguBin, tc.args...)
			cmd.Env = append(os.Environ(), "DOGU_HOME="+doguHome)

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
