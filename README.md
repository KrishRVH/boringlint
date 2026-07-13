# boringlint

`boringlint` enforces a deliberately restricted Go dialect: direct data and
control flow over language machinery that hides either one.

This is opinionated policy, not a claim that the rejected features are invalid
Go. The module is pre-v1 while the rules and public analyzer API settle.

## Install and run

Install the command with the same or a newer Go toolchain than the code it will
analyze:

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

The module requires Go 1.25 or newer. `nogenericmethod` analyzes Go 1.27 syntax,
so its binary must be built with Go 1.27 or newer. Until Go 1.27 is stable, use
the current release candidate when testing that rule:

```sh
GOTOOLCHAIN=go1.27rc2 go install github.com/KrishRVH/boringlint/cmd/boringlint@latest
```

## Rules

### `noiterator`

Rejects:

- direct imports of `iter`;
- Go 1.23 range-over-function;
- iterator-shaped project type declarations; and
- iterator-shaped project function and method declarations.

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

The rule intentionally governs imports, range statements, and type/function
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
