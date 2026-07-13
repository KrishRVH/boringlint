package integration

import "iter"

func values(yield func(int) bool) {
	yield(42)
}

func sequence() iter.Seq[int] {
	return values
}

func use() {
	for value := range sequence() {
		_ = value
	}
}
