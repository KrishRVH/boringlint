//go:build !go1.27

package rangefunc

// Before Go 1.27, go/types admits variadic-yield range operands; Go 1.27
// rejects them during type checking. These cases keep the range guard aligned
// with every function range that reaches analysis.
type variadicSequence interface {
	~func(func(...int) bool)
}

func variadicYield(yield func(...int) bool) {
	yield(1)
}

func rangeVariadicFunction() {
	for values := range variadicYield { // want `range over a function value`
		_ = values
	}
}

func rangeVariadicGeneric[S variadicSequence](sequence S) {
	for values := range sequence { // want `range over a function value`
		_ = values
	}
}
