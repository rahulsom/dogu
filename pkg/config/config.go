package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dogu-dev/dogu"
	"github.com/dogu-dev/dogu/pkg/env"
)

const (
	EnvDoguConfig = "DOGU_CONFIGURATION"
)

// RewriteRule defines a URL prefix matching rule.
type RewriteRule struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// RewriteRules represents a list of URL rewrite rules.
type RewriteRules []RewriteRule

// UnmarshalJSON implements json.Unmarshaler for RewriteRules supporting both map and list formats.
func (r *RewriteRules) UnmarshalJSON(data []byte) error {
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		*r = nil
		return nil
	}

	if trimmed[0] == '[' {
		var list []RewriteRule
		if err := json.Unmarshal(trimmed, &list); err != nil {
			return err
		}
		*r = list
		return nil
	}

	if trimmed[0] == '{' {
		var m map[string]string
		if err := json.Unmarshal(trimmed, &m); err != nil {
			return err
		}
		var list []RewriteRule
		for k, v := range m {
			list = append(list, RewriteRule{From: k, To: v})
		}
		*r = list
		return nil
	}

	return fmt.Errorf("invalid rewrite configuration: expected object or array")
}

// Config represents configuration holding tool base URLs and URL rewrite rules.
type Config struct {
	BaseURLs map[string]string `json:"baseUrls,omitempty"`
	Rewrite  RewriteRules      `json:"rewrite,omitempty"`
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
		Rewrite:  RewriteRules{},
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
			cfg.Rewrite = append(cfg.Rewrite, userConfig.Rewrite...)
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

// RewriteURL applies configured rewrite rules to rawURL and returns the rewritten URL.
// If no rule matches, rawURL is returned unchanged.
func (c *Config) RewriteURL(rawURL string) string {
	if c == nil || len(c.Rewrite) == 0 {
		return rawURL
	}
	return c.Rewrite.RewriteURL(rawURL)
}

// RewriteURL applies rewrite rules to rawURL in order.
func (r RewriteRules) RewriteURL(rawURL string) string {
	for _, rule := range r {
		if rule.From != "" && strings.HasPrefix(rawURL, rule.From) {
			return rule.To + strings.TrimPrefix(rawURL, rule.From)
		}
	}
	return rawURL
}
