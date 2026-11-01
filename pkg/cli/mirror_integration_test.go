package cli_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("failed to locate repo root from %s", dir)
	return ""
}

func getOrStartCachew(t *testing.T, repoRoot string) string {
	t.Helper()

	if url := os.Getenv("CACHEW_URL"); url != "" {
		return strings.TrimSuffix(url, "/")
	}

	// Verify docker command and daemon availability
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		t.Skip("Docker daemon is not running; skipping mirror integration tests")
		return ""
	}

	configPath := filepath.Join(repoRoot, "testdata", "cachew.hcl")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("cachew config not found at %s: %v", configPath, err)
	}

	// Start cachew container with an ephemeral host port
	runCmd := exec.Command("docker", "run", "-d", "--rm",
		"-p", "0:8080",
		"-v", fmt.Sprintf("%s:/app/cachew.hcl:ro", configPath),
		"ghcr.io/block/cachew:latest",
		"--config", "/app/cachew.hcl",
	)
	var stderr bytes.Buffer
	runCmd.Stderr = &stderr
	out, err := runCmd.Output()
	if err != nil {
		t.Fatalf("failed to start cachew container: %v, stdout: %s, stderr: %s", err, string(out), stderr.String())
	}
	containerID := strings.TrimSpace(string(out))

	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", containerID).Run()
	})

	// Query mapped port
	portCmd := exec.Command("docker", "port", containerID, "8080/tcp")
	portOut, err := portCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get container port: %v, output: %s", err, string(portOut))
	}

	lines := strings.Split(strings.TrimSpace(string(portOut)), "\n")
	if len(lines) == 0 {
		t.Fatalf("unexpected docker port output: %s", string(portOut))
	}
	parts := strings.Split(lines[0], ":")
	port := parts[len(parts)-1]

	baseURL := fmt.Sprintf("http://127.0.0.1:%s", port)

	// Wait for Cachew liveness probe
	deadline := time.Now().Add(15 * time.Second)
	client := &http.Client{Timeout: 1 * time.Second}
	var ready bool
	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/_liveness")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !ready {
		logs, _ := exec.Command("docker", "logs", containerID).CombinedOutput()
		t.Fatalf("cachew container failed to become ready at %s within 15s. Logs:\n%s", baseURL, string(logs))
	}

	return baseURL
}

func TestGetOrStartCachew(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mock Docker command requires a Unix shell")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_liveness" {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	for _, tc := range []struct {
		name   string
		stderr string
	}{
		{name: "cached image"},
		{name: "uncached image", stderr: "Unable to find image locally\nPulling from block/cachew\nDownloaded newer image\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			binDir := t.TempDir()
			script := `#!/bin/sh
case "$1" in
	info) exit 0 ;;
	run)
		printf '%s' "$DOCKER_TEST_STDERR" >&2
		printf 'cachew-container-id\n'
		;;
	port)
		[ "$2" = "cachew-container-id" ] || exit 1
		printf '%s\n' "$DOCKER_TEST_ADDRESS"
		;;
	rm)
		[ "$3" = "cachew-container-id" ] || exit 1
		;;
	*) exit 1 ;;
esac
`
			if err := os.WriteFile(filepath.Join(binDir, "docker"), []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("CACHEW_URL", "")
			t.Setenv("DOCKER_TEST_STDERR", tc.stderr)
			t.Setenv("DOCKER_TEST_ADDRESS", server.Listener.Addr().String())

			if got := getOrStartCachew(t, findRepoRoot(t)); got != server.URL {
				t.Fatalf("getOrStartCachew() = %q, want %q", got, server.URL)
			}
		})
	}
}

func TestMirrorIntegration(t *testing.T) {
	repoRoot := findRepoRoot(t)
	cachewURL := getOrStartCachew(t, repoRoot)

	tempDir := t.TempDir()
	doguBin := filepath.Join(tempDir, "dogu")

	buildCmd := exec.Command("go", "build", "-o", doguBin, "github.com/dogu-dev/dogu/cmd/dogu")
	buildCmd.Dir = repoRoot
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build dogu binary: %v", err)
	}

	testCases := []struct {
		name           string
		configContent  string
		setupFunc      func(workDir string)
		args           []string
		expectedOutput string
		expectedMirror string
	}{
		{
			name: "java-via-cachew-mirror",
			configContent: fmt.Sprintf(
				`{"baseUrls": {"java": "%s/api.foojay.io/disco/v3.0/packages"}}`,
				cachewURL,
			),
			args:           []string{"-v", "--force", "--java", "17", "--", "java", "-version"},
			expectedOutput: "17",
			expectedMirror: fmt.Sprintf("%s/api.foojay.io/disco/v3.0/packages", cachewURL),
		},
		{
			name: "maven-via-cachew-mirror",
			configContent: fmt.Sprintf(
				`{"baseUrls": {"maven": "%s/archive.apache.org/dist/maven/maven-3"}}`,
				cachewURL,
			),
			args:           []string{"-v", "--force", "--maven", "3.9.6", "--", "mvn", "-version"},
			expectedOutput: "3.9.6",
			expectedMirror: fmt.Sprintf("%s/archive.apache.org/dist/maven/maven-3", cachewURL),
		},
		{
			name: "sbt-via-cachew-mirror",
			configContent: fmt.Sprintf(
				`{"baseUrls": {"sbt": "%s/github.com/sbt/sbt/releases/download"}}`,
				cachewURL,
			),
			args:           []string{"-v", "--force", "--sbt", "1.10.10", "--", "sbt", "--version"},
			expectedOutput: "1.10.10",
			expectedMirror: fmt.Sprintf("%s/github.com/sbt/sbt/releases/download", cachewURL),
		},
		{
			name: "gradle-via-cachew-mirror",
			configContent: fmt.Sprintf(
				`{"baseUrls": {"gradle": "%s/services.gradle.org/distributions"}}`,
				cachewURL,
			),
			args:           []string{"-v", "--force", "--gradle", "8.10.2", "--", "gradle", "--version"},
			expectedOutput: "8.10.2",
			expectedMirror: fmt.Sprintf("%s/services.gradle.org/distributions", cachewURL),
		},
		{
			name: "ant-via-cachew-mirror",
			configContent: fmt.Sprintf(
				`{"baseUrls": {"ant": "%s/archive.apache.org/dist/ant/binaries"}}`,
				cachewURL,
			),
			args:           []string{"-v", "--force", "--ant", "1.10.14", "--", "ant", "-version"},
			expectedOutput: "1.10.14",
			expectedMirror: fmt.Sprintf("%s/archive.apache.org/dist/ant/binaries", cachewURL),
		},
		{
			name: "mill-via-cachew-mirror",
			configContent: fmt.Sprintf(
				`{"baseUrls": {"mill": "%s/github.com/com-lihaoyi/mill/releases/download"}}`,
				cachewURL,
			),
			setupFunc: func(workDir string) {
				millCacheDir := filepath.Join(workDir, ".cache", "mill", "download")
				_ = os.MkdirAll(millCacheDir, 0755)
				jarPath := filepath.Join(millCacheDir, "0.12.5")
				_ = os.WriteFile(jarPath, []byte("#!/bin/sh\necho \"0.12.5\"\n"), 0755)
			},
			args:           []string{"-v", "--force", "--mill", "0.12.5", "--", "mill", "--version"},
			expectedOutput: "0.12.5",
			expectedMirror: fmt.Sprintf("%s/github.com/com-lihaoyi/mill/releases/download", cachewURL),
		},
		{
			name: "bazel-via-cachew-mirror",
			configContent: fmt.Sprintf(
				`{"baseUrls": {"bazel": "%s/github.com/bazelbuild/bazel/releases/download"}}`,
				cachewURL,
			),
			args:           []string{"-v", "--force", "--bazel", "7.4.0", "--", "bazel", "--version"},
			expectedOutput: "7.4.0",
			expectedMirror: fmt.Sprintf("%s/github.com/bazelbuild/bazel/releases/download", cachewURL),
		},
		{
			name: "pants-via-cachew-mirror",
			configContent: fmt.Sprintf(
				`{"baseUrls": {"pants": "%s/github.com/pantsbuild/pants/releases/download"}}`,
				cachewURL,
			),
			args:           []string{"-v", "--force", "--pants", "2.33.1", "--", "pants", "--version"},
			expectedOutput: "2.33.1",
			expectedMirror: fmt.Sprintf("%s/github.com/pantsbuild/pants/releases/download", cachewURL),
		},
		{
			name: "buck2-via-cachew-mirror",
			configContent: fmt.Sprintf(
				`{"baseUrls": {"buck2": "%s/github.com/facebook/buck2/releases/download"}}`,
				cachewURL,
			),
			args:           []string{"-v", "--force", "--buck2", "2026-10-15", "--", "buck2", "--version"},
			expectedOutput: "buck2",
			expectedMirror: fmt.Sprintf("%s/github.com/facebook/buck2/releases/download", cachewURL),
		},
		{
			name: "node-via-cachew-mirror",
			configContent: fmt.Sprintf(
				`{"baseUrls": {"node": "%s/nodejs.org/dist"}}`,
				cachewURL,
			),
			args:           []string{"-v", "--force", "--node", "20.18.0", "--", "node", "--version"},
			expectedOutput: "v20.18.0",
			expectedMirror: fmt.Sprintf("%s/nodejs.org/dist", cachewURL),
		},
		{
			name: "npm-via-cachew-mirror",
			configContent: fmt.Sprintf(
				`{"baseUrls": {"node": "%s/nodejs.org/dist", "npm": "%s/registry.npmjs.org/npm/-"}}`,
				cachewURL,
				cachewURL,
			),
			args:           []string{"-v", "--force", "--node", "20.18.0", "--npm", "10.2.5", "--", "npm", "-v"},
			expectedOutput: "10.2.5",
			expectedMirror: fmt.Sprintf("%s/registry.npmjs.org/npm/-", cachewURL),
		},
		{
			name: "pnpm-via-cachew-mirror",
			configContent: fmt.Sprintf(
				`{"baseUrls": {"pnpm": "%s/github.com/pnpm/pnpm/releases/download"}}`,
				cachewURL,
			),
			args:           []string{"-v", "--force", "--pnpm", "8.15.1", "--", "pnpm", "-v"},
			expectedOutput: "8.15.1",
			expectedMirror: fmt.Sprintf("%s/github.com/pnpm/pnpm/releases/download", cachewURL),
		},
		{
			name: "yarn-via-cachew-mirror",
			configContent: fmt.Sprintf(
				`{"baseUrls": {"yarn": "%s/github.com/yarnpkg/yarn/releases/download"}}`,
				cachewURL,
			),
			args:           []string{"-v", "--force", "--yarn", "1.22.19", "--", "yarn", "-v"},
			expectedOutput: "1.22.19",
			expectedMirror: fmt.Sprintf("%s/github.com/yarnpkg/yarn/releases/download", cachewURL),
		},
		{
			name: "bun-via-cachew-mirror",
			configContent: fmt.Sprintf(
				`{"baseUrls": {"bun": "%s/github.com/oven-sh/bun/releases/download"}}`,
				cachewURL,
			),
			args:           []string{"-v", "--force", "--bun", "1.1.0", "--", "bun", "--version"},
			expectedOutput: "1.1.0",
			expectedMirror: fmt.Sprintf("%s/github.com/oven-sh/bun/releases/download", cachewURL),
		},
		{
			name: "python-via-cachew-mirror",
			configContent: fmt.Sprintf(
				`{"baseUrls": {"python": "%s/github.com/indygreg/python-build-standalone/releases/download"}}`,
				cachewURL,
			),
			args:           []string{"-v", "--force", "--python", "3.12.1", "--", "python", "--version"},
			expectedOutput: "3.12.1",
			expectedMirror: fmt.Sprintf("%s/github.com/indygreg/python-build-standalone/releases/download", cachewURL),
		},
		{
			name: "uv-via-cachew-mirror",
			configContent: fmt.Sprintf(
				`{"baseUrls": {"uv": "%s/github.com/astral-sh/uv/releases/download"}}`,
				cachewURL,
			),
			args:           []string{"-v", "--force", "--uv", "0.1.15", "--", "uv", "--version"},
			expectedOutput: "0.1.15",
			expectedMirror: fmt.Sprintf("%s/github.com/astral-sh/uv/releases/download", cachewURL),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			workDir := t.TempDir()
			doguHome := filepath.Join(workDir, "dogu_home")

			_ = os.WriteFile(filepath.Join(workDir, "BUILDROOT"), []byte(""), 0644)
			if tc.setupFunc != nil {
				tc.setupFunc(workDir)
			}

			if err := os.MkdirAll(doguHome, 0755); err != nil {
				t.Fatalf("failed to create doguHome: %v", err)
			}
			configFile := filepath.Join(doguHome, "config.json")
			if err := os.WriteFile(configFile, []byte(tc.configContent), 0644); err != nil {
				t.Fatalf("failed to write config.json: %v", err)
			}

			cmd := exec.Command(doguBin, tc.args...)
			cmd.Dir = workDir
			cmd.Env = append(os.Environ(), "DOGU_HOME="+doguHome, "HOME="+workDir)

			out, err := cmd.CombinedOutput()
			output := string(out)

			if err != nil {
				t.Fatalf("dogu %s failed: %v, output:\n%s", strings.Join(tc.args, " "), err, output)
			}

			if !strings.Contains(output, tc.expectedOutput) {
				t.Errorf("expected output to contain %q, got:\n%s", tc.expectedOutput, output)
			}

			if !strings.Contains(output, "mirror="+tc.expectedMirror) {
				t.Errorf("expected debug log to show mirror=%q, got:\n%s", tc.expectedMirror, output)
			}
		})
	}
}
