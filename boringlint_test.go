package boringlint

import (
	"go/ast"
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

	analysistest.Run(t, analysistest.TestData(), NoIterator, "iteratorboundary", "iteratorsignature", "rangefunc")
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
