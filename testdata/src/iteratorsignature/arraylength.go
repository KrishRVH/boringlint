package iteratorsignature

import "unsafe"

// Array lengths are expressions, so literal signatures and variable types
// remain allowed. Nested type declarations and function ranges still count.
type LiteralLength [unsafe.Sizeof(func(value func(func(int) bool)) {
	var local func(func(int) bool)
	_ = local
	type NestedIterator func(func(int) bool) // want `iterator-shaped type`
	for range func(yield func() bool) {} {   // want `range over a function value`
	}
})]byte

type ConvertedLength [unsafe.Sizeof((func(func(int) bool))(nil))]byte

type IteratorElements [unsafe.Sizeof(func(value func(func(int) bool)) {})]func(func(int) bool) // want `iterator-shaped type`

func ArrayParameter(value [unsafe.Sizeof(func(sequence func(func(int) bool)) {})]byte) {
	_ = value
}
