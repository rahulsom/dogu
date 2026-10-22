package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dogu-dev/dogu"
	"github.com/dogu-dev/dogu/pkg/env"
)

const (
	ConfigFileName = ".dogu.properties"
	EnvDoguConfig  = "DOGU_CONFIGURATION"
)

// Config represents configuration holding tool base URLs and options.
type Config struct {
	BaseURLs          map[string]string `json:"baseUrls,omitempty"`
	IncludePrerelease bool              `json:"includePrerelease,omitempty"`
}

// Properties represents legacy property file contents or loaded configuration values.
type Properties struct {
	values map[string]string
	Path   string
}

// Options options for loading configuration.
type Options struct {
	StartDir   string
	ConfigFile string
}

// Option modifier for Options.
type Option func(*Options)

// WithStartDir sets custom start directory for .dogu.properties lookup.
func WithStartDir(dir string) Option {
	return func(o *Options) {
		o.StartDir = dir
	}
}

// WithConfigFile sets explicit configuration file path to load instead of defaults/env/doguHome.
func WithConfigFile(path string) Option {
	return func(o *Options) {
		o.ConfigFile = path
	}
}

// NewConfig returns a Config initialized with empty maps.
func NewConfig() *Config {
	return &Config{
		BaseURLs: make(map[string]string),
	}
}

// DefaultConfig returns a new Config populated with default tool base URLs.
func DefaultConfig() (*Config, error) {
	cfg := NewConfig()
	cleaned := stripJSONComments(dogu.DefaultConfigBytes)
	if err := json.Unmarshal(cleaned, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse embedded default config.json: %w", err)
	}
	if cfg.BaseURLs == nil {
		cfg.BaseURLs = make(map[string]string)
	}
	return cfg, nil
}

// Load loads default configuration, overrides with user config JSON (via DOGU_CONFIGURATION or DOGU_HOME/config.json),
// and overrides with hierarchical .dogu.properties if present.
func Load(opts ...Option) (*Config, error) {
	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}

	cfg, err := DefaultConfig()
	if err != nil {
		return nil, err
	}

	// Determine user config JSON file path
	userConfigFile := options.ConfigFile
	if userConfigFile == "" {
		if envPath := os.Getenv(EnvDoguConfig); envPath != "" {
			userConfigFile = envPath
		} else {
			doguHome := env.GetDoguHome()
			userConfigFile = filepath.Join(doguHome, "config.json")
		}
	}

	// Read user JSON config if file exists
	if userConfigFile != "" {
		if data, err := os.ReadFile(userConfigFile); err == nil {
			var userConfig Config
			cleaned := stripJSONComments(data)
			if err := json.Unmarshal(cleaned, &userConfig); err == nil {
				for k, v := range userConfig.BaseURLs {
					cfg.BaseURLs[k] = v
				}
				if userConfig.IncludePrerelease {
					cfg.IncludePrerelease = true
				}
			} else if os.Getenv(EnvDoguConfig) != "" && userConfigFile == os.Getenv(EnvDoguConfig) {
				return nil, fmt.Errorf("failed to parse configuration file at %s: %w", userConfigFile, err)
			}
		} else if envPath := os.Getenv(EnvDoguConfig); envPath != "" && userConfigFile == envPath {
			return nil, fmt.Errorf("failed to read configuration file specified by %s (%s): %w", EnvDoguConfig, userConfigFile, err)
		}
	}

	// Hierarchical .dogu.properties overrides
	props, err := LoadProperties(options.StartDir)
	if err == nil && props != nil {
		for key, val := range props.values {
			lowerKey := strings.ToLower(key)
			if lowerKey == "includeprerelease" || lowerKey == "include-prerelease" || lowerKey == "prerelease" || lowerKey == "rc" || lowerKey == "include-rc" {
				cfg.IncludePrerelease = (val == "true" || val == "1" || val == "yes")
			} else if strings.HasSuffix(key, ".mirror") {
				tool := strings.TrimSuffix(key, ".mirror")
				cfg.BaseURLs[tool] = val
			} else {
				cfg.BaseURLs[key] = val
			}
		}
	}

	return cfg, nil
}

// stripJSONComments removes JavaScript/C-style single-line (//), hash-style (#), and multi-line (/* */) comments
// from JSON bytes while respecting string literals and escapes.
func stripJSONComments(data []byte) []byte {
	out := make([]byte, 0, len(data))
	inString := false
	isEscaped := false
	inLineComment := false
	inBlockComment := false

	n := len(data)
	for i := 0; i < n; i++ {
		ch := data[i]

		if inLineComment {
			if ch == '\n' || ch == '\r' {
				inLineComment = false
				out = append(out, ch)
			}
			continue
		}

		if inBlockComment {
			if ch == '*' && i+1 < n && data[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}

		if inString {
			out = append(out, ch)
			if isEscaped {
				isEscaped = false
			} else if ch == '\\' {
				isEscaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}

		if ch == '"' {
			inString = true
			out = append(out, ch)
			continue
		}

		if ch == '/' && i+1 < n && data[i+1] == '/' {
			inLineComment = true
			i++
			continue
		}

		if ch == '#' {
			inLineComment = true
			continue
		}

		if ch == '/' && i+1 < n && data[i+1] == '*' {
			inBlockComment = true
			i++
			continue
		}

		out = append(out, ch)
	}

	return out
}

// GetBaseURL returns the configured base URL for a given tool (e.g., "node", "java").
func (c *Config) GetBaseURL(toolName string) string {
	if c == nil || c.BaseURLs == nil {
		return ""
	}
	return c.BaseURLs[toolName]
}

// GetToolMirror is an alias for GetBaseURL for backward compatibility.
func (c *Config) GetToolMirror(toolName string) string {
	return c.GetBaseURL(toolName)
}

// FindConfigFile searches for .dogu.properties starting from startDir and moving up the directory hierarchy.
func FindConfigFile(startDir string) (string, error) {
	if startDir == "" {
		var err error
		startDir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	curr := filepath.Clean(startDir)
	for {
		candidate := filepath.Join(curr, ConfigFileName)
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return candidate, nil
		}

		parent := filepath.Dir(curr)
		if parent == curr {
			// Reached root directory
			break
		}
		curr = parent
	}

	return "", os.ErrNotExist
}

// LoadProperties loads properties from .dogu.properties found by navigating up from startDir.
// If no file is found, an empty Properties struct is returned.
func LoadProperties(startDir string) (*Properties, error) {
	props := &Properties{values: make(map[string]string)}

	path, err := FindConfigFile(startDir)
	if err != nil {
		if os.IsNotExist(err) {
			return props, nil
		}
		return nil, err
	}

	props.Path = path
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			props.values[key] = val
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return props, nil
}

// Get returns property value for key.
func (p *Properties) Get(key string) string {
	if p == nil || p.values == nil {
		return ""
	}
	return p.values[key]
}

// GetToolMirror returns the configured mirror URL for a given tool (e.g., node.mirror, java.mirror).
func (p *Properties) GetToolMirror(toolName string) string {
	if val := p.Get(toolName + ".mirror"); val != "" {
		return val
	}
	return p.Get(toolName)
}
