package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dogu-dev/dogu/pkg/config"
)

func TestFindAndLoadConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "a", "b", "c")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subDir: %v", err)
	}

	configFile := filepath.Join(tempDir, ".dogu.properties")
	content := []byte("java.mirror=https://mirror.example.com/java\nnode.mirror=https://mirror.example.com/node\n# comment\n")
	if err := os.WriteFile(configFile, content, 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	found, err := config.FindConfigFile(subDir)
	if err != nil {
		t.Fatalf("expected to find config file, got error: %v", err)
	}
	if found != configFile {
		t.Errorf("expected found path %s, got %s", configFile, found)
	}

	props, err := config.LoadProperties(subDir)
	if err != nil {
		t.Fatalf("expected to load properties, got error: %v", err)
	}

	if val := props.GetToolMirror("java"); val != "https://mirror.example.com/java" {
		t.Errorf("expected java mirror URL, got %s", val)
	}
	if val := props.GetToolMirror("node"); val != "https://mirror.example.com/node" {
		t.Errorf("expected node mirror URL, got %s", val)
	}
}
