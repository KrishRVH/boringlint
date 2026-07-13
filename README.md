# boringlint

`boringlint` enforces a deliberately restricted Go dialect: direct data and
control flow over language machinery that hides either one.

This is opinionated policy, not a claim that the rejected features are invalid
Go. The module is pre-v1 while the rules and public analyzer API settle.

## Install and run

After publishing a tagged release, install the command with the same or a newer
Go toolchain than the code it will analyze:

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
  including constraints, fields, parameters, and results.

```go
import "iter" // rejected

type Sequence func(func(int) bool) // rejected

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

The rule intentionally governs imports, range statements, and types in project
declarations. It does not attempt an expression-level ban on every value whose
type happens to be iterator-shaped.

### `nogenericmethod`

Rejects method-local type parameters introduced in Go 1.27, in both method
declarations and uses. Generic functions and methods that only use their
receiver type's parameters remain allowed.

```go
func (box Box[T]) Map[U any](convert func(T) U) U { // rejected
	return convert(box.value)
}

func Map[T, U any](box Box[T], convert func(T) U) U { // allowed
	return convert(box.value)
}
```

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

## License

MIT
