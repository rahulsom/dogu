package tools

import (
	"context"
)

type KotlinTool struct{}

func init()                      { Register(&KotlinTool{}) }
func (*KotlinTool) Name() string { return "kotlin" }
func (*KotlinTool) Install(ctx context.Context, spec, home, mirror string, opts InstallOptions) (*InstallResult, error) {
	d := kotlinDefinition
	return d.Install(ctx, spec, home, mirror, opts)
}
