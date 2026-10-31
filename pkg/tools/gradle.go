package tools

import "context"

// GradleTool installs gradle using the shared tool definition.
type GradleTool struct{}

func init() { Register(&GradleTool{}) }

func (*GradleTool) Name() string { return "gradle" }

func (*GradleTool) Install(ctx context.Context, versionSpec, doguHome, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	return gradleDefinition.Install(ctx, versionSpec, doguHome, mirrorBaseURL, opts)
}
