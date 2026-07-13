package boringlint

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzersAreValid(t *testing.T) {
	t.Parallel()

	if err := analysis.Validate([]*analysis.Analyzer{NoIterator, NoGenericMethod}); err != nil {
		t.Fatal(err)
	}
}

func TestNoIterator(t *testing.T) {
	t.Parallel()

	analysistest.Run(t, analysistest.TestData(), NoIterator, "iteratorsignature", "rangefunc")
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

func TestNoGenericMethodSyntax(t *testing.T) {
	t.Parallel()

	const source = `package p

type Result[V any] struct{ value V }

func (result Result[V]) Map[U any](convert func(V) U) U { return convert(result.value) }
func (result Result[V]) Get() V { return result.value }
func Map[V, U any](value V, convert func(V) U) U { return convert(value) }
`

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "generic_method.go", source, parser.SkipObjectResolution)
	if file == nil {
		t.Fatalf("parser returned no file: %v", err)
	}

	var got []string
	reportGenericMethods(file, func(decl *ast.FuncDecl) {
		got = append(got, decl.Name.Name)
	})

	if parserSupportsGenericMethods() {
		if len(got) != 1 || got[0] != "Map" {
			t.Fatalf("reported methods = %v, want [Map]", got)
		}
		return
	}
	if err == nil {
		t.Fatal("pre-Go 1.27 parser accepted a generic method")
	}
	if len(got) != 0 {
		t.Fatalf("pre-Go 1.27 parser retained method type parameters: %v", got)
	}
}

func parserSupportsGenericMethods() bool {
	fileSet := token.NewFileSet()
	file, _ := parser.ParseFile(
		fileSet,
		"probe.go",
		"package p\ntype R[V any] struct{}\nfunc (R[V]) M[U any]() {}\n",
		parser.SkipObjectResolution,
	)
	if file == nil {
		return false
	}
	for _, declaration := range file.Decls {
		decl, ok := declaration.(*ast.FuncDecl)
		if ok && decl.Recv != nil && decl.Type.TypeParams != nil {
			return true
		}
	}
	return false
}
