package tools

import "context"

// MavenTool installs maven using the shared tool definition.
type MavenTool struct{}

func init() { Register(&MavenTool{}) }

func (*MavenTool) Name() string { return "maven" }

func (*MavenTool) Install(ctx context.Context, versionSpec, doguHome, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	return mavenDefinition.Install(ctx, versionSpec, doguHome, mirrorBaseURL, opts)
}

func resolveMavenVersion(ctx context.Context, versionSpec, baseURL string, includePrerelease bool) string {
	return mavenDefinition.resolveVersion(ctx, versionSpec, baseURL, includePrerelease)
}
