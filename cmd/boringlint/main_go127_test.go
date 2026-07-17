//go:build go1.27

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

import "example.com/genericmethod/dependency"

type Box[T any] struct{}

func (Box[T]) LocalMap[U any](convert func(T) U) U {
	var value T
	return convert(value)
}

func use(box dependency.Box[int], local Box[int]) {
	_ = box.Map(func(int) string { return "" })
	_ = local.LocalMap(func(int) string { return "" })
}
`)

	binary := buildCommand(t)
	assertDiagnostics(
		t,
		directory,
		binary,
		[]string{"./project"},
		nil,
		[]string{
			"project/genericmethod.go:7:15: generic method LocalMap declares method-local type parameters, which are forbidden by boringlint; use a package-level generic function",
			"project/genericmethod.go:13:10: use of generic method Map is forbidden by boringlint; use a package-level generic function",
			"project/genericmethod.go:14:12: use of generic method LocalMap is forbidden by boringlint; use a package-level generic function",
		},
	)
	assertDiagnostics(
		t,
		directory,
		"go",
		[]string{vetSubcommand, "-vettool=" + binary, "./project"},
		nil,
		[]string{
			"project/genericmethod.go:7:15: generic method LocalMap declares method-local type parameters, which are forbidden by boringlint; use a package-level generic function",
			"project/genericmethod.go:13:10: use of generic method Map is forbidden by boringlint; use a package-level generic function",
			"project/genericmethod.go:14:12: use of generic method LocalMap is forbidden by boringlint; use a package-level generic function",
		},
	)
}
