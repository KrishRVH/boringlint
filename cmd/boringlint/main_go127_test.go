//go:build go1.27

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommandRejectsGenericMethods(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, "go.mod"), "module example.com/genericmethod\n\ngo 1.27\n")
	writeTestFile(t, filepath.Join(directory, "genericmethod.go"), `package genericmethod

type Box[T any] struct{}

func (Box[T]) Map[U any](convert func(T) U) U {
	var value T
	return convert(value)
}

func use(box Box[int]) {
	_ = box.Map(func(int) string { return "" })
}
`)

	binary := buildCommand(t)
	assertDiagnostics(
		t,
		directory,
		binary,
		[]string{"."},
		"declares method-local type parameters",
		"use of generic method Map",
	)
	assertDiagnostics(
		t,
		directory,
		"go",
		[]string{"vet", "-vettool=" + binary, "."},
		"declares method-local type parameters",
		"use of generic method Map",
	)
}

func writeTestFile(t *testing.T, path string, contents string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
