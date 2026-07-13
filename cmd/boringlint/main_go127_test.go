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
		"declares method-local type parameters",
		"use of generic method Map",
		"use of generic method LocalMap",
	)
	assertDiagnostics(
		t,
		directory,
		"go",
		[]string{"vet", "-vettool=" + binary, "./project"},
		"declares method-local type parameters",
		"use of generic method Map",
		"use of generic method LocalMap",
	)
}
