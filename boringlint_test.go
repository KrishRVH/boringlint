package boringlint

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

func TestAnalyzersAreValid(t *testing.T) {
	t.Parallel()

	if err := analysis.Validate([]*analysis.Analyzer{NoIterator, NoGenericMethod}); err != nil {
		t.Fatal(err)
	}
}

func TestAnalyzerMetadata(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		analyzer *analysis.Analyzer
		name     string
		url      string
	}{
		{analyzer: NoIterator, name: "noiterator", url: "https://github.com/KrishRVH/boringlint#noiterator"},
		{analyzer: NoGenericMethod, name: "nogenericmethod", url: "https://github.com/KrishRVH/boringlint#nogenericmethod"},
	} {
		if test.analyzer.Name != test.name {
			t.Errorf("analyzer name = %q, want %q", test.analyzer.Name, test.name)
		}
		if test.analyzer.URL != test.url {
			t.Errorf("%s URL = %q, want %q", test.name, test.analyzer.URL, test.url)
		}
		if len(test.analyzer.Requires) != 1 || test.analyzer.Requires[0] != inspect.Analyzer {
			t.Errorf("%s Requires = %v, want inspect.Analyzer", test.name, test.analyzer.Requires)
		}
	}
}

func TestNoIterator(t *testing.T) {
	t.Parallel()

	analysistest.Run(t, analysistest.TestData(), NoIterator, "iteratorboundary", "iteratorsignature", "rangefunc")
}

func TestIteratorTypeSets(t *testing.T) {
	t.Parallel()

	const source = `package typeparams

type Mixed interface {
	~func(func(int) bool) | ~[]int
}

type MixedMap interface {
	~func(func(int) bool) | ~map[string]int
}

type Eliminated interface {
	Mixed
	~[]int
}

type Empty interface {
	Mixed
	~string
}

type MethodSequence func(func(int) bool)

func (MethodSequence) Keep() {}

type MethodSlice []int

type MethodNarrowed interface {
	MethodSequence | MethodSlice
	Keep()
}

type TildeMethodSequence interface {
	~func(func(int) bool)
	Keep()
}

type MissingIterator interface {
	func(func(int) bool)
	Missing()
}

type MissingBool interface {
	bool
	Missing()
}

type PointerBase struct{}

func (*PointerBase) Keep() {}

func heterogeneous[T interface {
	~func(func(int) bool) | ~func(func(string) bool)
}](value T) {}

func heterogeneousYield[Y interface {
	~func(int) bool | ~func(string) bool
}](sequence func(Y)) {}

func exactBool[B bool](sequence func(func(int) B)) {}

func mixed[T Mixed](value T) {}

func narrowed[T interface {
	Mixed
	MixedMap
}](value T) {}

func approximateBool[B ~bool](sequence func(func(int) B)) {}

func eliminated[T Eliminated](value T) {}

func empty[T Empty](value T) {}

func methodNarrowed[T MethodNarrowed](value T) {}

func tildeMethodSequence[T TildeMethodSequence](value T) {}

func missingIterator[T MissingIterator](value T) {}

func missingBool[B MissingBool](sequence func(func(int) B)) {}

func top[T interface {
	any | ~func(func(int) bool)
}](value T) {}

func comparableIterator[T interface {
	~func(func(int) bool)
	comparable
}](value T) {}

func methodsOnly[T interface{ Keep() }](value T) {}

func anything[T any](value T) {}

func pointerInherited[T interface {
	~*PointerBase
	Keep()
}](value T) {}

func pointerEmpty[T interface {
	~*int
	Keep()
}](value T) {}

type IntSequence interface {
	~func(func(int) bool)
}

type StringSequence interface {
	~func(func(string) bool)
}

type DiamondLeft interface{ Mixed }

type DiamondRight interface{ Mixed }

func anyUnionWithMethod[T interface {
	any | ~func(func(int) bool)
	Keep()
}](value T) {}

func interfaceUnion[T interface {
	IntSequence | StringSequence
}](value T) {}

func mixedInterfaceUnion[T interface {
	Mixed | MixedMap
}](value T) {}

func exactNamed[T MethodSequence](value T) {}

func sliceWithMethod[T interface {
	~func(func(int) bool) | ~[]int
	Keep()
}](value T) {}

func structWithMethod[T interface {
	~func(func(int) bool) | ~struct{ Value int }
	Keep()
}](value T) {}

func intWithMethod[T interface {
	~func(func(int) bool) | ~int
	Keep()
}](value T) {}

func variadicMethod[T interface {
	~func(func(int) bool)
	Keep(...int)
}](value T) {}

func diamond[T interface {
	DiamondLeft
	DiamondRight
	MixedMap
}](value T) {}
`
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "typeparams.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := new(types.Config).Check("typeparams", fileSet, []*ast.File{file}, nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name string
		want bool
	}{
		{name: "heterogeneous", want: true},
		{name: "heterogeneousYield", want: true},
		{name: "exactBool", want: true},
		{name: "narrowed", want: true},
		{name: "methodNarrowed", want: true},
		{name: "tildeMethodSequence", want: true},
		{name: "interfaceUnion", want: true},
		{name: "exactNamed", want: true},
		{name: "variadicMethod", want: true},
		{name: "diamond", want: true},
		{name: "anyUnionWithMethod", want: false},
		{name: "mixedInterfaceUnion", want: false},
		{name: "sliceWithMethod", want: false},
		{name: "structWithMethod", want: false},
		{name: "intWithMethod", want: false},
		{name: "mixed", want: false},
		{name: "approximateBool", want: false},
		{name: "eliminated", want: false},
		{name: "empty", want: false},
		{name: "missingIterator", want: false},
		{name: "missingBool", want: false},
		{name: "top", want: false},
		{name: "comparableIterator", want: false},
		{name: "methodsOnly", want: false},
		{name: "anything", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			function := pkg.Scope().Lookup(test.name).Type().(*types.Signature)
			if got := isIteratorType(function.Params().At(0).Type()); got != test.want {
				t.Fatalf("isIteratorType() = %t, want %t", got, test.want)
			}
		})
	}

	for _, test := range []struct {
		name string
		want bool
	}{
		{name: "pointerInherited", want: true},
		{name: "pointerEmpty", want: false},
		{name: "methodsOnly", want: false},
		{name: "anything", want: false},
		{name: "top", want: false},
	} {
		t.Run(test.name+"Witness", func(t *testing.T) {
			t.Parallel()

			function := pkg.Scope().Lookup(test.name).Type().(*types.Signature)
			typeParam := function.TypeParams().At(0)
			got := hasOnlyAcceptedTypes(typeParam, func(types.Type) bool { return true })
			if got != test.want {
				t.Fatalf("hasOnlyAcceptedTypes() = %t, want %t", got, test.want)
			}
		})
	}

	// Equal offsets normalize 200 syntactic terms to 100 candidates. Offset 99
	// produces 199 distinct candidates whose intersection retains exactly one.
	for _, offset := range []int64{0, 99} {
		if !isIteratorType(newIteratorIntersection(offset)) {
			t.Fatalf("isIteratorType() = false for accepted 100-term intersections with offset %d", offset)
		}
	}
}

// newIteratorIntersection returns a type parameter constrained by two 100-term
// unions of iterator signatures. The second union's yield parameters start at
// [offset]int, so offset 0 repeats the first union.
func newIteratorIntersection(offset int64) *types.TypeParam {
	union := func(offset int64) *types.Interface {
		terms := make([]*types.Term, 100)
		for index := range terms {
			yield := types.NewSignatureType(
				nil,
				nil,
				nil,
				types.NewTuple(types.NewParam(
					token.NoPos,
					nil,
					"value",
					types.NewArray(types.Typ[types.Int], offset+int64(index)),
				)),
				types.NewTuple(types.NewParam(token.NoPos, nil, "ok", types.Typ[types.Bool])),
				false,
			)
			sequence := types.NewSignatureType(
				nil,
				nil,
				nil,
				types.NewTuple(types.NewParam(token.NoPos, nil, "yield", yield)),
				nil,
				false,
			)
			terms[index] = types.NewTerm(true, sequence)
		}
		return types.NewInterfaceType(nil, []types.Type{types.NewUnion(terms)}).Complete()
	}
	constraint := types.NewInterfaceType(nil, []types.Type{union(0), union(offset)}).Complete()
	return types.NewTypeParam(types.NewTypeName(token.NoPos, nil, "T", nil), constraint)
}

func TestIteratorTypeSetUnexportedMethods(t *testing.T) {
	t.Parallel()

	projectPkg := types.NewPackage("example.com/project", "project")
	pkg := types.NewPackage("example.com/dependency", "dependency")
	otherPkg := types.NewPackage("example.com/other", "other")
	yield := types.NewSignatureType(
		nil,
		nil,
		nil,
		types.NewTuple(types.NewParam(token.NoPos, nil, "value", types.Typ[types.Int])),
		types.NewTuple(types.NewParam(token.NoPos, nil, "ok", types.Typ[types.Bool])),
		false,
	)
	iterator := types.NewSignatureType(
		nil,
		nil,
		nil,
		types.NewTuple(types.NewParam(token.NoPos, nil, "yield", yield)),
		nil,
		false,
	)
	for _, test := range []struct {
		name     string
		otherPkg *types.Package
		want     bool
	}{
		{name: "same package", otherPkg: pkg, want: true},
		// A witness can declare only its own package's unexported methods, and
		// a function type cannot embed a type that would promote the others.
		{name: "different packages", otherPkg: otherPkg, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// NewInterfaceType records receivers in method signatures, so each
			// parallel subtest needs its own.
			methodSignature := types.NewSignatureType(nil, nil, nil, nil, nil, false)
			methods := []*types.Func{
				types.NewFunc(token.NoPos, pkg, "first", methodSignature),
				types.NewFunc(token.NoPos, test.otherPkg, "second", methodSignature),
			}
			constraint := types.NewInterfaceType(
				methods,
				[]types.Type{types.NewUnion([]*types.Term{types.NewTerm(true, iterator)})},
			).Complete()
			typeParam := types.NewTypeParam(
				types.NewTypeName(token.NoPos, projectPkg, "T", nil),
				constraint,
			)

			if got := isIteratorType(typeParam); got != test.want {
				t.Fatalf("isIteratorType() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestParenthesizedIteratorTypeHasOneDiagnostic(t *testing.T) {
	t.Parallel()

	// gofumpt strips these parentheses from checked-in fixtures.
	testdata, cleanup, err := analysistest.WriteFiles(map[string]string{
		"parenthesized/parenthesized.go": `package parenthesized

type Sequence func(func(int) bool) // want "iterator-shaped type"

type ParenthesizedIterator (Sequence) // want "iterator-shaped type"
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)

	analysistest.Run(t, testdata, NoIterator, "parenthesized")
}

func BenchmarkIsIteratorType(b *testing.B) {
	yield := types.NewSignatureType(
		nil,
		nil,
		nil,
		types.NewTuple(types.NewParam(token.NoPos, nil, "value", types.Typ[types.Int])),
		types.NewTuple(types.NewParam(token.NoPos, nil, "ok", types.Typ[types.Bool])),
		false,
	)
	iterator := types.NewSignatureType(
		nil,
		nil,
		nil,
		types.NewTuple(types.NewParam(token.NoPos, nil, "yield", yield)),
		nil,
		false,
	)

	b.ReportAllocs()
	for b.Loop() {
		if !isIteratorType(iterator) {
			b.Fatal("iterator signature was not recognized")
		}
	}
}

// BenchmarkIsIteratorTypeSet measures the type-set path, which builds and
// checks a candidate for each of 199 distinct terms.
func BenchmarkIsIteratorTypeSet(b *testing.B) {
	typeParam := newIteratorIntersection(99)

	b.ReportAllocs()
	for b.Loop() {
		if !isIteratorType(typeParam) {
			b.Fatal("iterator type set was not recognized")
		}
	}
}
