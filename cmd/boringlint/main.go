// Command boringlint is a vet tool for a deliberately restricted Go dialect.
package main

import (
	"golang.org/x/tools/go/analysis/multichecker"

	"github.com/KrishRVH/boringlint"
)

func main() {
	multichecker.Main(
		boringlint.NoIterator,
		boringlint.NoGenericMethod,
	)
}
