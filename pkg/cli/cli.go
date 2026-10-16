package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/dogu-dev/dogu/pkg/config"
	"github.com/dogu-dev/dogu/pkg/env"
	"github.com/dogu-dev/dogu/pkg/tools"
)

type toolHelp struct {
	name        string
	description string
	examples    string
}

type ecosystemHelp struct {
	name  string
	tools []toolHelp
}

// PrintUsage prints usage instructions including the list of supported tools grouped by ecosystem.
func PrintUsage(out *os.File) {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	ecoTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	toolNameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	exampleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	flagNameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))

	var sb strings.Builder

	sb.WriteString(headerStyle.Render("Usage:") + "\n")
	sb.WriteString("  dogu [-v|--verbose] [--force] [tools...] -- <command> [args...]\n\n")

	sb.WriteString(headerStyle.Render("Supported Tools:") + "\n\n")

	ecosystems := []ecosystemHelp{
		{
			name: "Java / JVM Ecosystem",
			tools: []toolHelp{
				{
					name:        "java",
					description: "Major versions, exact versions, or vendor distribution suffixes (-zulu, -tem)",
					examples:    "--java 21, --java 17-zulu, --java 25.0.3.0+9-tem",
				},
				{
					name:        "maven",
					description: "Major versions, major.minor, or exact semver versions",
					examples:    "--maven 3, --maven 3.9, --maven 3.9.6",
				},
			},
		},
		{
			name: "Node.js / JavaScript Ecosystem",
			tools: []toolHelp{
				{
					name:        "node",
					description: "Major versions, major.minor, or exact semver versions",
					examples:    "--node 20, --node 22.11, --node 22.11.0",
				},
				{
					name:        "npm",
					description: "Major versions, major.minor, or exact semver versions",
					examples:    "--npm 10, --npm 10.2, --npm 10.2.4",
				},
				{
					name:        "pnpm",
					description: "Major versions, major.minor, or exact semver versions",
					examples:    "--pnpm 8, --pnpm 8.15, --pnpm 8.15.1",
				},
				{
					name:        "yarn",
					description: "Major versions, major.minor, or exact semver versions",
					examples:    "--yarn 1, --yarn 1.22, --yarn 1.22.19",
				},
				{
					name:        "bun",
					description: "Major versions, major.minor, or exact semver versions",
					examples:    "--bun 1, --bun 1.1, --bun 1.1.0",
				},
			},
		},
		{
			name: "Python Ecosystem",
			tools: []toolHelp{
				{
					name:        "python",
					description: "Major versions, major.minor, or exact semver versions",
					examples:    "--python 3, --python 3.11, --python 3.11.7",
				},
				{
					name:        "uv",
					description: "Major versions, major.minor, or exact semver versions",
					examples:    "--uv 0, --uv 0.5, --uv 0.5.1",
				},
			},
		},
	}

	for _, eco := range ecosystems {
		sb.WriteString("  " + ecoTitleStyle.Render(eco.name) + "\n")
		for _, tool := range eco.tools {
			sb.WriteString(fmt.Sprintf("    • %s: %s\n", toolNameStyle.Render(tool.name), descStyle.Render(tool.description)))
			sb.WriteString(fmt.Sprintf("      %s %s\n", exampleStyle.Render("Examples:"), exampleStyle.Render(tool.examples)))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(headerStyle.Render("Flags:") + "\n")
	sb.WriteString(fmt.Sprintf("  %s  %s\n", flagNameStyle.Render("-v, --verbose"), descStyle.Render("Enable verbose logging")))
	sb.WriteString(fmt.Sprintf("  %s    %s\n", flagNameStyle.Render("-f, --force"), descStyle.Render("Force re-downloading candidate versions even if already installed")))
	sb.WriteString(fmt.Sprintf("  %s     %s\n", flagNameStyle.Render("-h, --help"), descStyle.Render("Show usage information")))

	fmt.Fprint(out, sb.String())
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

	specs, force, verbose, err := parseToolSpecs(flagArgs)
	if err != nil {
		return err
	}

	log.SetOutput(os.Stderr)
	if verbose {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.WarnLevel)
	}

	log.Debug("Parsing CLI arguments", "flagArgs", flagArgs)
	log.Debug("Parsed tool specifications", "specs", specs, "force", force, "verbose", verbose)

	if len(cmdArgs) == 0 {
		PrintUsage(os.Stderr)
		return fmt.Errorf("no command provided to execute after '--'")
	}

	doguHome := env.GetDoguHome()
	log.Debug("Resolved dogu home directory", "path", doguHome)

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
			log.Debug("Provisioning tool candidate", "name", s.Name, "version", s.Version, "mirror", mirrorURL)
			res, err := tool.Install(ctx, s.Version, doguHome, mirrorURL, force)
			if err != nil {
				errs[idx] = fmt.Errorf("failed to provision tool %s (%s): %w", s.Name, s.Version, err)
				return
			}
			log.Debug("Tool candidate provisioned", "name", s.Name, "version", res.NormalizedVer, "dir", res.InstallDir)
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

	for k, v := range extraEnv {
		log.Debug("Setting environment variable", "key", k, "value", v)
	}
	if len(binDirs) > 0 {
		log.Debug("Prepending PATH entries", "binDirs", binDirs)
	}

	updatedEnv := env.BuildEnvMap(os.Environ(), binDirs, extraEnv)

	cmdName := cmdArgs[0]
	cmdFlags := cmdArgs[1:]

	resolvedCmdName := resolveExecutable(cmdName, updatedEnv)
	log.Debug("Resolved executable", "cmd", cmdName, "resolvedPath", resolvedCmdName)

	cmd := exec.Command(resolvedCmdName, cmdFlags...)
	cmd.Env = updatedEnv
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Debug("Executing sub-command", "cmd", resolvedCmdName, "args", cmdFlags)
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

func parseToolSpecs(args []string) ([]tools.Spec, bool, bool, error) {
	var specs []tools.Spec
	force := false
	verbose := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--force" || arg == "-f" {
			force = true
			continue
		}
		if arg == "--verbose" || arg == "-v" {
			verbose = true
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
					return nil, false, false, fmt.Errorf("flag --%s requires a version argument", toolName)
				}
				specs = append(specs, tools.Spec{Name: toolName, Version: args[i+1]})
				i++
			}
		}
	}
	return specs, force, verbose, nil
}
