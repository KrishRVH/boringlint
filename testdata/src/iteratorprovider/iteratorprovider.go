package iteratorprovider

import "iter"

func Values() iter.Seq[int] {
	return func(yield func(int) bool) {
		yield(42)
	}
}
