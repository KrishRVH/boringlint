package iteratorsignature

type Sequence func(func(int) bool) // want `iterator-shaped type`

type SequenceAlias = func(func(string) bool) // want `iterator-shaped type`

type GenericSequence[T any] func(func(T) bool) // want `iterator-shaped type`

type EmptySequence func(func() bool) // want `iterator-shaped type`

type PairSequence func(func(string, int) bool) // want `iterator-shaped type`

type NamedYield func(int) bool

type NamedYieldSequence func(NamedYield) // want `iterator-shaped type`

type SequenceConstraint[T any] interface {
	~func(func(T) bool) // want `iterator-shaped type`
}

type YieldFunc[T any] interface {
	~func(T) bool
}

type ConstrainedSequence[T ~func(func(int) bool)] struct{} // want `iterator-shaped type`

type Holder struct {
	Values Sequence // want `iterator-shaped type`
}

type NestedIteratorField struct {
	Values func(func(Sequence) bool) // want `iterator-shaped type` `iterator-shaped type`
}

type NamedIterator Sequence // want `iterator-shaped type`

type InstantiatedIterator GenericSequence[int] // want `iterator-shaped type`

type NestedIteratorArgument GenericSequence[Sequence] // want `iterator-shaped type` `iterator-shaped type`

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

type BoolAlias = bool

type AliasBoolSequence func(func(int) BoolAlias) // want `iterator-shaped type`

type NamedBool bool

type NamedBoolSequence func(func(int) NamedBool)

type VariadicSequence func(func(...int) bool)

type MixedVariadicSequence func(func(int, ...string) bool)

type NoParamSequence func()

type TwoParamSequence func(func(int) bool, int)

type ThreeValueSequence func(func(int, string, bool) bool)

type NoResultYieldSequence func(func(int))

type TwoResultYieldSequence func(func(int) (bool, error))

// Iterator-shaped types in variable declarations and function literals are
// intentionally allowed.
var IteratorValue = func(yield func(int) bool) {
	_ = yield
}

var ExplicitIteratorValue func(func(int) bool)

var ConsumeLiteral = func(sequence func(func(int) bool)) {
	_ = sequence
}

func Apply(predicate func(int) bool) bool {
	return predicate(1)
}
