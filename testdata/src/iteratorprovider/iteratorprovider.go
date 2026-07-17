package iteratorprovider

import "iter"

type Sequence[T any] interface {
	~func(func(T) bool)
}

type HeterogeneousSequence interface {
	~func(func(int) bool) | ~func(func(string) bool)
}

type HeterogeneousYield interface {
	~func(int) bool | ~func(string) bool
}

type YieldOrInt interface {
	~func(int) bool | ~int
}

type methodSequence func(func(int) bool)

func (methodSequence) keep() {}

type methodSlice []int

type MethodNarrowed interface {
	methodSequence | methodSlice
	keep()
}

type MethodSequenceConstraint interface {
	~func(func(int) bool)
	keep()
}

type EmptyMethodSequence interface {
	func(func(int) bool)
	missing()
}

type EmptyBool interface {
	bool
	missing()
}

type FieldCollisionSequence func(func(int) bool)

func (FieldCollisionSequence) Keep() {}

type FieldCollisionNarrowed interface {
	~func(func(int) bool) | ~struct{ Keep bool }
	Keep()
}

type SequenceOrSlice interface {
	~func(func(int) bool) | ~[]int
}

type SequenceOrMap interface {
	~func(func(int) bool) | ~map[int]int
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
