// Command boringlint is a vet tool for a deliberately restricted Go dialect.
package main

import (
	"golang.org/x/tools/go/analysis/unitchecker"

	"github.com/KrishRVH/boringlint"
)

func main() {
	unitchecker.Main(
		boringlint.NoIterator,
		boringlint.NoGenericMethod,
	)
}
