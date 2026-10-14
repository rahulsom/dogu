package env

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GetDoguHome returns $DOGU_HOME or defaults to $HOME/.dogu
func GetDoguHome() string {
	if home := os.Getenv("DOGU_HOME"); home != "" {
		return home
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		userHome = os.Getenv("HOME")
	}
	return filepath.Join(userHome, ".dogu")
}

// SanitizeVersionForEnv converts version string into env var friendly format (e.g., 21.0.3 -> 21_0_3 or major 17 -> 17)
func SanitizeVersionForEnv(ver string) string {
	// Extract leading digits or sanitize characters
	re := regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	sanitized := re.ReplaceAllString(ver, "_")
	return strings.Trim(sanitized, "_")
}

// ExtractMajorVersion extracts the major version number from a version string (e.g., "17.0.2" -> "17", "21-zulu" -> "21")
func ExtractMajorVersion(ver string) string {
	var digits []rune
	for _, r := range ver {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
		} else if len(digits) > 0 {
			break
		}
	}
	if len(digits) > 0 {
		return string(digits)
	}
	return ver
}

// FormatEnvKey returns key like JAVA_17_HOME or JAVA_HOME or NODE_24_HOME
func FormatEnvKey(toolName string, subKey string) string {
	prefix := strings.ToUpper(strings.ReplaceAll(toolName, "-", "_"))
	if subKey != "" {
		return prefix + "_" + strings.ToUpper(subKey) + "_HOME"
	}
	return prefix + "_HOME"
}

// BuildEnvMap creates updated environment map with PATH prepended and tool-specific env vars set
func BuildEnvMap(currentEnv []string, binPaths []string, extraEnv map[string]string) []string {
	envMap := make(map[string]string)
	for _, e := range currentEnv {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	for k, v := range extraEnv {
		envMap[k] = v
	}

	if len(binPaths) > 0 {
		existingPath := envMap["PATH"]
		newPath := strings.Join(binPaths, string(os.PathListSeparator))
		if existingPath != "" {
			newPath = newPath + string(os.PathListSeparator) + existingPath
		}
		envMap["PATH"] = newPath
	}

	var result []string
	for k, v := range envMap {
		result = append(result, k+"="+v)
	}
	return result
}
