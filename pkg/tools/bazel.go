package tools

import "context"

// BazelTool installs bazel using the shared tool definition.
type BazelTool struct{}

func init() { Register(&BazelTool{}) }

func (*BazelTool) Name() string { return "bazel" }

func (*BazelTool) Install(ctx context.Context, versionSpec, doguHome, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	return bazelDefinition.Install(ctx, versionSpec, doguHome, mirrorBaseURL, opts)
}
