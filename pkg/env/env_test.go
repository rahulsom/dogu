package env_test

import (
	"strings"
	"testing"

	"github.com/dogu-dev/dogu/pkg/env"
)

func TestExtractMajorVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"17", "17"},
		{"21-zulu", "21"},
		{"25.0.3.0+9-tem", "25"},
		{"v24.1.0", "24"},
	}

	for _, tt := range tests {
		res := env.ExtractMajorVersion(tt.input)
		if res != tt.expected {
			t.Errorf("ExtractMajorVersion(%s) = %s; want %s", tt.input, res, tt.expected)
		}
	}
}

func TestFormatEnvKey(t *testing.T) {
	if res := env.FormatEnvKey("java", "17"); res != "JAVA_17_HOME" {
		t.Errorf("expected JAVA_17_HOME, got %s", res)
	}
	if res := env.FormatEnvKey("java", ""); res != "JAVA_HOME" {
		t.Errorf("expected JAVA_HOME, got %s", res)
	}
	if res := env.FormatEnvKey("node", "24"); res != "NODE_24_HOME" {
		t.Errorf("expected NODE_24_HOME, got %s", res)
	}
}

func TestBuildEnvMap(t *testing.T) {
	current := []string{"FOO=BAR", "PATH=/usr/bin:/bin"}
	binPaths := []string{"/home/user/.dogu/candidates/java/17/bin", "/home/user/.dogu/candidates/node/24/bin"}
	extraEnv := map[string]string{
		"JAVA_17_HOME": "/home/user/.dogu/candidates/java/17",
		"JAVA_HOME":    "/home/user/.dogu/candidates/java/17",
	}

	result := env.BuildEnvMap(current, binPaths, extraEnv)
	resMap := make(map[string]string)
	for _, item := range result {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) == 2 {
			resMap[parts[0]] = parts[1]
		}
	}

	if resMap["FOO"] != "BAR" {
		t.Errorf("expected FOO=BAR, got %s", resMap["FOO"])
	}
	if resMap["JAVA_HOME"] != "/home/user/.dogu/candidates/java/17" {
		t.Errorf("expected JAVA_HOME set, got %s", resMap["JAVA_HOME"])
	}

	expectedPathPrefix := "/home/user/.dogu/candidates/java/17/bin:/home/user/.dogu/candidates/node/24/bin"
	if !strings.HasPrefix(resMap["PATH"], expectedPathPrefix) {
		t.Errorf("expected PATH to start with prepended bin paths, got %s", resMap["PATH"])
	}
}
