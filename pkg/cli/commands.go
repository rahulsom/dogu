package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dogu-dev/dogu/pkg/config"
	"github.com/dogu-dev/dogu/pkg/tools"
	"github.com/dogu-dev/dogu/pkg/version"
)

func runInfoCommand(args []string, cfg *config.Config) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	switch args[0] {
	case "version":
		if len(args) != 1 {
			return true, fmt.Errorf("usage: dogu version")
		}
		_, err := fmt.Fprintln(os.Stdout, version.GetVersion())
		return true, err
	case "autocomplete":
		if len(args) > 2 {
			return true, fmt.Errorf("usage: dogu autocomplete [bash|zsh|fish|powershell]")
		}
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = os.Getenv("COMSPEC")
		}
		if len(args) == 2 {
			shell = args[1]
		}
		return true, writeAutocomplete(os.Stdout, shell)
	case "list-versions":
		opts, err := parseVersionListArgs(args[1:])
		if err != nil {
			return true, err
		}
		if _, err := tools.FindTool(opts.tool); err != nil {
			return true, err
		}
		if cfg == nil {
			var err error
			cfg, err = config.Load()
			if err != nil {
				return true, err
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		versions, err := tools.ListVersions(ctx, opts.tool, cfg.GetBaseURL(opts.tool))
		if err != nil {
			return true, err
		}
		return true, writeVersionList(os.Stdout, os.Stderr, versions, opts)
	}
	return false, nil
}

func writeAutocomplete(out io.Writer, shell string) error {
	shell = strings.ToLower(strings.TrimSuffix(filepath.Base(shell), ".exe"))
	names := tools.List()
	words := "version list-versions autocomplete self-update --help -h --verbose -v --force -f --prerelease --"
	for _, name := range names {
		words += " --" + name
	}
	toolWords := strings.Join(names, " ") + " --all --limit"
	shells := "bash zsh fish powershell"
	var script string
	switch shell {
	case "bash":
		script = fmt.Sprintf(`_dogu() {
    local words='%s'
    case "${COMP_WORDS[1]}" in
        list-versions) words='%s' ;;
        autocomplete) words='%s' ;;
    esac
    COMPREPLY=( $(compgen -W "$words" -- "${COMP_WORDS[COMP_CWORD]}") )
}
complete -F _dogu dogu
`, words, toolWords, shells)
	case "zsh":
		script = fmt.Sprintf(`#compdef dogu
_dogu() {
    local -a choices
    case "$words[2]" in
        list-versions) choices=(%s) ;;
        autocomplete) choices=(%s) ;;
        *) choices=(%s) ;;
    esac
    compadd -- "${choices[@]}"
}
compdef _dogu dogu
`, toolWords, shells, words)
	case "fish":
		script = fmt.Sprintf("complete -c dogu -f -n '__fish_use_subcommand' -a '%s'\ncomplete -c dogu -f -n '__fish_seen_subcommand_from list-versions' -a '%s'\ncomplete -c dogu -f -n '__fish_seen_subcommand_from autocomplete' -a '%s'\n", words, toolWords, shells)
	case "powershell", "pwsh":
		script = fmt.Sprintf(`Register-ArgumentCompleter -Native -CommandName dogu -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    $choices = '%s'.Split(' ')
    if ($commandAst.CommandElements.Count -gt 1) {
        switch ($commandAst.CommandElements[1].ToString()) {
            'list-versions' { $choices = '%s'.Split(' ') }
            'autocomplete' { $choices = '%s'.Split(' ') }
        }
    }
    $choices | Where-Object { $_.StartsWith($wordToComplete) } | ForEach-Object {
        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
    }
}
`, words, toolWords, shells)
	default:
		return fmt.Errorf("cannot generate autocomplete for shell %q; specify bash, zsh, fish, or powershell", shell)
	}
	_, err := io.WriteString(out, script)
	return err
}

type versionListOptions struct {
	tool, prefix string
	limit        int
}

func parseVersionListArgs(args []string) (versionListOptions, error) {
	opts := versionListOptions{limit: 20}
	all, explicitLimit := false, false
	usage := "usage: dogu list-versions <tool> [version-prefix] [--limit N | --all]"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--all":
			all = true
		case "--limit":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("%s", usage)
			}
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 {
				return opts, fmt.Errorf("--limit must be a positive integer")
			}
			opts.limit = n
			explicitLimit = true
		default:
			if strings.HasPrefix(args[i], "-") {
				return opts, fmt.Errorf("unknown option %s; %s", args[i], usage)
			}
			if opts.tool == "" {
				opts.tool = args[i]
			} else if opts.prefix == "" {
				opts.prefix = args[i]
			} else {
				return opts, fmt.Errorf("%s", usage)
			}
		}
	}
	if opts.tool == "" || all && explicitLimit {
		return opts, fmt.Errorf("%s", usage)
	}
	if all {
		opts.limit = 0
	}
	return opts, nil
}

func writeVersionList(out, notices io.Writer, versions []string, opts versionListOptions) error {
	var matches []string
	for _, v := range versions {
		if opts.prefix == "" || tools.MatchVersion(v, opts.prefix) {
			matches = append(matches, v)
		}
	}
	if len(matches) == 0 {
		return fmt.Errorf("no versions match %q for %s", opts.prefix, opts.tool)
	}
	count := len(matches)
	if opts.limit > 0 && count > opts.limit {
		count = opts.limit
	}
	for _, v := range matches[:count] {
		if _, err := fmt.Fprintln(out, v); err != nil {
			return err
		}
	}
	if count < len(matches) {
		_, err := fmt.Fprintf(notices, "Showing %d of %d versions. Use --all for every version, --limit N for more, or a version prefix to narrow the list.\n", count, len(matches))
		return err
	}
	return nil
}
