package docs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
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
	t.Fatalf("failed to find repo root from %s", dir)
	return ""
}

func TestGenerateDocumentation(t *testing.T) {
	repoRoot := findRepoRoot(t)
	docsDir := filepath.Join(repoRoot, "docs")
	buildDocsDir := filepath.Join(repoRoot, "build", "docs")

	if err := os.MkdirAll(buildDocsDir, 0755); err != nil {
		t.Fatalf("failed to create build/docs directory: %v", err)
	}

	entries, err := os.ReadDir(docsDir)
	if err != nil {
		t.Fatalf("failed to read docs directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".adoc") {
			continue
		}

		srcPath := filepath.Join(docsDir, entry.Name())
		content, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatalf("failed to read %s: %v", srcPath, err)
		}

		dstPath := filepath.Join(buildDocsDir, entry.Name())
		if err := os.WriteFile(dstPath, content, 0644); err != nil {
			t.Fatalf("failed to write %s: %v", dstPath, err)
		}
	}

	// Verify that generated configuration.adoc in build directory contains expected mirror configurations
	generatedConfigPath := filepath.Join(buildDocsDir, "configuration.adoc")
	data, err := os.ReadFile(generatedConfigPath)
	if err != nil {
		t.Fatalf("failed to read generated configuration.adoc at %s: %v", generatedConfigPath, err)
	}

	contentStr := string(data)

	expectedProperties := []string{
		"java.mirror=http://localhost:8080/api.foojay.io/disco/v3.0/packages",
		"node.mirror=http://localhost:8080/nodejs.org/dist",
		"maven.mirror=http://localhost:8080/archive.apache.org/dist/maven/maven-3",
		"sbt.mirror=http://localhost:8080/github.com/sbt/sbt/releases/download",
		"gradle.mirror=http://localhost:8080/services.gradle.org/distributions",
		"ant.mirror=http://localhost:8080/archive.apache.org/dist/ant/binaries",
		"mill.mirror=http://localhost:8080/github.com/com-lihaoyi/mill/releases/download",
		"bazel.mirror=http://localhost:8080/github.com/bazelbuild/bazel/releases/download",
		"pants.mirror=http://localhost:8080/github.com/pantsbuild/pants/releases/download",
		"buck2.mirror=http://localhost:8080/github.com/facebook/buck2/releases/download",
		"npm.mirror=http://localhost:8080/registry.npmjs.org/npm/-",
		"yarn.mirror=http://localhost:8080/github.com/yarnpkg/yarn/releases/download",
		"pnpm.mirror=http://localhost:8080/github.com/pnpm/pnpm/releases/download",
		"bun.mirror=http://localhost:8080/github.com/oven-sh/bun/releases/download",
		"python.mirror=http://localhost:8080/github.com/indygreg/python-build-standalone/releases/download",
		"uv.mirror=http://localhost:8080/github.com/astral-sh/uv/releases/download",
	}

	for _, prop := range expectedProperties {
		if !strings.Contains(contentStr, prop) {
			t.Errorf("generated configuration.adoc missing expected property %q", prop)
		}
	}
}
