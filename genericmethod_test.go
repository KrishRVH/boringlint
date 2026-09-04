package boringlint

import (
	"bytes"
	"go/token"
	"go/types"
	"os"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestNoGenericMethod(t *testing.T) {
	t.Parallel()

	testdata, cleanup, err := analysistest.WriteFiles(map[string]string{
		"dependency/a_plain.go": `package dependency

type Box[T any] struct{}

func (Box[T]) Value() T {
	var value T
	return value
}

func Map[T, U any](value T, convert func(T) U) U {
	return convert(value)
}

type Wrapper[T any] struct{ Box[T] }
`,
		"dependency/b_generic.go": `package dependency

func (Box[T]) Map[U any](convert func(T) U) U { // want "generic method Map"
	var value T
	return convert(value)
}
`,
		"project/a_plain.go": `package project

import "dependency"

func useAllowed(box dependency.Box[int]) {
	_ = box.Value()
	_ = dependency.Map(1, func(int) string { return "" })
}
`,
		"project/b_generic.go": `package project

import "dependency"

func use(box dependency.Box[int]) {
	_ = box.Map[string](func(int) string { return "" }) // want "use of generic method Map"
	_ = box.Map(func(int) string { return "" })        // want "use of generic method Map"
	methodValue := box.Map[string]                     // want "use of generic method Map"
	_ = methodValue
	methodExpression := dependency.Box[int].Map[string] // want "use of generic method Map"
	_ = methodExpression
	var wrapper dependency.Wrapper[int]
	_ = wrapper.Map[string] // want "use of generic method Map"
	_ = []func(func(int) string) string{box.Map} // want "use of generic method Map"
	_ = func(func(int) string) string(box.Map) // want "use of generic method Map"
}

func methodValue(box dependency.Box[int]) func(func(int) string) string {
	return box.Map // want "use of generic method Map"
}
`,
		"stdlib/stdlib.go": `package stdlib

import "math/rand/v2"

type Random struct{ *rand.Rand }

func use(random *rand.Rand, embedded Random) {
	_ = random.N(10) // want "use of generic method N"
	_ = random.N[int64](10) // want "use of generic method N"
	_ = (*rand.Rand).N[int] // want "use of generic method N"
	_ = embedded.N[int] // want "use of generic method N"
	_ = rand.N(10)
	_ = random.IntN(10)
}
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)

	results := analysistest.Run(t, testdata, NoGenericMethod, "dependency", "project", "stdlib")
	assertDiagnosticStartsAt(t, results, "declares method-local type parameters", "Map")
}

func TestNoIteratorGenericMethods(t *testing.T) {
	t.Parallel()

	testdata, cleanup, err := analysistest.WriteFiles(map[string]string{
		"genericiterator/genericiterator.go": `package genericiterator

type Source struct{}

func (Source) Values[T any](yield func(T) bool) {} // want "iterator-shaped type"

func (Source) Consume[S ~func(func(int) bool)](sequence S) {} // want "iterator-shaped type" "iterator-shaped type"

func (Source) Collect[T any](values []T) []T { return values }

func use(source Source) {
	for range source.Values[int] { // want "range over a function value"
	}
}
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)

	analysistest.Run(t, testdata, NoIterator, "genericiterator")
}

func assertDiagnosticStartsAt(
	t *testing.T,
	results []*analysistest.Result,
	message string,
	want string,
) {
	t.Helper()

	matches := 0
	for _, result := range results {
		for _, diagnostic := range result.Diagnostics {
			if !strings.Contains(diagnostic.Message, message) {
				continue
			}
			position := result.Pass.Fset.Position(diagnostic.Pos)
			contents, err := os.ReadFile(position.Filename)
			if err != nil {
				t.Fatal(err)
			}
			if position.Offset >= len(contents) ||
				!bytes.HasPrefix(contents[position.Offset:], []byte(want)) {
				t.Errorf("diagnostic position = %s, want start of %q", position, want)
			}
			matches++
		}
	}
	if matches != 1 {
		t.Errorf("matched diagnostics = %d, want 1", matches)
	}
}

func TestHasMethodTypeParameters(t *testing.T) {
	t.Parallel()

	constraint := types.NewInterfaceType(nil, nil).Complete()
	newTypeParameter := func(name string) *types.TypeParam {
		return types.NewTypeParam(types.NewTypeName(token.NoPos, nil, name, nil), constraint)
	}
	receiverType := types.NewNamed(
		types.NewTypeName(token.NoPos, nil, "Receiver", nil),
		types.NewStruct(nil, nil),
		nil,
	)
	receiver := types.NewVar(token.NoPos, nil, "receiver", receiverType)

	method := types.NewFunc(
		token.NoPos,
		nil,
		"Map",
		types.NewSignatureType(receiver, nil, []*types.TypeParam{newTypeParameter("T")}, nil, nil, false),
	)
	if !hasMethodTypeParameters(method) {
		t.Fatal("generic method was not recognized")
	}

	function := types.NewFunc(
		token.NoPos,
		nil,
		"Map",
		types.NewSignatureType(nil, nil, []*types.TypeParam{newTypeParameter("T")}, nil, nil, false),
	)
	if hasMethodTypeParameters(function) {
		t.Fatal("generic function was recognized as a generic method")
	}

	receiverGenericMethod := types.NewFunc(
		token.NoPos,
		nil,
		"Get",
		types.NewSignatureType(receiver, []*types.TypeParam{newTypeParameter("R")}, nil, nil, nil, false),
	)
	if hasMethodTypeParameters(receiverGenericMethod) {
		t.Fatal("receiver type parameters were recognized as method-local type parameters")
	}

	if hasMethodTypeParameters(types.NewTypeName(token.NoPos, nil, "NotAFunction", nil)) {
		t.Fatal("non-function object was recognized as a generic method")
	}
}
