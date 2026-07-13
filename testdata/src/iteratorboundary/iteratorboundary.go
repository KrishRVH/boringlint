package iteratorboundary

import (
	"iteratorprovider"
	"slices"
)

func collectDependencyIterator() []int {
	return slices.Collect(iteratorprovider.Values())
}
