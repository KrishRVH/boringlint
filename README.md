# boringlint

[![Go Reference](https://pkg.go.dev/badge/github.com/KrishRVH/boringlint.svg)](https://pkg.go.dev/github.com/KrishRVH/boringlint)

`boringlint` enforces a deliberately restricted Go dialect: direct data and
control flow over language machinery that hides either one.

Its policy is inspired by ThePrimeagen's
[*I am done with Golang*](https://youtu.be/WqSWZuGS9pc): keep Go procedural,
make control flow and costs visible, and keep codebases familiar.

This is opinionated policy, not a claim that the rejected features are invalid
Go. v0.9.0 is the release candidate for the v1 rule scope and public analyzer
API. v1.0 will follow external review and a passing compatibility gate on the
stable Go 1.27 toolchain.

## Install and run

Install the command with Go 1.25 or newer, using a toolchain at least as new as
the code it will analyze:

```sh
go install github.com/KrishRVH/boringlint/cmd/boringlint@latest
boringlint ./...
```

The command also implements the `go vet` tool protocol:

```sh
go vet ./...
go vet -vettool="$(command -v boringlint)" ./...
```

Run both commands. `-vettool` selects `boringlint` in place of the standard vet
analyzers; it does not add checks to the standard set.

## Suppression

Per-site suppression is deliberately absent: `//nolint` is a golangci-lint
convention ignored by standalone and vettool modes, and a policy linter with
site-level escape hatches stops being policy. To run only one analyzer, select
it for the invocation:

```sh
boringlint -noiterator ./...
go vet -vettool="$(command -v boringlint)" -noiterator ./...
```

Explicit package patterns restrict analysis in either mode. A custom
[golangci-lint v2 module-plugin adapter](https://golangci-lint.run/docs/plugins/module-plugins/)
can register the exported analyzers by rule name, after which golangci-lint
honors `//nolint:noiterator`; `boringlint` does not ship that adapter.

## Go version support

`boringlint`'s source-project floor is Go 1.23, which introduced
[range-over-function and `iter`](https://go.dev/doc/go1.23). New Go releases
are supported only after both command modes pass the compatibility gate; the
current upper bound is Go 1.27, tested with the Go 1.27rc2 toolchain. Building,
installing, or running `boringlint` requires Go 1.25 or newer. Importing its
analyzers into a custom driver has the same floor because a module's
[`go` directive is a mandatory minimum](https://go.dev/ref/mod#go-mod-file-go).
A standalone or vettool binary may analyze projects declaring Go 1.23 or 1.24,
provided the binary was built with a supported toolchain at least as new as the
source it analyzes.

`nogenericmethod` analyzes Go 1.27 syntax, so its binary must be built with Go
1.27 or newer. The repository currently tests that rule with Go 1.27rc2:

```sh
GOTOOLCHAIN=go1.27rc2 go install github.com/KrishRVH/boringlint/cmd/boringlint@latest
```

Go 1.27 is still a release candidate. v0.9.0 validates its current syntax and
tooling behavior, but v1.0 will not claim stable Go 1.27 support until the final
toolchain passes the same compatibility gate.

The Go 1.25 tool floor is deliberate. Analysis drivers consume
[compiler export data](https://github.com/golang/tools/blob/v0.48.0/go/gcexportdata/gcexportdata.go#L5-L23)
and implement the [`go vet -vettool` protocol](https://github.com/golang/tools/blob/v0.48.0/go/analysis/unitchecker/unitchecker.go#L82-L96),
so ordinary Go source compatibility does not make an old driver
forward-compatible. [`x/tools` v0.36.0](https://github.com/golang/tools/blob/v0.36.0/go.mod)
was the last release with a Go 1.23 floor before
[v0.37.0 raised it](https://github.com/golang/tools/blob/v0.37.0/go.mod), but Go
[subrepositories generally support only the previous two Go releases and tip](https://go.dev/wiki/X-Repositories).
Freezing the driver there would trade a lower build floor for unsupported
compiler integration. The compatible
[`x/tools` v0.48.0](https://github.com/golang/tools/blob/v0.48.0/go.mod) requires
Go 1.25.

The floor will rise only when correct support for a newly supported Go release
requires a newer analysis driver. Each increase must be explicit in `go.mod`
and this section, and both standalone and vettool compatibility must pass
before support for that source version is claimed.

## Rules

### `noiterator`

Rejects:

- direct imports of `iter`;
- Go 1.23 range-over-function;
- iterator-shaped types in project type, function, and method declarations,
  including constraints, fields, parameters, and results, even when the
  admitted iterator types have different valid yield signatures;
- project type declarations that name constraints containing iterator-shaped
  terms, including mixed unions and intersections that eliminate those terms.

Each independently named hidden constraint term is reported at that term. A
whole type-parameter diagnostic does not replace diagnostics for distinct
named terms inside its constraint.

```go
import "iter" // rejected

type Sequence func(func(int) bool) // rejected

type MaybeSequence = dependency.SequenceOrSlice // rejected

func Values(yield func(int) bool) { // rejected
	// ...
}

for value := range dependency.Values() { // rejected
	// ...
}
```

Range over arrays, slices, strings, maps, channels, and integers remains
allowed. Values returned by dependencies are also allowed so they can be
materialized without naming the iterator type:

```go
values := slices.Collect(dependency.Values())
```

The rule intentionally governs imports, range statements, and iterator-shaped
types in project type, function, and method declarations. Iterator-shaped types
in variable declarations and function-literal signatures remain allowed; the
rule does not attempt an expression-level ban on every value whose type happens
to be iterator-shaped.

#### Why

The [`iter.Seq` protocol](https://pkg.go.dev/iter#Seq) is push-based: the
producer advances the sequence by invoking a caller-provided `yield` function.
Range-over-function presents that callback as ordinary loop syntax;
[`iter.Pull`](https://pkg.go.dev/iter#Pull) recovers caller-driven advancement
with `next`, but callers that abandon the sequence must invoke `stop`.
Materializing dependency iterators at the boundary keeps iterator-shaped types
out of project type, function, and method declarations and restores ordinary
range over concrete collections.

### `nogenericmethod`

Rejects method-local type parameters introduced in Go 1.27, in both method
declarations and uses. Generic functions remain allowed, as do methods that use
only their receiver type's parameters.

```go
func (box Box[T]) Map[U any](convert func(T) U) U { // rejected
	return convert(box.value)
}

func Map[T, U any](box Box[T], convert func(T) U) U { // allowed
	return convert(box.value)
}
```

#### Why

[Go 1.27](https://go.dev/doc/go1.27#language) permits neither type parameters on
interface methods nor generic methods as interface-method implementations. On
a generic receiver, method-local type parameters layer method instantiation on
top of receiver instantiation. A package-level generic function makes the
receiver an ordinary argument and keeps generic operations in a single
package-level namespace.

## Use as a package

The analyzers are ordinary `go/analysis` values and can be composed into a
custom driver:

```go
package main

import (
	"github.com/KrishRVH/boringlint"
	"golang.org/x/tools/go/analysis/multichecker"
)

func main() {
	multichecker.Main(boringlint.NoIterator, boringlint.NoGenericMethod)
}
```

## Development

[mise](https://mise.jdx.dev/) is the developer interface:

```sh
mise run tasks
mise run standards
mise run standards:check
```

The full gate checks formatting, module integrity, static analysis,
vulnerabilities, secrets, race behavior, command integration, Go 1.25
compatibility, and Go 1.27 behavior.

### Release

Release only an exact commit that has passed both the local and remote gates:

1. Review the pinned mise tools and CI mise runtime. If a tool pin changes, run
   `mise run lock`; never edit the lockfile by hand.
2. Run `mise run standards:check`, review the complete diff, and commit with a
   Conventional Commit.
3. Confirm `git status --short` is empty, push the commit, and wait for CI to
   pass on that exact SHA.
4. Create the annotated tag from the exact SHA that passed CI with
   `git tag -a -m vX.Y.Z vX.Y.Z <verified-SHA>`, then run
   `git push origin vX.Y.Z`.
5. Publish the GitHub Release from that tag, for example with
   `gh release create vX.Y.Z --verify-tag --generate-notes`.

Do not publish the tag or release while its commit's CI run is still pending.

## License

MIT
