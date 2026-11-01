package version

import (
	"os"
	"testing"
)

func TestGetVersion(t *testing.T) {
	oldVersion := Version
	defer func() { Version = oldVersion }()

	Version = "v1.2.3"
	if got := GetVersion(); got != "v1.2.3" {
		t.Errorf("GetVersion() = %q, want %q", got, "v1.2.3")
	}

	Version = "dev"
	gotDev := GetVersion()
	if gotDev == "" {
		t.Errorf("GetVersion() returned empty string for dev")
	}
}

func TestIsBinaryDistribution(t *testing.T) {
	oldVersion := Version
	defer func() { Version = oldVersion }()

	t.Setenv("DOGU_FORCE_BINARY_DISTRIBUTION", "true")
	if !IsBinaryDistribution() {
		t.Errorf("expected IsBinaryDistribution to be true when DOGU_FORCE_BINARY_DISTRIBUTION=true")
	}

	t.Setenv("DOGU_FORCE_BINARY_DISTRIBUTION", "false")
	if IsBinaryDistribution() {
		t.Errorf("expected IsBinaryDistribution to be false when DOGU_FORCE_BINARY_DISTRIBUTION=false")
	}

	_ = os.Unsetenv("DOGU_FORCE_BINARY_DISTRIBUTION")

	Version = "dev"
	if IsBinaryDistribution() {
		t.Errorf("expected IsBinaryDistribution() to be false when Version='dev'")
	}

	Version = ""
	if IsBinaryDistribution() {
		t.Errorf("expected IsBinaryDistribution() to be false when Version=''")
	}

	Version = "(devel)"
	if IsBinaryDistribution() {
		t.Errorf("expected IsBinaryDistribution() to be false when Version='(devel)'")
	}
}
