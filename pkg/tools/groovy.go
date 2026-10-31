package tools

import (
	"context"
)

type GroovyTool struct{}

func init()                      { Register(&GroovyTool{}) }
func (*GroovyTool) Name() string { return "groovy" }
func (*GroovyTool) Install(ctx context.Context, spec, home, mirror string, opts InstallOptions) (*InstallResult, error) {
	d := groovyDefinition
	return d.Install(ctx, spec, home, mirror, opts)
}
