package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func expandArgsFile(args []string) ([]string, error) {
	fileIndex := -1
	for i, arg := range args {
		if strings.HasPrefix(arg, "@") {
			if fileIndex >= 0 {
				return nil, fmt.Errorf("only one args file can be specified")
			}
			fileIndex = i
		}
	}

	if fileIndex >= 0 {
		path := strings.TrimPrefix(args[fileIndex], "@")
		if path == "" {
			return nil, fmt.Errorf("args file path after @ is empty")
		}
		fileArgs, err := readArgsFile(path)
		if err != nil {
			return nil, err
		}
		expanded := make([]string, 0, len(args)-1+len(fileArgs))
		expanded = append(expanded, args[:fileIndex]...)
		expanded = append(expanded, fileArgs...)
		return append(expanded, args[fileIndex+1:]...), nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("find dogu.args: %w", err)
	}
	for {
		path := filepath.Join(dir, "dogu.args")
		fileArgs, err := readArgsFile(path)
		if err == nil {
			return append(fileArgs, args...), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return args, nil
		}
		dir = parent
	}
}

func readArgsFile(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read args file %q: %w", path, err)
	}
	var args []string
	for _, line := range strings.Split(string(content), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			args = append(args, strings.Fields(line)...)
		}
	}
	if _, _, _, _, _, err := parseToolSpecsMode(args, true); err != nil {
		return nil, fmt.Errorf("invalid args file %q: %w", path, err)
	}
	return args, nil
}
