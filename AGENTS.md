# Repository Guidelines

## Project Structure & Module Organization

dogu is a multiplatform developer tool manager written in Go. `cmd/dogu/main.go` is the CLI entry point. Packages under `pkg/` handle CLI/TUI behavior, configuration, downloads, environment construction, self-updates, tool providers, and version metadata. `internal/testsupport/` supplies shared test helpers; `testdata/` contains fixtures and Cachew configuration. Tests live beside source files. User documentation is AsciiDoc in `docs/`, with assets in `images/`.

Tool providers are grouped by ecosystem in `pkg/tools/`. Extend `catalog.go` and the shared definition workflow when existing rules fit; keep specialized installation logic in Go. Read `pkg/tools/README.md` before changing providers.

## Build, Test, and Development Commands

Use Go 1.25 as declared in `go.mod`; the README's older minimum is outdated.

- `go build ./...`: compile all packages.
- `go build -o build/bin/dogu ./cmd/dogu`: build a local CLI binary.
- `go run ./cmd/dogu --help`: inspect CLI usage during development.
- `./build.sh`: cross-compile Linux, macOS, and Windows binaries into `build/bin/`.
- `go test -race -cover ./...`: run the local suite with race detection and coverage.
- `./scripts/test-with-cachew.sh`: run live integration tests through a Docker Cachew mirror; cached downloads persist in `build/cachew/`.

## Coding Style & Naming Conventions

Format Go changes with `gofmt`, using tabs and standard Go import grouping. Use lowercase package names, exported `MixedCaps` identifiers, and descriptive filenames consistent with neighboring code. Document exported APIs with Go comments. No separate lint configuration is checked in.

## Testing Guidelines

Use Go's `testing` package, `*_test.go` files, and `TestXxx` functions. Prefer table-driven subtests, local HTTP fixtures, and fake package managers for deterministic coverage. Provider changes should cover platform asset naming, version resolution, fresh installation, reuse, forced reinstallation, and failure cleanup. Live tests require Docker and upstream network access. CI runs race detection and coverage through Cachew; no numeric coverage threshold is configured.

## Commit & Pull Request Guidelines

Recent commits use concise imperative subjects, such as `Add Ruby and Lua language support`; Conventional Commit prefixes are occasional, not mandatory. Keep changes focused. PRs should explain the behavior change, list validation commands and results, link relevant issues, and update `docs/` for user-visible changes. Include terminal output or screenshots when CLI/TUI presentation changes.
