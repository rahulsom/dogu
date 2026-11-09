package cli_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dogu-dev/dogu/pkg/config"
	"github.com/dogu-dev/dogu/pkg/tools"
)

type docCommand struct {
	command, contains string
	exitCode          int // Some upstream launchers return 1 even for successful help.
}
type toolDocExample struct {
	tool     string
	commands []docCommand
}

// These exact shell commands are both rendered in the reference and executed
// against real installations. Library-only tools use a load/version check first.
var toolDocExamples = []toolDocExample{
	{"java", []docCommand{
		{command: `dogu --java 21 -- java --help`, contains: `usage`},
	}},
	{"groovy", []docCommand{
		{command: `dogu --java 21 --groovy 4.0 -- groovy --help`, contains: `usage`},
		{command: `dogu --java 21 --groovy 4.0 -- groovy -e 'println "hello"'`, contains: `hello`},
	}},
	{"kotlin", []docCommand{
		{command: `dogu --java 21 --kotlin 2.1 -- kotlin -help`, contains: `usage`},
		{command: `dogu --java 21 --kotlin 2.1 -- kotlin -e 'println("hello")'`, contains: `hello`},
	}},
	{"scala", []docCommand{
		{command: `dogu --java 21 --scala 3.3 -- scala -help`, contains: `usage`},
		{command: `dogu --java 21 --scala 3.3 -- scala -e 'println("hello")'`, contains: `hello`},
	}},
	{"maven", []docCommand{
		{command: `dogu --java 21 --maven 3.9 -- mvn --help`, contains: `usage`},
	}},
	{"sbt", []docCommand{
		{command: `dogu --java 21 --sbt 1 -- sbt --help`, contains: `usage`, exitCode: 1},
	}},
	{"gradle", []docCommand{
		{command: `dogu --java 21 --gradle 8 -- gradle --help`, contains: `usage`},
	}},
	{"clojure", []docCommand{
		{command: `dogu --java 21 --clojure 1.12 -- clojure -h`, contains: `usage`},
		{command: `dogu --java 21 --clojure 1.12 -- clojure -M -e '(println "hello")'`, contains: `hello`},
	}},
	{"leiningen", []docCommand{
		{command: `dogu --java 21 --leiningen 2.13 -- lein help`, contains: `tasks`},
	}},
	{"ant", []docCommand{
		{command: `dogu --java 21 --ant 1.10 -- ant -help`, contains: `-projecthelp`},
	}},
	{"mill", []docCommand{
		{command: `dogu --java 21 --mill 0.12 -- mill --help`, contains: `usage`},
	}},
	{"bazel", []docCommand{
		{command: `dogu --bazel 7 -- bazel help`, contains: `usage`},
	}},
	{"pants", []docCommand{
		{command: `dogu --pants 2.33 -- env PANTS_BUILDROOT_OVERRIDE="$PWD" pants --no-pantsd help`, contains: `usage`},
	}},
	{"buck2", []docCommand{
		{command: `dogu --buck2 2026-10 -- buck2 --help`, contains: `usage`},
	}},
	{"node", []docCommand{
		{command: `dogu --node 22 -- node --help`, contains: `usage`},
		{command: `dogu --node 22 -- node -e 'console.log("hello")'`, contains: `hello`},
	}},
	{"npm", []docCommand{
		{command: `dogu --node 22 --npm 10 -- npm help`, contains: `usage`},
	}},
	{"pnpm", []docCommand{
		{command: `dogu --node 22 --pnpm 8.15 -- pnpm --help`, contains: `usage`},
	}},
	{"yarn", []docCommand{
		{command: `dogu --node 22 --yarn 1.22 -- yarn --help`, contains: `usage`},
	}},
	{"bun", []docCommand{
		{command: `dogu --bun 1 -- bun --help`, contains: `usage`},
		{command: `dogu --bun 1 -- bun -e 'console.log("hello")'`, contains: `hello`},
	}},
	{"python", []docCommand{
		{command: `dogu --python 3.12 -- python --help`, contains: `usage`},
		{command: `dogu --python 3.12 -- python -c 'print("hello")'`, contains: `hello`},
	}},
	{"uv", []docCommand{
		{command: `dogu --uv 0.5 -- uv --help`, contains: `usage`},
	}},
	{"go", []docCommand{
		{command: `dogu --go 1.24 -- go help`, contains: `usage`},
	}},
	{"rust", []docCommand{
		{command: `dogu --rust 1.85 -- rustc --help`, contains: `usage`},
	}},
	{"ghc", []docCommand{
		{command: `dogu --ghc 9.6 -- ghc --help`, contains: `usage`},
		{command: `dogu --ghc 9.6 -- ghc -e 'putStrLn "hello"'`, contains: `hello`},
	}},
	{"r", []docCommand{
		{command: `dogu --r 4.4 -- R --help`, contains: `usage`},
		{command: `dogu --r 4.4 -- R --vanilla --slave -e 'cat("hello\n")'`, contains: `hello`},
	}},
	{"julia", []docCommand{
		{command: `dogu --julia 1.11 -- julia --help`, contains: `usage`},
		{command: `dogu --julia 1.11 -- julia -e 'println("hello")'`, contains: `hello`},
	}},
	{"elm", []docCommand{
		{command: `dogu --elm 0.19.1 -- elm --help`, contains: `elm`},
	}},
	{"ruby", []docCommand{
		{command: `dogu --ruby 3.4 -- ruby --help`, contains: `usage`},
		{command: `dogu --ruby 3.4 -- ruby -e 'puts "hello"'`, contains: `hello`},
	}},
	{"lua", []docCommand{
		{command: `dogu --lua 5.4 -- lua -v`, contains: `Lua 5.4`},
		{command: `dogu --lua 5.4 -- lua -e 'print("hello")'`, contains: `hello`},
	}},
	{"asciitosvg", []docCommand{
		{command: `dogu --go 1.24 -- dogu --prerelease --asciitosvg 0 -- a2s -h`, contains: `usage`},
	}},
	{"barby", []docCommand{
		{command: `dogu --ruby 3.4 -- dogu --barby 0.6 -- ruby -rbarby -e 'puts Barby::VERSION::STRING'`, contains: `0.6`},
		{command: `dogu --ruby 3.4 -- dogu --barby 0.6 -- ruby -rbarby -rbarby/barcode/code_128 -rbarby/outputter/ascii_outputter -e 'puts Barby::Code128B.new("hello").to_ascii'`, contains: `##`},
	}},
	{"rqrcode", []docCommand{
		{command: `dogu --ruby 3.4 -- dogu --rqrcode 3 -- ruby -rrqrcode -e 'puts RQRCode::VERSION'`, contains: `3.`},
		{command: `dogu --ruby 3.4 -- dogu --rqrcode 3 -- ruby -rrqrcode -e 'puts RQRCode::QRCode.new("hello").as_svg'`, contains: `<svg`},
	}},
	{"bytefield-svg", []docCommand{
		{command: `dogu --node 22 -- dogu --bytefield-svg 1 -- bytefield-svg --help`, contains: `usage`},
	}},
	{"d2", []docCommand{
		{command: `dogu --d2 0.7 -- d2 --help`, contains: `usage`},
	}},
	{"dbml-renderer", []docCommand{
		{command: `dogu --node 22 -- dogu --dbml-renderer 1 -- dbml-renderer --help`, contains: `usage`},
	}},
	{"ditaa", []docCommand{
		{command: `dogu --java 21 --ditaa 0.11 -- ditaa --help`, contains: `usage`},
	}},
	{"erd", []docCommand{
		{command: `dogu --ghc 9.0 -- dogu --erd 0.2 -- erd --help`, contains: `Usage: erd [flags]`, exitCode: 1},
	}},
	{"gnuplot", []docCommand{
		{command: `dogu --gnuplot 6 -- gnuplot --help`, contains: `gnuplot`},
		{command: `dogu --gnuplot 6 -- gnuplot -e 'set terminal dumb; plot sin(x)'`, contains: `+---`},
	}},
	{"graphviz", []docCommand{
		{command: `dogu --graphviz 16 -- dot -?`, contains: `usage`},
	}},
	{"lilypond", []docCommand{
		{command: `dogu --lilypond 2.26 -- lilypond --help`, contains: `usage`},
	}},
	{"mermaid-cli", []docCommand{
		{command: `dogu --node 22 -- dogu --mermaid-cli 11 -- mmdc --help`, contains: `usage`},
	}},
	{"penrose", []docCommand{
		{command: `dogu --node 22 -- dogu --penrose 3.3 -- roger --help`, contains: `roger <command>`},
	}},
	{"plantuml", []docCommand{
		{command: `dogu --java 21 --plantuml 1.2025 -- plantuml -help`, contains: `usage`},
	}},
	{"jsyntrax", []docCommand{
		{command: `dogu --java 21 --jsyntrax 1 -- syntrax --help`, contains: `usage`},
	}},
	{"vega-lite", []docCommand{
		{command: `dogu --node 22 -- dogu --vega-lite 5 -- vl2svg --help`, contains: `usage`},
	}},
	{"vega", []docCommand{
		{command: `dogu --node 22 -- dogu --vega 5 -- vg2svg --help`, contains: `usage`},
	}},
}

func renderToolExample(example toolDocExample) string {
	var b strings.Builder
	b.WriteString("// Generated by TestToolExampleDocs; do not edit.\n\n[source,bash]\n----\n")
	for _, command := range example.commands {
		b.WriteString(command.command + "\n")
	}
	b.WriteString("----\n")
	return b.String()
}

// Generate build artifacts from the same commands exercised by live tests.
// Run with -count=1 when building docs to bypass Go's test-result cache.
func TestToolExampleDocs(t *testing.T) {
	root := findRepoRoot(t)
	seen := map[string]bool{}
	sections, err := filepath.Glob(filepath.Join(root, "docs", "tools", "*.adoc"))
	if err != nil {
		t.Fatal(err)
	}
	var includes strings.Builder
	for _, section := range sections {
		data, err := os.ReadFile(section)
		if err != nil {
			t.Fatal(err)
		}
		includes.Write(data)
	}
	for _, example := range toolDocExamples {
		if seen[example.tool] {
			t.Fatalf("duplicate tool %s", example.tool)
		}
		seen[example.tool] = true
		include := "include::../../build/docs-examples/" + example.tool + ".adoc[]"
		if strings.Count(includes.String(), include) != 1 {
			t.Errorf("expected exactly one %s in the tool reference", include)
		}
		if len(example.commands) == 0 {
			t.Errorf("no commands for %s", example.tool)
		}
		for _, command := range example.commands {
			if !strings.HasPrefix(command.command, "dogu ") || !strings.Contains(command.command, "--"+example.tool+" ") || command.contains == "" {
				t.Errorf("invalid example for %s: %+v", example.tool, command)
			}
		}
		path := filepath.Join(root, "build", "docs-examples", example.tool+".adoc")
		want := renderToolExample(example)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(want), 0644); err != nil {
			t.Fatal(err)
		}
	}
	fragments, err := filepath.Glob(filepath.Join(root, "build", "docs-examples", "*.adoc"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range fragments {
		tool := strings.TrimSuffix(filepath.Base(fragment), ".adoc")
		if !seen[tool] {
			if err := os.Remove(fragment); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, tool := range tools.List() {
		if !seen[tool] {
			t.Errorf("missing documentation example for %s", tool)
		}
		delete(seen, tool)
	}
	for tool := range seen {
		t.Errorf("example for unknown tool %s", tool)
	}
}

func TestToolExampleIntegrations(t *testing.T) {
	if os.Getenv("DOGU_INTEGRATION") != "1" {
		t.Skip("set DOGU_INTEGRATION=1 to run documented commands against live installations")
	}
	if runtime.GOOS == "windows" {
		t.Skip("documentation examples use a POSIX shell")
	}
	root := findRepoRoot(t)
	cachewURL := getOrStartCachew(t, root)
	binDir := t.TempDir()
	build := exec.Command("go", "build", "-o", filepath.Join(binDir, "dogu"), "github.com/dogu-dev/dogu/testdata/cachew-dogu")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	// Reuse expensive runtime/source builds across tools; each example still
	// runs in its own empty workspace without project configuration or inputs.
	home := toolExampleHome(t)
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.BaseURLs["ant"] = "https://dlcdn.apache.org/ant/binaries"
	cfg.BaseURLs["maven"] = "https://dlcdn.apache.org/maven/maven-3"
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "config.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	for _, example := range toolDocExamples {
		t.Run(example.tool, func(t *testing.T) {
			work := t.TempDir()
			for i, command := range example.commands {
				t.Run(fmt.Sprintf("command-%d", i+1), func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
					defer cancel()
					cmd := exec.CommandContext(ctx, "sh", "-c", command.command)
					configureExampleProcess(cmd)
					cmd.Dir = work
					cmd.Env = append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"), "DOGU_HOME="+home, "DOGU_CONFIGURATION=", "CACHEW_URL="+cachewURL)
					// Keep source builds practical without overriding an explicit setting.
					if os.Getenv("MAKEFLAGS") == "" {
						cmd.Env = append(cmd.Env, "MAKEFLAGS=-j2")
					}
					out, err := cmd.CombinedOutput()
					exitCode := 0
					if err != nil {
						var exitErr *exec.ExitError
						if !errors.As(err, &exitErr) {
							t.Fatalf("%s: %v\n%s", command.command, err, out)
						}
						exitCode = exitErr.ExitCode()
					}
					if ctx.Err() != nil || exitCode != command.exitCode {
						t.Fatalf("%s: exit %d, want %d (context: %v)\n%s", command.command, exitCode, command.exitCode, ctx.Err(), out)
					}
					if !strings.Contains(strings.ToLower(string(out)), strings.ToLower(command.contains)) {
						t.Errorf("%s: expected %q in output:\n%s", command.command, command.contains, out)
					}
				})
			}
		})
	}
}

func toolExampleHome(t *testing.T) string {
	t.Helper()
	home := os.Getenv("DOGU_EXAMPLE_HOME")
	if home == "" {
		home = t.TempDir()
	}
	home, err := filepath.Abs(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(home, 0755); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestToolExampleRelativeHome(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	t.Setenv("DOGU_EXAMPLE_HOME", filepath.Join("build", "example-home"))
	home := toolExampleHome(t)
	if !filepath.IsAbs(home) {
		t.Fatalf("relative example home: %s", home)
	}
	configFile := filepath.Join(home, "config.json")
	if err := os.WriteFile(configFile, []byte("retained"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	data, err := os.ReadFile(filepath.Join(home, "config.json"))
	if err != nil || string(data) != "retained" {
		t.Fatalf("home did not survive workspace change: %q, %v", data, err)
	}
}
