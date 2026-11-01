package version

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
)

// Version is populated at build time via -ldflags="-X github.com/dogu-dev/dogu/pkg/version.Version=...".
// Defaults to "dev" if not set during compilation.
var Version = "dev"

// GetVersion returns the effective version of dogu.
// It prioritizes the build-time Version variable. If unset or "dev",
// it attempts to read the module version from runtime/debug.BuildInfo.
func GetVersion() string {
	if Version != "" && Version != "dev" && Version != "(devel)" {
		return Version
	}

	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
	}

	return Version
}

// IsBinaryDistribution reports whether the running binary is a distributed/released binary
// rather than being run via `go run` or an unversioned local development build.
//
// Self-update and update checking are restricted to binary distributions to avoid
// attempting to self-update development source trees or temporary go-run binaries.
func IsBinaryDistribution() bool {
	// Provide an environment variable override for testing or manual overrides.
	if forced := os.Getenv("DOGU_FORCE_BINARY_DISTRIBUTION"); forced != "" {
		return forced == "true" || forced == "1" || forced == "yes"
	}

	v := GetVersion()
	if v == "" || v == "dev" || v == "(devel)" {
		return false
	}

	exe, err := os.Executable()
	if err != nil {
		return false
	}

	// Resolve any symlinks in the executable path
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	// Check if running from a go-build temporary directory (used by `go run`)
	lowerExe := strings.ToLower(exe)
	if strings.Contains(lowerExe, "/go-build") ||
		strings.Contains(lowerExe, "\\go-build") ||
		strings.Contains(lowerExe, "/tmp/go-build") ||
		strings.Contains(lowerExe, "\\temp\\go-build") ||
		strings.Contains(lowerExe, "/b001/exe/") ||
		strings.Contains(lowerExe, "\\b001\\exe\\") {
		return false
	}

	return true
}
