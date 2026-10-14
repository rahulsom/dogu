package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

const ConfigFileName = ".dogu.properties"

// Properties represents configuration loaded from .dogu.properties.
type Properties struct {
	values map[string]string
	Path   string
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

// Get returns the property value for the key.
func (p *Properties) Get(key string) string {
	if p == nil || p.values == nil {
		return ""
	}
	return p.values[key]
}

// GetToolMirror returns the configured mirror URL for a given tool (e.g., node.mirror, java.mirror).
func (p *Properties) GetToolMirror(toolName string) string {
	return p.Get(toolName + ".mirror")
}
