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

	// Verify that generated configuration.adoc in build directory contains expected mirror configuration
	generatedConfigPath := filepath.Join(buildDocsDir, "configuration.adoc")
	data, err := os.ReadFile(generatedConfigPath)
	if err != nil {
		t.Fatalf("failed to read generated configuration.adoc at %s: %v", generatedConfigPath, err)
	}

	expectedComment := "# http://localhost:8080/archive.apache.org/ mirrors https://archive.apache.org/ on our proxy"
	expectedProperty := "maven.mirror=http://localhost:8080/archive.apache.org/dist/maven/maven-3"

	contentStr := string(data)
	if !strings.Contains(contentStr, expectedComment) {
		t.Errorf("generated configuration.adoc missing expected comment %q", expectedComment)
	}
	if !strings.Contains(contentStr, expectedProperty) {
		t.Errorf("generated configuration.adoc missing expected property %q", expectedProperty)
	}
}
