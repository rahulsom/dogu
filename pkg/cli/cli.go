package cli

import (
	"context"
	_ "embed"
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
	"github.com/dogu-dev/dogu/pkg/selfupdate"
	"github.com/dogu-dev/dogu/pkg/tools"
	"github.com/dogu-dev/dogu/pkg/version"
	"time"
)

//go:embed toolbox.txt
var toolboxArt string

type ecosystemHelp struct {
	name  string
	tools string
}

// PrintUsage prints usage instructions including the list of supported tools grouped by ecosystem.
func PrintUsage(out *os.File) {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))

	var sb strings.Builder

	// Explicit ANSI colors keep the toolbox palette independent of help heading styles.
	sb.WriteString(strings.NewReplacer(
		"{{steel}}", "\x1b[38;2;173;193;204m",
		"{{wood}}", "\x1b[38;2;216;163;101m",
		"{{handle}}", "\x1b[38;2;82;108;121m",
		"{{teal}}", "\x1b[38;2;57;165;185m",
		"{{yellow}}", "\x1b[38;2;242;188;85m",
		"{{rim}}", "\x1b[38;2;247;124;97m",
		"{{panel}}", "\x1b[38;2;133;47;45;48;2;239;102;80m",
		"{{side}}", "\x1b[38;2;116;47;45;48;2;173;48;44m",
		"{{latch}}", "\x1b[38;2;41;61;72;48;2;214;225;228m",
		"{{label}}", "\x1b[38;2;255;240;211;48;2;239;102;80m",
		"{{reset}}", "\x1b[0m",
	).Replace(toolboxArt))

	sb.WriteString("Run commands with the tool versions you need.\n\n")
	sb.WriteString(headerStyle.Render("Usage:") + "\n")
	sb.WriteString("  dogu [options] [--<tool> <version> ...] -- <command> [args...]\n")
	sb.WriteString("  dogu self-update\n")
	sb.WriteString("  dogu version\n")
	sb.WriteString("  dogu list-versions <tool> [version-prefix] [--limit N | --all]\n")
	sb.WriteString("  dogu autocomplete [bash|zsh|fish|powershell]\n")
	sb.WriteString("  dogu --help\n\n")
	sb.WriteString("  Tools are downloaded as needed and reused on later runs.\n")
	sb.WriteString("  Use -- to separate dogu options from the command and its arguments.\n\n")

	sb.WriteString(headerStyle.Render("Examples:") + "\n")
	for _, example := range []string{
		"dogu --node 22 -- node --version",
		"dogu --java 21 --maven 3.9 -- mvn clean test",
		"dogu --python 3.11 --uv 0.5 -- uv run pytest",
		"dogu --node 22 --pnpm 8.15 -- pnpm install",
	} {
		sb.WriteString("  " + valueStyle.Render(example) + "\n")
	}

	sb.WriteString("\n" + headerStyle.Render("Options:") + "\n")
	for _, option := range [][2]string{
		{"-h, --help", "Show this help"},
		{"-v, --verbose", "Show debug logs"},
		{"-f, --force", "Download tools again, even if already installed"},
		{"--prerelease", "Include prereleases when matching partial versions"},
		{"--no-prerelease", "Exclude prereleases (default)"},
	} {
		sb.WriteString("  " + valueStyle.Render(fmt.Sprintf("%-18s", option[0])) + option[1] + "\n")
	}

	sb.WriteString("\n" + headerStyle.Render("Supported tools:") + "\n")
	sb.WriteString("  Select one or more with --<tool> <version> or --<tool>=<version>.\n\n")
	for _, ecosystem := range []ecosystemHelp{
		{"Java / JVM", "java, groovy, kotlin, scala, maven, sbt, gradle"},
		{"Build tools", "ant, mill, bazel, pants, buck2"},
		{"JavaScript", "node, npm, pnpm, yarn, bun"},
		{"Python", "python, uv"},
		{"Other languages", "go, rust, ghc, r, julia, elm"},
	} {
		sb.WriteString(fmt.Sprintf("  %-18s%s\n", ecosystem.name, ecosystem.tools))
	}

	sb.WriteString("\n" + headerStyle.Render("Versions:") + "\n")
	sb.WriteString("  Use a major, major.minor, or exact version (e.g. 22, 22.11, 22.11.0).\n")
	sb.WriteString("  Partial versions select the newest matching installed version, or\n")
	sb.WriteString("  the newest matching release if none is installed.\n")
	sb.WriteString("  Java also accepts vendor suffixes: --java 17-zulu, --java 21-tem.\n")
	sb.WriteString("  Buck2 accepts release dates or tags: --buck2 2026-10-15.\n")

	sb.WriteString("\n" + headerStyle.Render("Commands:") + "\n")
	commands := [][2]string{
		{"self-update", "Update dogu to the latest release (official binaries only)"},
		{"version", "Print the dogu version"},
		{"list-versions <tool> [version-prefix] [--limit N | --all]", "List available versions (newest 20 by default)"},
		{"autocomplete [shell]", "Generate a shell completion script"},
	}
	width := 0
	for _, command := range commands {
		if len(command[0]) > width {
			width = len(command[0])
		}
	}
	for _, command := range commands {
		sb.WriteString("  " + valueStyle.Render(fmt.Sprintf("%-*s", width, command[0])) + "  " + command[1] + "\n")
	}

	fmt.Fprint(out, sb.String())
}

// Run parses CLI args, provisions tools, updates environment, and executes sub-command.
func Run(args []string) error {
	if handled, err := runInfoCommand(args, nil); handled {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load dogu configuration: %w", err)
	}
	return RunWithConfig(args, cfg)
}

// RunWithConfig parses CLI args, provisions tools using the provided config, updates environment, and executes sub-command.
func RunWithConfig(args []string, cfg *config.Config) error {
	if handled, err := runInfoCommand(args, cfg); handled {
		return err
	}
	if len(args) > 0 && args[0] == "self-update" {
		ctx := context.Background()
		curVer := version.GetVersion()
		return selfupdate.SelfUpdate(ctx, curVer, selfupdate.DefaultGitHubOwner, selfupdate.DefaultGitHubRepo)
	}

	// In binary distributions, check for an available update to suggest dogu self-update
	if version.IsBinaryDistribution() {
		defer checkAndSuggestUpdate()
	}

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

	specs, force, verbose, cliPrerelease, err := parseToolSpecs(flagArgs)
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

	if cfg == nil {
		var err error
		cfg, err = config.Load()
		if err != nil {
			return fmt.Errorf("failed to load dogu configuration: %w", err)
		}
	}

	includePrerelease := cliPrerelease != nil && *cliPrerelease

	installOpts := tools.InstallOptions{
		Force:             force,
		IncludePrerelease: includePrerelease,
		RewriteRules:      cfg.Rewrite,
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

			baseURL := cfg.GetBaseURL(s.Name)
			log.Debug("Provisioning tool candidate", "name", s.Name, "version", s.Version, "mirror", baseURL)
			res, err := tool.Install(ctx, s.Version, doguHome, baseURL, installOpts)
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

func checkAndSuggestUpdate() {
	curVer := version.GetVersion()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rel, available, err := selfupdate.CheckForUpdate(ctx, curVer, selfupdate.DefaultGitHubOwner, selfupdate.DefaultGitHubRepo)
	if err == nil && available && rel != nil {
		latestVer := rel.TagName
		if latestVer == "" {
			latestVer = rel.Name
		}
		selfupdate.SuggestUpdateNotification(os.Stderr, curVer, latestVer)
	}
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

func parseToolSpecs(args []string) ([]tools.Spec, bool, bool, *bool, error) {
	var specs []tools.Spec
	force := false
	verbose := false
	var includePrerelease *bool

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
		if arg == "--prerelease" || arg == "--include-prerelease" || arg == "--rc" || arg == "--include-rc" {
			val := true
			includePrerelease = &val
			continue
		}
		if arg == "--no-prerelease" || arg == "--include-prerelease=false" || arg == "--prerelease=false" || arg == "--rc=false" || arg == "--include-rc=false" {
			val := false
			includePrerelease = &val
			continue
		}
		if strings.HasPrefix(arg, "--prerelease=") || strings.HasPrefix(arg, "--include-prerelease=") || strings.HasPrefix(arg, "--rc=") || strings.HasPrefix(arg, "--include-rc=") {
			parts := strings.SplitN(arg, "=", 2)
			val := (parts[1] == "true" || parts[1] == "1" || parts[1] == "yes")
			includePrerelease = &val
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
					return nil, false, false, nil, fmt.Errorf("flag --%s requires a version argument", toolName)
				}
				specs = append(specs, tools.Spec{Name: toolName, Version: args[i+1]})
				i++
			}
		}
	}
	return specs, force, verbose, includePrerelease, nil
}
