package iteratorsignature

type Sequence func(func(int) bool) // want `iterator-shaped type`

type SequenceAlias = func(func(string) bool) // want `iterator-shaped type`

type GenericSequence[T any] func(func(T) bool) // want `iterator-shaped type`

type SequenceConstraint[T any] interface {
	~func(func(T) bool) // want `iterator-shaped type`
}

type YieldFunc[T any] interface {
	~func(T) bool
}

type Holder struct {
	Values Sequence // want `iterator-shaped type`
}

type Source interface {
	Values() Sequence // want `iterator-shaped type`
}

func Values() Sequence { // want `iterator-shaped type`
	return nil
}

func Consume(sequence Sequence) { // want `iterator-shaped type`
	_ = sequence
}

func Yield(yield func(int) bool) { // want `iterator-shaped type`
	_ = yield
}

func GenericYield[Y YieldFunc[int]](yield Y) { // want `iterator-shaped type`
	_ = yield
}

type Predicate func(int) bool

type Handler func(func(int) bool) error

func Apply(predicate func(int) bool) bool {
	return predicate(1)
}
