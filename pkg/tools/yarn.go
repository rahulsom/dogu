package tools

import "context"

// YarnTool installs yarn using the shared tool definition.
type YarnTool struct{}

func init() { Register(&YarnTool{}) }

func (*YarnTool) Name() string { return "yarn" }

func (*YarnTool) Install(ctx context.Context, versionSpec, doguHome, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	return yarnDefinition.Install(ctx, versionSpec, doguHome, mirrorBaseURL, opts)
}
