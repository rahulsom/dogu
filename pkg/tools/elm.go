package tools

import (
	"context"
	"fmt"
	"runtime"
)

type ElmTool struct{}

func init()                   { Register(&ElmTool{}) }
func (*ElmTool) Name() string { return "elm" }
func (*ElmTool) Install(ctx context.Context, spec, home, mirror string, opts InstallOptions) (*InstallResult, error) {
	d := elmDefinition
	if runtime.GOARCH != "amd64" && !(runtime.GOOS == "darwin" && runtime.GOARCH == "arm64") {
		return nil, fmt.Errorf("elm supports x86-64 and macOS ARM64 binaries")
	}
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		d.asset = "binary-for-mac-64-bit-ARM.gz"
	}
	return d.Install(ctx, spec, home, mirror, opts)
}
