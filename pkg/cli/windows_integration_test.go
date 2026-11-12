//go:build windows

package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWindowsIntegration(t *testing.T) {
	if os.Getenv("DOGU_INTEGRATION") != "1" {
		t.Skip("set DOGU_INTEGRATION=1 to download live Go and Maven releases")
	}

	work := t.TempDir()
	home := filepath.Join(work, "dogu-home")
	binary := filepath.Join(work, "dogu.exe")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", binary, "github.com/dogu-dev/dogu/cmd/dogu")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build dogu: %v\n%s", err, output)
	}
	t.Setenv("DOGU_HOME", home)
	t.Setenv("DOGU_CONFIGURATION", "")
	t.Setenv("GOTOOLCHAIN", "local")

	for _, tc := range []struct {
		tool, version string
		command       []string
		want          string
	}{
		{"go", "1.25.0", []string{"go", "version"}, "go version go1.25.0 windows/" + runtime.GOARCH},
		{"maven", "3.9.16", []string{"mvn", "-version"}, "Apache Maven 3.9.16"},
	} {
		t.Run(tc.tool, func(t *testing.T) {
			args := append([]string{"--" + tc.tool, tc.version, "--"}, tc.command...)
			cmd := exec.CommandContext(t.Context(), binary, args...)
			cmd.Dir = work
			output, err := cmd.CombinedOutput()
			if err != nil || !strings.Contains(string(output), tc.want) {
				t.Fatalf("dogu %v: expected %q, got %v\n%s", args, tc.want, err, output)
			}
			if tc.tool == "maven" {
				candidate := filepath.Join(home, "candidates", tc.tool, tc.version)
				if !strings.Contains(strings.ToLower(string(output)), strings.ToLower("Maven home: "+candidate)) {
					t.Fatalf("Maven did not launch from %s:\n%s", candidate, output)
				}
			}
		})
	}
}
