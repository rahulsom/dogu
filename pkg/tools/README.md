# Tool providers

`catalog.go` contains typed definitions for Ant, Maven, Gradle, SBT, Yarn,
Bazel, Buck2, Mill, Groovy, Kotlin, Scala, and Elm. `definition.go` implements their common workflow:
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

Templates support `{version}`, `{os}`, `{arch}`, `{ext}`, `{suffix}`, and
`{binary}`. Download paths additionally support `{asset}`. Extra environment
values use the same substitution rules. Versions are cleaned after selection,
with the requested version as fallback when discovery fails or finds no match.
Existing mirror and download-rewrite behavior is retained.

Fresh and reused installs apply the same home environment metadata. Reused
installs derive the normalized version and major-version environment keys from
the selected candidate directory rather than the requested version range.

Providers with custom discovery or setup remain ordinary Go implementations:
Java, Node, Python, Pants, Bun, UV, npm, and pnpm. Extend the catalog when a
provider fits its existing rules; keep specialized behavior in Go rather than
adding a configuration language for it.

When adding or migrating a definition, cover its release naming on the relevant
platforms and use local fixtures to verify version resolution, extracted or
binary payloads, cached installation metadata, and forced reinstallation.

Go and Julia select host archives from their official JSON release indexes in
`languages.go`. Rust standalone installers and GHC bindists run their setup
commands after extraction. R is built from CRAN sources. These three providers
support Linux and macOS and require the prerequisites documented in
`docs/tools.adoc`; failed setup removes the candidate so it cannot be reused.
Elm's gzip-compressed executable uses the single-binary installer.
