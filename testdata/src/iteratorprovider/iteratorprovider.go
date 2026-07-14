package iteratorprovider

import "iter"

type Sequence[T any] interface {
	~func(func(T) bool)
}

type HeterogeneousSequence interface {
	~func(func(int) bool) | ~func(func(string) bool)
}

type SequenceOrSlice interface {
	~func(func(int) bool) | ~[]int
}

type TransitiveSequence interface {
	SequenceOrSlice
	~[]int
}

type EmptySequence interface {
	SequenceOrSlice
	~string
}

type Predicate interface {
	~func(int) bool
}

type Methods interface {
	Iterate(func(int) bool)
}

func Values() iter.Seq[int] {
	return func(yield func(int) bool) {
		yield(42)
	}
}
