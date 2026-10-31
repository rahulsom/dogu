package tools

import "context"

// AntTool installs ant using the shared tool definition.
type AntTool struct{}

func init() { Register(&AntTool{}) }

func (*AntTool) Name() string { return "ant" }

func (*AntTool) Install(ctx context.Context, versionSpec, doguHome, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	return antDefinition.Install(ctx, versionSpec, doguHome, mirrorBaseURL, opts)
}
