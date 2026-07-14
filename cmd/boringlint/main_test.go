package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCommand(t *testing.T) {
	t.Parallel()

	binary := buildCommand(t)
	command := exec.CommandContext(t.Context(), binary, "help") // #nosec G204 -- binary is built by this test.
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("list analyzers: %v\n%s", err, output)
	}
	for _, analyzer := range []string{"noiterator", "nogenericmethod"} {
		if !bytes.Contains(output, []byte(analyzer)) {
			t.Errorf("help output does not contain %q:\n%s", analyzer, output)
		}
	}

	testCommandModes(
		t,
		binary,
		filepath.Join("..", ".."),
		".",
		"./testdata/integration",
	)
}

func TestCommandTargetGoVersions(t *testing.T) {
	t.Parallel()

	binary := buildCommand(t)
	for _, version := range []string{"1.23.0", "1.24.0"} {
		t.Run("go"+version, func(t *testing.T) {
			directory := t.TempDir()
			writeTestFile(
				t,
				filepath.Join(directory, "go.mod"),
				"module example.com/target\n\ngo "+version+"\n",
			)
			writeTestFile(t, filepath.Join(directory, "clean", "clean.go"), `package clean

func values() []int {
	return []int{1, 2, 3}
}

func use() {
	for range values() {
	}
}
`)
			writeTestFile(t, filepath.Join(directory, "dependency", "dependency.go"), `package dependency

type Sequence[T any] interface {
	~func(func(T) bool)
}
`)
			writeTestFile(t, filepath.Join(directory, "diagnostic", "diagnostic.go"), `package diagnostic

import (
	"example.com/target/dependency"
	"iter"
)

type SequenceAlias = dependency.Sequence[int]

func values(yield func(int) bool) {
	yield(1)
}

func sequence() iter.Seq[int] {
	return values
}

func use() {
	for range sequence() {
	}
}
`)

			testCommandModes(
				t,
				binary,
				directory,
				"./clean",
				"./diagnostic",
				"iterator-shaped type example.com/target/dependency.Sequence[int]",
			)
		})
	}
}

func testCommandModes(
	t *testing.T,
	binary string,
	directory string,
	cleanTarget string,
	diagnosticTarget string,
	extraDiagnostics ...string,
) {
	t.Helper()

	tests := []struct {
		name                string
		executable          string
		diagnosticArguments []string
		cleanArguments      []string
	}{
		{
			name:                "standalone",
			executable:          binary,
			diagnosticArguments: []string{diagnosticTarget},
			cleanArguments:      []string{cleanTarget},
		},
		{
			name:                "vettool",
			executable:          "go",
			diagnosticArguments: []string{"vet", "-vettool=" + binary, diagnosticTarget},
			cleanArguments:      []string{"vet", "-vettool=" + binary, cleanTarget},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := append(
				[]string{"import of iter is forbidden", "range over a function value"},
				extraDiagnostics...,
			)
			assertDiagnostics(
				t,
				directory,
				test.executable,
				test.diagnosticArguments,
				diagnostics...,
			)
			assertSuccess(t, directory, test.executable, test.cleanArguments)
		})
	}
}

func buildCommand(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "boringlint.exe")
	command := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-o", binary, ".") // #nosec G204 -- all arguments are test-controlled.
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build boringlint: %v\n%s", err, output)
	}
	return binary
}

func assertDiagnostics(
	t *testing.T,
	directory string,
	executable string,
	arguments []string,
	want ...string,
) {
	t.Helper()

	command := exec.CommandContext(t.Context(), executable, arguments...) // #nosec G204 -- all arguments are test-controlled.
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("command succeeded; want diagnostics:\n%s", output)
	}
	for _, diagnostic := range want {
		if !bytes.Contains(output, []byte(diagnostic)) {
			t.Errorf("output does not contain %q:\n%s", diagnostic, output)
		}
	}
}

func assertSuccess(t *testing.T, directory string, executable string, arguments []string) {
	t.Helper()

	command := exec.CommandContext(t.Context(), executable, arguments...) // #nosec G204 -- all arguments are test-controlled.
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("command failed: %v\n%s", err, output)
	}
}

func writeTestFile(t *testing.T, path string, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
