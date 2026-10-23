package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dogu-dev/dogu/pkg/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("expected DefaultConfig to succeed, got %v", err)
	}

	if val := cfg.GetBaseURL("node"); val != "https://nodejs.org/dist" {
		t.Errorf("expected default node base URL 'https://nodejs.org/dist', got '%s'", val)
	}
	if val := cfg.GetBaseURL("java"); val != "https://api.foojay.io/disco/v3.0/packages" {
		t.Errorf("expected default java base URL 'https://api.foojay.io/disco/v3.0/packages', got '%s'", val)
	}
}

func TestDoguHomeConfigOverride(t *testing.T) {
	tempDoguHome := t.TempDir()
	t.Setenv("DOGU_HOME", tempDoguHome)
	t.Setenv("DOGU_CONFIGURATION", "")

	userConfig := filepath.Join(tempDoguHome, "config.json")
	content := []byte(`{
		// Single line comment
		/* Multi-line
		   comment */
		# Hash comment
		"baseUrls": {
			"node": "https://custom-node-mirror.internal/dist", // node comment
			"java": "https://custom-java-mirror.internal/packages"
		}
	}`)
	if err := os.WriteFile(userConfig, content, 0644); err != nil {
		t.Fatalf("failed to write user config: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected Load to succeed, got %v", err)
	}

	if val := cfg.GetBaseURL("node"); val != "https://custom-node-mirror.internal/dist" {
		t.Errorf("expected overridden node URL, got '%s'", val)
	}
	if val := cfg.GetBaseURL("java"); val != "https://custom-java-mirror.internal/packages" {
		t.Errorf("expected overridden java URL, got '%s'", val)
	}
	// Verify unset properties retain default values
	if val := cfg.GetBaseURL("maven"); val != "https://archive.apache.org/dist/maven/maven-3" {
		t.Errorf("expected default maven URL, got '%s'", val)
	}
}

func TestDoguConfigurationEnvOverride(t *testing.T) {
	tempDir := t.TempDir()
	customConfigFile := filepath.Join(tempDir, "my-custom-config.json")
	content := []byte(`{"baseUrls": {"maven": "https://custom-maven.internal"}}`)
	if err := os.WriteFile(customConfigFile, content, 0644); err != nil {
		t.Fatalf("failed to write custom config: %v", err)
	}

	t.Setenv("DOGU_CONFIGURATION", customConfigFile)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected Load to succeed, got %v", err)
	}

	if val := cfg.GetBaseURL("maven"); val != "https://custom-maven.internal" {
		t.Errorf("expected overridden maven URL, got '%s'", val)
	}
}

func TestWithConfigFileOption(t *testing.T) {
	tempDir := t.TempDir()
	customConfigFile := filepath.Join(tempDir, "opt-config.json")
	content := []byte(`{"baseUrls": {"gradle": "https://custom-gradle.internal"}}`)
	if err := os.WriteFile(customConfigFile, content, 0644); err != nil {
		t.Fatalf("failed to write custom config: %v", err)
	}

	cfg, err := config.Load(config.WithConfigFile(customConfigFile))
	if err != nil {
		t.Fatalf("expected Load to succeed, got %v", err)
	}

	if val := cfg.GetBaseURL("gradle"); val != "https://custom-gradle.internal" {
		t.Errorf("expected overridden gradle URL, got '%s'", val)
	}
}

func TestProgrammaticConfigInjection(t *testing.T) {
	customCfg := config.NewConfig()
	customCfg.BaseURLs["python"] = "https://custom-python.internal"

	if val := customCfg.GetBaseURL("python"); val != "https://custom-python.internal" {
		t.Errorf("expected injected python URL, got '%s'", val)
	}
}
