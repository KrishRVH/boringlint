//go:build !go1.27

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommandLegacyAliasRepresentation(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, "go.mod"), "module example.com/legacyalias\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(directory, "dependency", "dependency.go"), `package dependency

type MixedAlias = interface {
	~func(func(int) bool) | ~[]int
}
`)
	writeTestFile(t, filepath.Join(directory, "project", "project.go"), `package project

import "example.com/legacyalias/dependency"

type Missed = dependency.MixedAlias
`)

	binary := buildCommand(t)
	want := [][]string{
		{
			"project/project.go:5:15: constraint example.com/legacyalias/dependency.MixedAlias contains an iterator-shaped term, which is forbidden by boringlint; materialize dependency iterators at the call boundary",
		},
		{
			"project/project.go:5:15: constraint interface{~func(func(int) bool) | ~[]int} contains an iterator-shaped term, which is forbidden by boringlint; materialize dependency iterators at the call boundary",
		},
	}
	for _, test := range []struct {
		name       string
		executable string
		arguments  []string
	}{
		{name: "standalone", executable: binary, arguments: []string{"./project"}},
		{name: "vettool", executable: "go", arguments: []string{vetSubcommand, "-vettool=" + binary, "./project"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertDiagnosticAlternatives(
				t,
				directory,
				test.executable,
				test.arguments,
				append(os.Environ(), "GODEBUG=gotypesalias=0"),
				want,
			)
		})
	}
}
