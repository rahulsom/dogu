package tools

import "context"

// MillTool installs mill using the shared tool definition.
type MillTool struct{}

func init() { Register(&MillTool{}) }

func (*MillTool) Name() string { return "mill" }

func (*MillTool) Install(ctx context.Context, versionSpec, doguHome, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	return millDefinition.Install(ctx, versionSpec, doguHome, mirrorBaseURL, opts)
}
