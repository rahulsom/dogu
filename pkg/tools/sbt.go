package tools

import "context"

// SBTTool installs sbt using the shared tool definition.
type SBTTool struct{}

func init() { Register(&SBTTool{}) }

func (*SBTTool) Name() string { return "sbt" }

func (*SBTTool) Install(ctx context.Context, versionSpec, doguHome, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	return sbtDefinition.Install(ctx, versionSpec, doguHome, mirrorBaseURL, opts)
}
