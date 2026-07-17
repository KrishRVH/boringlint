package boringlint

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
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
	if !strings.Contains(NoGenericMethod.Doc, "Go 1.27 or newer") {
		t.Error("nogenericmethod metadata omits the Go 1.27 driver requirement")
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

	makeTerms := func(offset int64) []*types.Term {
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
		return terms
	}
	// Equal offsets normalize 200 syntactic terms to 100 candidates. Offset 99
	// produces 199 distinct candidates whose intersection retains exactly one.
	for _, offset := range []int64{0, 99} {
		first := types.NewInterfaceType(nil, []types.Type{types.NewUnion(makeTerms(0))}).Complete()
		second := types.NewInterfaceType(nil, []types.Type{types.NewUnion(makeTerms(offset))}).Complete()
		constraint := types.NewInterfaceType(nil, []types.Type{first, second}).Complete()
		typeParam := types.NewTypeParam(types.NewTypeName(token.NoPos, nil, "T", nil), constraint)
		if !isIteratorType(typeParam) {
			t.Fatalf("isIteratorType() = false for accepted 100-term intersections with offset %d", offset)
		}
	}
}

func TestIteratorTypeSetUnexportedMethods(t *testing.T) {
	t.Parallel()

	methodSignature := types.NewSignatureType(nil, nil, nil, nil, nil, false)
	pkg := types.NewPackage("example.com/dependency", "dependency")
	methods := []*types.Func{
		types.NewFunc(token.NoPos, pkg, "first", methodSignature),
		types.NewFunc(token.NoPos, pkg, "second", methodSignature),
	}
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
	constraint := types.NewInterfaceType(
		methods,
		[]types.Type{types.NewUnion([]*types.Term{types.NewTerm(true, iterator)})},
	).Complete()
	typeParam := types.NewTypeParam(
		types.NewTypeName(token.NoPos, pkg, "T", nil),
		constraint,
	)

	if !isIteratorType(typeParam) {
		t.Fatal("isIteratorType() = false for same-package unexported methods")
	}
}

func TestSamePackage(t *testing.T) {
	t.Parallel()

	pkg := types.NewPackage("example.com/dependency", "dependency")
	for _, test := range []struct {
		name        string
		left, right *types.Package
		want        bool
	}{
		{name: "same object", left: pkg, right: pkg, want: true},
		{
			name:  "same path",
			left:  types.NewPackage("example.com/dependency", "first"),
			right: types.NewPackage("example.com/dependency", "second"),
			want:  true,
		},
		{
			name:  "different path",
			left:  types.NewPackage("example.com/first", "dependency"),
			right: types.NewPackage("example.com/second", "dependency"),
			want:  false,
		},
		{name: "both nil", want: true},
		{name: "one nil", left: pkg, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := samePackage(test.left, test.right); got != test.want {
				t.Fatalf("samePackage() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestParenthesizedIteratorTypeHasOneDiagnostic(t *testing.T) {
	t.Parallel()

	const source = `package parenthesized

type Sequence func(func(int) bool)
type ParenthesizedIterator (Sequence)
`
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "parenthesized.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	typeInfo := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	pkg, err := new(types.Config).Check("parenthesized", fileSet, []*ast.File{file}, typeInfo)
	if err != nil {
		t.Fatal(err)
	}
	typeSpec := file.Decls[1].(*ast.GenDecl).Specs[0].(*ast.TypeSpec)
	var diagnostics []analysis.Diagnostic
	pass := &analysis.Pass{
		Analyzer:  NoIterator,
		Fset:      fileSet,
		Pkg:       pkg,
		TypesInfo: typeInfo,
		Report: func(diagnostic analysis.Diagnostic) {
			diagnostics = append(diagnostics, diagnostic)
		},
	}

	reportIteratorTypeSpec(pass, typeSpec)

	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %v, want exactly one", diagnostics)
	}
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

func TestReportGenericMethods(t *testing.T) {
	t.Parallel()

	genericMethod := &ast.FuncDecl{
		Name: ast.NewIdent("Map"),
		Recv: &ast.FieldList{List: []*ast.Field{{Type: ast.NewIdent("Receiver")}}},
		Type: &ast.FuncType{TypeParams: &ast.FieldList{
			List: []*ast.Field{{Names: []*ast.Ident{ast.NewIdent("T")}}},
		}},
	}
	file := &ast.File{Decls: []ast.Decl{
		genericMethod,
		&ast.FuncDecl{Name: ast.NewIdent("GenericFunction"), Type: genericMethod.Type},
		&ast.FuncDecl{Name: ast.NewIdent("PlainMethod"), Recv: genericMethod.Recv, Type: &ast.FuncType{}},
	}}

	var got []string
	reportGenericMethods(file, func(decl *ast.FuncDecl) {
		got = append(got, decl.Name.Name)
	})
	if len(got) != 1 || got[0] != "Map" {
		t.Fatalf("reported methods = %v, want [Map]", got)
	}
}
