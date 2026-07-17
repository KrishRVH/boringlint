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

type MixedConstraintPair[
	T ~func(func(int) bool), // want `iterator-shaped type`
	U iteratorprovider.HeterogeneousSequence, // want `contains an iterator-shaped term`
] struct{}

type NamedSequenceHolder[T iteratorprovider.Sequence[int]] struct { // want `contains an iterator-shaped term`
	Value T
}

type NamedSequenceMarker[T iteratorprovider.Sequence[int]] struct{} // want `contains an iterator-shaped term`

type SharedNamedConstraint[
	T, U iteratorprovider.Sequence[int], // want `contains an iterator-shaped term`
] struct {
	Left  T
	Right U
}

type SeparateNamedConstraints[
	T iteratorprovider.Sequence[int], // want `contains an iterator-shaped term`
	U iteratorprovider.Sequence[int], // want `contains an iterator-shaped term`
] struct{}

type NarrowedHiddenPair[T interface {
	iteratorprovider.SequenceOrSlice // want `contains an iterator-shaped term`
	iteratorprovider.SequenceOrMap   // want `contains an iterator-shaped term`
}] struct {
	Value T
}

type VisibleAndHidden[T interface {
	~func(func(int) bool)                  // want `iterator-shaped type`
	iteratorprovider.HeterogeneousSequence // want `contains an iterator-shaped term`
}] struct {
	Value T
}

type CombinedConstraint[T interface {
	~func(func(int) bool)                  // want `iterator-shaped type`
	iteratorprovider.HeterogeneousSequence // want `contains an iterator-shaped term`
}] struct{}

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

func declareHeterogeneousDependencyConstraint[S iteratorprovider.HeterogeneousSequence]() { // want `iterator-shaped type`
}

func declareMixedDependencyConstraint[S iteratorprovider.SequenceOrSlice]() {
}

func declareEliminatedDependencyConstraint[S iteratorprovider.TransitiveSequence]() {
}

func declareEmptyDependencyConstraint[S iteratorprovider.EmptySequence]() {
}

func consumeHeterogeneousYield[Y iteratorprovider.HeterogeneousYield](sequence func(Y)) { // want `iterator-shaped type`
	_ = sequence
}

func consumeExactBool[B bool](sequence func(func(int) B)) { // want `iterator-shaped type`
	_ = sequence
}

func consumeApproximateBool[B ~bool](sequence func(func(int) B)) {
	_ = sequence
}

func declareMethodNarrowed[S iteratorprovider.MethodNarrowed]() { // want `iterator-shaped type`
}

func declareTildeMethodSequence[S iteratorprovider.MethodSequenceConstraint]() { // want `iterator-shaped type`
}

func declareEmptyMethodSequence[S iteratorprovider.EmptyMethodSequence]() {
}

func consumeEmptyBool[B iteratorprovider.EmptyBool](sequence func(func(int) B)) {
	_ = sequence
}

type HeterogeneousYieldSequence[Y iteratorprovider.HeterogeneousYield] func(Y) // want `iterator-shaped type`

type ExactBoolSequence[B bool] func(func(int) B) // want `iterator-shaped type`

type ApproximateBoolSequence[B ~bool] func(func(int) B)

type MixedYieldSequence[Y iteratorprovider.YieldOrInt] func(Y)

type HeterogeneousYieldReceiver[Y iteratorprovider.HeterogeneousYield] struct{}

func (HeterogeneousYieldReceiver[Y]) consume(sequence func(Y)) { // want `iterator-shaped type`
	_ = sequence
}

type ExactBoolReceiver[B bool] struct{}

func (ExactBoolReceiver[B]) consume(sequence func(func(int) B)) { // want `iterator-shaped type`
	_ = sequence
}

type ApproximateBoolReceiver[B ~bool] struct{}

func (ApproximateBoolReceiver[B]) consume(sequence func(func(int) B)) {
	_ = sequence
}
