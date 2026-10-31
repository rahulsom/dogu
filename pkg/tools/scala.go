package tools

import (
	"context"
	"strings"
)

type ScalaTool struct{}

func init()                     { Register(&ScalaTool{}) }
func (*ScalaTool) Name() string { return "scala" }
func (*ScalaTool) Install(ctx context.Context, spec, home, mirror string, opts InstallOptions) (*InstallResult, error) {
	d := scalaDefinition
	if strings.HasPrefix(CleanVersion(spec), "3") {
		d.baseURL = "https://github.com/scala/scala3/releases/download"
		d.source.owner, d.source.repository = "scala", "scala3"
		d.asset, d.downloadPath = "scala3-{version}.zip", "{version}/{asset}"
	}
	return d.Install(ctx, spec, home, mirror, opts)
}
