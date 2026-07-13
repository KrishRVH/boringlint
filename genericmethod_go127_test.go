//go:build go1.27

package boringlint

import (
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestNoGenericMethod(t *testing.T) {
	t.Parallel()

	testdata, cleanup, err := analysistest.WriteFiles(map[string]string{
		"dependency/dependency.go": `package dependency

type Box[T any] struct{}

func (Box[T]) Map[U any](convert func(T) U) U { // want "generic method Map"
	var value T
	return convert(value)
}

func (Box[T]) Value() T {
	var value T
	return value
}

func Map[T, U any](value T, convert func(T) U) U {
	return convert(value)
}

type Wrapper[T any] struct{ Box[T] }
`,
		"project/project.go": `package project

import "dependency"

func use(box dependency.Box[int]) {
	_ = box.Map[string](func(int) string { return "" }) // want "use of generic method Map"
	_ = box.Map(func(int) string { return "" })         // want "use of generic method Map"
	methodValue := box.Map[string]                       // want "use of generic method Map"
	_ = methodValue
	methodExpression := dependency.Box[int].Map[string] // want "use of generic method Map"
	_ = methodExpression
	var wrapper dependency.Wrapper[int]
	_ = wrapper.Map[string] // want "use of generic method Map"
	_ = box.Value()
	_ = dependency.Map(1, func(int) string { return "" })
}
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)

	analysistest.Run(t, testdata, NoGenericMethod, "dependency", "project")
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
