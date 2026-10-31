package tools

import "context"

// Buck2Tool installs buck2 using the shared tool definition.
type Buck2Tool struct{}

func init() { Register(&Buck2Tool{}) }

func (*Buck2Tool) Name() string { return "buck2" }

func (*Buck2Tool) Install(ctx context.Context, versionSpec, doguHome, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error) {
	return buck2Definition.Install(ctx, versionSpec, doguHome, mirrorBaseURL, opts)
}
