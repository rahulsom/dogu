package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/dogu-dev/dogu/pkg/config"
	"github.com/dogu-dev/dogu/pkg/env"
	"github.com/dogu-dev/dogu/pkg/tools"
)

// PrintUsage prints usage instructions including the list of supported tools.
func PrintUsage(out *os.File) {
	supported := tools.List()
	sort.Strings(supported)
	var formatted []string
	for _, t := range supported {
		formatted = append(formatted, "--"+t+" <version>")
	}

	fmt.Fprintf(out, "Usage: dogu [--force] [%s] -- <command> [args...]\n\n", strings.Join(formatted, " | "))
	fmt.Fprintf(out, "Supported tools:\n")
	for _, t := range supported {
		fmt.Fprintf(out, "  - %s\n", t)
	}
	fmt.Fprintf(out, "\nFlags:\n")
	fmt.Fprintf(out, "  --force        Force re-downloading candidate versions even if already installed\n")
	fmt.Fprintf(out, "  --help, -h     Show usage information\n")
}

// Run parses CLI args, provisions tools, updates environment, and executes sub-command.
func Run(args []string) error {
	doubleDashIdx := -1
	for i, arg := range args {
		if arg == "--" {
			doubleDashIdx = i
			break
		}
	}

	var flagArgs []string
	var cmdArgs []string

	if doubleDashIdx != -1 {
		flagArgs = args[:doubleDashIdx]
		cmdArgs = args[doubleDashIdx+1:]
	} else {
		flagArgs = args
	}

	for _, arg := range flagArgs {
		if arg == "--help" || arg == "-h" {
			PrintUsage(os.Stdout)
			return nil
		}
	}

	specs, force, err := parseToolSpecs(flagArgs)
	if err != nil {
		return err
	}

	if len(cmdArgs) == 0 {
		PrintUsage(os.Stderr)
		return fmt.Errorf("no command provided to execute after '--'")
	}

	doguHome := env.GetDoguHome()
	props, err := config.LoadProperties("")
	if err != nil {
		return fmt.Errorf("failed to load dogu properties: %w", err)
	}

	ctx := context.Background()

	results := make([]*tools.InstallResult, len(specs))
	errs := make([]error, len(specs))

	var wg sync.WaitGroup
	for i, spec := range specs {
		wg.Add(1)
		go func(idx int, s tools.Spec) {
			defer wg.Done()
			tool, err := tools.FindTool(s.Name)
			if err != nil {
				errs[idx] = err
				return
			}

			mirrorURL := props.GetToolMirror(s.Name)
			res, err := tool.Install(ctx, s.Version, doguHome, mirrorURL, force)
			if err != nil {
				errs[idx] = fmt.Errorf("failed to provision tool %s (%s): %w", s.Name, s.Version, err)
				return
			}
			results[idx] = res
		}(i, spec)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	var binDirs []string
	extraEnv := make(map[string]string)

	for _, res := range results {
		// Prepend binDirs so the LAST requested tool version appears FIRST in PATH
		binDirs = append(res.BinDirs, binDirs...)
		for k, v := range res.EnvVars {
			extraEnv[k] = v
		}
	}

	updatedEnv := env.BuildEnvMap(os.Environ(), binDirs, extraEnv)

	cmdName := cmdArgs[0]
	cmdFlags := cmdArgs[1:]

	resolvedCmdName := resolveExecutable(cmdName, updatedEnv)

	cmd := exec.Command(resolvedCmdName, cmdFlags...)
	cmd.Env = updatedEnv
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func resolveExecutable(cmdName string, envSlice []string) string {
	if filepath.IsAbs(cmdName) || strings.Contains(cmdName, string(os.PathSeparator)) {
		return cmdName
	}

	var pathVal string
	for _, e := range envSlice {
		if strings.HasPrefix(e, "PATH=") {
			pathVal = strings.TrimPrefix(e, "PATH=")
			break
		}
	}

	if pathVal == "" {
		return cmdName
	}

	dirs := filepath.SplitList(pathVal)
	for _, dir := range dirs {
		target := filepath.Join(dir, cmdName)
		if fi, err := os.Stat(target); err == nil && !fi.IsDir() && isExecutable(fi) {
			return target
		}
		if os.PathSeparator == '\\' {
			for _, ext := range []string{".exe", ".cmd", ".bat"} {
				targetExt := target + ext
				if fi, err := os.Stat(targetExt); err == nil && !fi.IsDir() {
					return targetExt
				}
			}
		}
	}

	return cmdName
}

func isExecutable(fi os.FileInfo) bool {
	return fi.Mode()&0111 != 0
}

func parseToolSpecs(args []string) ([]tools.Spec, bool, error) {
	var specs []tools.Spec
	force := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--force" {
			force = true
			continue
		}
		if arg == "--help" || arg == "-h" {
			continue
		}

		if strings.HasPrefix(arg, "--") {
			toolName := strings.TrimPrefix(arg, "--")
			if strings.Contains(toolName, "=") {
				parts := strings.SplitN(toolName, "=", 2)
				specs = append(specs, tools.Spec{Name: parts[0], Version: parts[1]})
			} else {
				if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
					return nil, false, fmt.Errorf("flag --%s requires a version argument", toolName)
				}
				specs = append(specs, tools.Spec{Name: toolName, Version: args[i+1]})
				i++
			}
		}
	}
	return specs, force, nil
}
