# Agent Guide

Read `README.md` before changing behavior or public terminology.

## Design target

`boringlint` is a small, opinionated `go/analysis` package and command. Prefer
direct AST/type analysis, precise diagnostics, and a tiny public API. Do not add
configuration, compatibility fallbacks, dependencies, or abstractions without
a demonstrated need.

The analyzers must remain deterministic, safe for concurrent package analysis,
and usable both as a standalone `multichecker` command and through
`go vet -vettool`.

## Commands

Repository development goes through mise:

- `mise run tasks`: list tasks.
- `mise run standards`: format source and tidy the module.
- `mise run test`: run the primary tests.
- `mise run test:compat`: test Go 1.25 compatibility and Go 1.27 behavior,
  including `nogenericmethod` diagnostics.
- `mise run standards:check`: run the complete release gate.

Do not invoke package managers, compilers, linters, or test runners directly
unless repairing the corresponding mise task.

## Changes

- Keep rule contracts explicit in `README.md` and analyzer metadata.
- Add an `analysistest` regression for every diagnostic behavior change.
- Keep command integration coverage for both standalone and vettool modes.
- Treat `testdata/` as behavioral fixtures, not example applications.
- Set the public minimum Go version in `go.mod`; pin the development toolchain
  separately in `.config/mise/config.toml`.
- Before each release, review the pinned mise tools and CI mise runtime.
- Do not hand-edit `.config/mise/mise.lock`; after changing mise tool pins, run
  `mise run lock`.
- Do not commit `.cache/`, coverage output, binaries, dependencies, or secrets.

Before handoff, run `mise run standards:check` and report any skipped check.
Use Conventional Commits 1.0.0 for commit messages.
