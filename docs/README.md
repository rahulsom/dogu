# Maintaining the tool reference

Runnable examples live in `pkg/cli/doc_examples_test.go`. Each entry contains
an exact POSIX shell command and an expected output substring. Start with help
(or a version/load check for tools without help), and add a self-contained
execution example when the tool can evaluate an expression directly. Include
runtime flags and nest dogu when a runtime is required during installation.
Do not use project files that the reader has not created.

Generate the AsciiDoc fragments into `build/docs-examples/` before rendering
the reference (no downloads required):

```sh
go test ./pkg/cli -run '^TestToolExampleDocs$' -count=1
```

Include `../../build/docs-examples/<tool>.adoc[]` in the corresponding
section under `docs/tools/`. Ordinary tests generate fragments and check
registry coverage and includes. Use `-count=1` to regenerate even when Go
has cached a previous test result. The docs publishing job runs this step
before Asciidoctor. Generation does not certify that a command works; run the live tests:

```sh
./scripts/test-with-cachew.sh go test ./pkg/cli -run '^TestToolExampleIntegrations$' -timeout 90m -v
```

Use Go's subtest filtering to validate one tool, for example
`-run '^TestToolExampleIntegrations$/^rqrcode$'`. These tests execute the exact
shell commands shown in the docs against real installations, assert the expected
exit code and output, and share installations within the suite. sbt and erd return
status 1 for help; their cases explicitly record that upstream behavior.
Each tool gets an empty workspace. No fake tool executables are used.

CI runs the live documentation examples in four parallel shards. To reproduce
one shard locally, set `DOGU_EXAMPLE_SHARD` to `erd`, `r`, `ruby` (which also
runs the Barby and RQRCode examples), or `remaining`. Leave it unset to run
all examples. CI also runs the live tool and mirror tests in the remaining job.

Live tests require Docker (or `CACHEW_URL`), upstream access, and the build
prerequisites listed in the tool reference. In particular, install Cabal and
run `cabal update` before testing erd; C/C++ and Fortran compilers, make,
pkg-config, and development libraries are needed for source builds and native
npm dependencies. These tests can take substantially longer than unit tests. Set
`DOGU_EXAMPLE_HOME` to a dedicated scratch directory to retain installations
between runs; the suite writes its test configuration there. Set
`CACHEW_GITHUB_TOKEN` when the public GitHub API quota is insufficient; CI uses
its scoped workflow token. The test transport sends it only to the GitHub API,
not to release asset hosts, and only through a localhost or loopback Cachew
server. Remote mirrors do not receive it.
