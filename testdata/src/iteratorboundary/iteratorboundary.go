package iteratorboundary

import (
	"iteratorprovider"
	"slices"
)

func collectDependencyIterator() []int {
	return slices.Collect(iteratorprovider.Values())
}

func rangeDependencyIterator() {
	for range iteratorprovider.Values() { // want `range over a function value`
	}
}

func declareDependencyConstraint[S iteratorprovider.Sequence[int]]() { // want `iterator-shaped type`
}
