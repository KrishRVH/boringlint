package iteratorboundary

import (
	"iteratorprovider"
	"slices"
)

type SequenceAlias = iteratorprovider.Sequence[int] // want `iterator-shaped type`

type EmbeddedSequence interface {
	iteratorprovider.Sequence[int] // want `iterator-shaped type`
}

type HeterogeneousSequenceAlias = iteratorprovider.HeterogeneousSequence // want `iterator-shaped type`

type MixedSequenceAlias = iteratorprovider.SequenceOrSlice // want `iterator-shaped type`

type TransitiveSequenceAlias = iteratorprovider.TransitiveSequence // want `iterator-shaped type`

type EmptySequenceAlias = iteratorprovider.EmptySequence // want `iterator-shaped type`

type GenericSequenceHolder[T iteratorprovider.HeterogeneousSequence] struct { // want `iterator-shaped type`
	Value T
}

type PredicateAlias = iteratorprovider.Predicate

type MethodsAlias = iteratorprovider.Methods

func collectDependencyIterator() []int {
	return slices.Collect(iteratorprovider.Values())
}

func rangeDependencyIterator() {
	for range iteratorprovider.Values() { // want `range over a function value`
	}
}

func declareDependencyConstraint[S iteratorprovider.Sequence[int]]() { // want `iterator-shaped type`
}
