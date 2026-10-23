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

func TestConfigFileValidationAndParseError(t *testing.T) {
	// 1. Verify root config.json and docs/config-example.json are valid configurations
	repoRoot := filepath.Join("..", "..")

	configFiles := []string{
		filepath.Join(repoRoot, "config.json"),
		filepath.Join(repoRoot, "docs", "config-example.json"),
	}

	for _, cfgFile := range configFiles {
		cfg, err := config.Load(config.WithConfigFile(cfgFile))
		if err != nil {
			t.Errorf("expected configuration file %s to be valid, got error: %v", cfgFile, err)
		} else if len(cfg.BaseURLs) == 0 {
			t.Errorf("expected configuration file %s to contain baseUrls", cfgFile)
		}
	}

	// 2. Verify invalid JSON in DOGU_HOME/config.json returns error
	tempDoguHome := t.TempDir()
	t.Setenv("DOGU_HOME", tempDoguHome)
	t.Setenv("DOGU_CONFIGURATION", "")

	userConfigFile := filepath.Join(tempDoguHome, "config.json")
	if err := os.WriteFile(userConfigFile, []byte(`{ invalid json`), 0644); err != nil {
		t.Fatalf("failed to write invalid user config: %v", err)
	}

	_, err := config.Load()
	if err == nil {
		t.Errorf("expected Load to fail when user config.json contains invalid JSON, but it succeeded")
	}
}

func TestRewriteRulesMapAndArrayFormats(t *testing.T) {
	tempDir := t.TempDir()

	// Map format
	mapConfigFile := filepath.Join(tempDir, "map-config.json")
	mapContent := []byte(`{
		"rewrite": {
			"https://github.com/adoptium/temurin25-binaries/releases/download/": "https://artifactory.internal/temurin25/"
		}
	}`)
	if err := os.WriteFile(mapConfigFile, mapContent, 0644); err != nil {
		t.Fatalf("failed to write map config: %v", err)
	}

	cfgMap, err := config.Load(config.WithConfigFile(mapConfigFile))
	if err != nil {
		t.Fatalf("failed to load map config: %v", err)
	}

	origURL := "https://github.com/adoptium/temurin25-binaries/releases/download/jdk-25.0.3%2B9/OpenJDK25U-jdk_s390x_linux_hotspot_25.0.3_9.tar.gz"
	expectedURL := "https://artifactory.internal/temurin25/jdk-25.0.3%2B9/OpenJDK25U-jdk_s390x_linux_hotspot_25.0.3_9.tar.gz"

	if rewritten := cfgMap.RewriteURL(origURL); rewritten != expectedURL {
		t.Errorf("map rewrite failed: got %q, expected %q", rewritten, expectedURL)
	}

	// Array format
	arrConfigFile := filepath.Join(tempDir, "arr-config.json")
	arrContent := []byte(`{
		"rewrite": [
			{
				"from": "https://github.com/adoptium/temurin25-binaries/releases/download/",
				"to": "https://artifactory.internal/temurin25/"
			}
		]
	}`)
	if err := os.WriteFile(arrConfigFile, arrContent, 0644); err != nil {
		t.Fatalf("failed to write arr config: %v", err)
	}

	cfgArr, err := config.Load(config.WithConfigFile(arrConfigFile))
	if err != nil {
		t.Fatalf("failed to load arr config: %v", err)
	}

	if rewritten := cfgArr.RewriteURL(origURL); rewritten != expectedURL {
		t.Errorf("array rewrite failed: got %q, expected %q", rewritten, expectedURL)
	}

	// Unmatched URL
	unmatchedURL := "https://github.com/other/releases/download/v1.0.0"
	if rewritten := cfgArr.RewriteURL(unmatchedURL); rewritten != unmatchedURL {
		t.Errorf("expected unmatched URL to remain unchanged, got %q", rewritten)
	}
}
