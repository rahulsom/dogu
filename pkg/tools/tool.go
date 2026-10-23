package tools

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Spec represents a requested tool version specification.
type Spec struct {
	Name    string
	Version string
}

type RewriteRule interface {
	RewriteURL(url string) string
}

// InstallOptions options for installing a tool candidate.
type InstallOptions struct {
	Force             bool
	IncludePrerelease bool
	RewriteRules      RewriteRule
}

// InstallResult holds information about an installed candidate tool.
type InstallResult struct {
	ToolName      string
	RequestedVer  string
	NormalizedVer string
	InstallDir    string
	HomeDir       string
	BinDirs       []string
	EnvVars       map[string]string
}

// Tool defines the interface for tool providers.
type Tool interface {
	Name() string
	Install(ctx context.Context, version string, doguHome string, mirrorBaseURL string, opts InstallOptions) (*InstallResult, error)
}

var (
	registry = make(map[string]Tool)
	mu       sync.RWMutex
)

// Register registers a tool provider.
func Register(t Tool) {
	mu.Lock()
	defer mu.Unlock()
	registry[t.Name()] = t
}

// Get retrieves a registered tool provider.
func Get(name string) (Tool, bool) {
	mu.RLock()
	defer mu.RUnlock()
	t, ok := registry[name]
	return t, ok
}

// List returns the names of all registered tools sorted alphabetically.
func List() []string {
	mu.RLock()
	defer mu.RUnlock()
	var names []string
	for k := range registry {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// FindTool returns error if tool is not registered.
func FindTool(name string) (Tool, error) {
	t, ok := Get(name)
	if !ok {
		return nil, fmt.Errorf("unsupported or unknown tool: %s", name)
	}
	return t, nil
}
