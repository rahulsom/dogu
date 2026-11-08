# Tool providers

`catalog.go` contains typed definitions for Ant, Maven, Clojure CLI, Leiningen,
Gradle, SBT, Yarn, Bazel, Buck2, Mill, Groovy, Kotlin, Scala, and Elm.
`definition.go` implements their common workflow:
reuse an installed candidate, resolve a version, expand release naming rules,
and call the archive or single-binary installer in `common.go`.
The exported provider types remain small adapters so callers can still use
zero values such as `&MavenTool{}`.

A definition contains:

- A download base URL and version source (Apache indexes, Gradle, GitHub, or npm).
- Asset and relative download-path templates.
- Optional OS/architecture names and Windows extension/suffix overrides.
- A binary name for single-binary installs; an empty name selects archives.
- Optional cache naming, home environment keys, and extra environment values.

Templates support `{version}`, `{major}`, `{os}`, `{arch}`, `{ext}`, `{suffix}`, and
`{binary}`. Download paths additionally support `{asset}`. Extra environment
values use the same substitution rules. Versions are cleaned after selection,
with the requested version as fallback when discovery fails or finds no match.
Existing mirror and download-rewrite behavior is retained.
An empty version spec matches all releases and candidates, excluding prereleases
by default. Discovery must succeed for empty specs; there is no explicit version
to fall back to. Never treat the tool's candidates root as an exact empty match.
Shared installers scope cached downloads by candidate version so releases with
identical asset filenames cannot reuse another version's payload.
They extract or copy into a temporary directory before replacing the candidate.
Setup runs with an incomplete marker; only successful setup clears it and writes
a completion marker. Candidate searches reject marked incomplete installations,
while unmarked legacy installations remain reusable. Failed extraction removes
its cached archive so the next attempt can download it again. Forced installs
replace candidate contents even when the resolved version is unchanged.

Fresh and reused installs apply the same home environment metadata. Reused
installs derive the normalized version and major-version environment keys from
the selected candidate directory rather than the requested version range.

Providers with custom discovery or setup remain ordinary Go implementations:
Java, Node, Python, Pants, Bun, UV, npm, pnpm, Clojure CLI, and Leiningen.
Extend the catalog when a provider fits its existing rules; keep specialized
behavior in Go rather than adding a configuration language for it.
Python resolves interpreter versions and host assets from standalone build
release metadata, not the date-based release tags. Scala's empty spec considers
both Scala 2 and Scala 3 releases.

Provider implementations are grouped in `jvm.go`, `js.go`, `python.go`,
`lang.go`, and `diagram.go`. Generic build tools live in `lang.go`; shared
catalog, installation, version, and registry logic stays in this package.

When adding or migrating a definition, cover its release naming on the relevant
platforms and use local fixtures to verify version resolution, extracted or
binary payloads, cached installation metadata, and forced reinstallation.

Go and Julia select host archives from their official JSON release indexes in
`lang.go`. Rust standalone installers and GHC bindists run their setup
commands after extraction. R is built from CRAN sources. These three providers
support Linux and macOS and require the prerequisites documented in
`docs/tools.adoc`; failed setup removes the candidate so it cannot be reused.
Elm's gzip-compressed executable uses the single-binary installer.

Ruby and Lua also use the configured-language workflow in `lang.go`.
Their official source indexes drive both installation and version listing.
Ruby runs configure and make; Lua uses the Linux or macOS make target and
installs with `INSTALL_TOP` pointing to its candidate directory.

`diagram.go` registers the diagramming category. Providers use private npm,
RubyGems, Go, or Cabal prefixes, official binary/jar distributions, browser
source archives, or configured source builds. A completion marker distinguishes
successful setup from partial candidates; failures remove the candidate.
Package-manager prerequisites must already be on the installer's PATH because
CLI provisioning is concurrent. Barby and RQRCode expose GEM_PATH, npm tools
expose NODE_PATH and local executables, and JS Sequence Diagrams supplies browser
assets without inventing a CLI. See `docs/tools.adoc` for commands and platform
requirements. `diagrams_test.go` uses in-memory release responses and fake package
managers to check discovery, resolved versions, reuse, jars, source builds, and
failure cleanup without installing third-party tools.
