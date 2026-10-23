package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dogu-dev/dogu"
	"github.com/dogu-dev/dogu/pkg/env"
)

const (
	EnvDoguConfig = "DOGU_CONFIGURATION"
)

// Config represents configuration holding tool base URLs.
type Config struct {
	BaseURLs map[string]string `json:"baseUrls,omitempty"`
}

// Options options for loading configuration.
type Options struct {
	ConfigFile string
}

// Option modifier for Options.
type Option func(*Options)

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

// Load loads default configuration and overrides with user config JSON (via DOGU_CONFIGURATION or DOGU_HOME/config.json).
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
			if err := json.Unmarshal(cleaned, &userConfig); err != nil {
				return nil, fmt.Errorf("failed to parse configuration file at %s: %w", userConfigFile, err)
			}
			for k, v := range userConfig.BaseURLs {
				cfg.BaseURLs[k] = v
			}
		} else if envPath := os.Getenv(EnvDoguConfig); envPath != "" && userConfigFile == envPath {
			return nil, fmt.Errorf("failed to read configuration file specified by %s (%s): %w", EnvDoguConfig, userConfigFile, err)
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
