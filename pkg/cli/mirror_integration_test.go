package cli_test

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	out, err := runCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to start cachew container: %v, output: %s", err, string(out))
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
		propContent    string
		args           []string
		expectedOutput string
		expectedMirror string
	}{
		{
			name: "maven-via-cachew-mirror",
			propContent: fmt.Sprintf(
				"maven.mirror=%s/archive.apache.org/dist/maven/maven-3\n",
				cachewURL,
			),
			args:           []string{"-v", "--force", "--maven", "3.9.6", "--", "mvn", "-version"},
			expectedOutput: "3.9.6",
			expectedMirror: fmt.Sprintf("%s/archive.apache.org/dist/maven/maven-3", cachewURL),
		},
		{
			name: "node-via-cachew-mirror",
			propContent: fmt.Sprintf(
				"node.mirror=%s/nodejs.org/dist\n",
				cachewURL,
			),
			args:           []string{"-v", "--force", "--node", "20.18.0", "--", "node", "--version"},
			expectedOutput: "v20.18.0",
			expectedMirror: fmt.Sprintf("%s/nodejs.org/dist", cachewURL),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			workDir := t.TempDir()
			doguHome := filepath.Join(workDir, "dogu_home")

			propFile := filepath.Join(workDir, ".dogu.properties")
			if err := os.WriteFile(propFile, []byte(tc.propContent), 0644); err != nil {
				t.Fatalf("failed to write .dogu.properties: %v", err)
			}

			cmd := exec.Command(doguBin, tc.args...)
			cmd.Dir = workDir
			cmd.Env = append(os.Environ(), "DOGU_HOME="+doguHome)

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
