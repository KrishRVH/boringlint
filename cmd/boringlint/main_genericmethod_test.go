package main

import (
	"path/filepath"
	"testing"
)

func TestCommandRejectsGenericMethods(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, "go.mod"), "module example.com/genericmethod\n\ngo 1.27\n")
	writeTestFile(t, filepath.Join(directory, "dependency", "genericmethod.go"), `package dependency

type Box[T any] struct{}

func (Box[T]) Map[U any](convert func(T) U) U {
	var value T
	return convert(value)
}
`)
	writeTestFile(t, filepath.Join(directory, "project", "genericmethod.go"), `package project

import (
	"example.com/genericmethod/dependency"
	"math/rand/v2"
)

type Box[T any] struct{}

func (Box[T]) LocalMap[U any](convert func(T) U) U {
	var value T
	return convert(value)
}

func use(box dependency.Box[int], local Box[int], random *rand.Rand) {
	_ = box.Map(func(int) string { return "" })
	_ = local.LocalMap(func(int) string { return "" })
	_ = random.N(10)
}
`)

	writeTestFile(t, filepath.Join(directory, "clean", "clean.go"), `package clean

import "math/rand/v2"

type Fields struct {
	Value   int
	Convert func(int) int
}

type Record struct{ Fields }

func identity[T any](value T) T { return value }

func allowed(random *rand.Rand) {
	_ = Record{Value: 1, Convert: identity}
	_ = []func(int) int{identity}
	_ = func(int) int(identity)
	_ = rand.N(10)
	_ = random.IntN(10)
}

func converter() func(int) int { return identity }
`)

	testCommandModes(
		t,
		buildCommand(t),
		directory,
		"./clean",
		"./project",
		[]string{
			"project/genericmethod.go:10:15: generic method LocalMap declares method-local type parameters, which are forbidden by boringlint; use a package-level generic function",
			"project/genericmethod.go:16:10: use of generic method Map is forbidden by boringlint; use a package-level generic function",
			"project/genericmethod.go:17:12: use of generic method LocalMap is forbidden by boringlint; use a package-level generic function",
			"project/genericmethod.go:18:13: use of generic method N is forbidden by boringlint; use a package-level generic function",
		},
	)
}
