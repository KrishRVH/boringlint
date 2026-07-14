package iteratorboundary

import (
	"iteratorprovider"
	"slices"
)

type SequenceAlias = iteratorprovider.Sequence[int] // want `contains an iterator-shaped term`

type EmbeddedSequence interface {
	iteratorprovider.Sequence[int] // want `contains an iterator-shaped term`
}

type HeterogeneousSequenceAlias = iteratorprovider.HeterogeneousSequence // want `contains an iterator-shaped term`

type MixedSequenceAlias = iteratorprovider.SequenceOrSlice // want `contains an iterator-shaped term`

type TransitiveSequenceAlias = iteratorprovider.TransitiveSequence // want `contains an iterator-shaped term`

type EmptySequenceAlias = iteratorprovider.EmptySequence // want `contains an iterator-shaped term`

type GenericSequenceHolder[T iteratorprovider.HeterogeneousSequence] struct { // want `contains an iterator-shaped term`
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
