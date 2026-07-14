package iteratorprovider

import "iter"

type Sequence[T any] interface {
	~func(func(T) bool)
}

func Values() iter.Seq[int] {
	return func(yield func(int) bool) {
		yield(42)
	}
}
