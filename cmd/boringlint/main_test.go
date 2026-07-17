package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	standaloneMode = "standalone"
	vettoolMode    = "vettool"
	vetSubcommand  = "vet"
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
	command = exec.CommandContext(t.Context(), binary, "help", "nogenericmethod") // #nosec G204 -- binary is built by this test.
	output, err = command.CombinedOutput()
	if err != nil {
		t.Fatalf("show nogenericmethod help: %v\n%s", err, output)
	}
	if !bytes.Contains(output, []byte("Go 1.27 or newer")) {
		t.Errorf("nogenericmethod help omits the driver requirement:\n%s", output)
	}

	want := []string{
		"testdata/integration/integration.go:3:8: import of iter is forbidden by boringlint; accept dependency iterators without naming their type and materialize them at the boundary",
		"testdata/integration/integration.go:5:6: iterator-shaped type func(yield func(int) bool) is forbidden by boringlint; materialize dependency iterators at the call boundary",
		"testdata/integration/integration.go:9:17: iterator-shaped type iter.Seq[int] is forbidden by boringlint; materialize dependency iterators at the call boundary",
		"testdata/integration/integration.go:14:15: range over a function value (iter.Seq[int]) is forbidden by boringlint; iterate concrete data or materialize at the dependency boundary",
	}
	testCommandModes(
		t,
		binary,
		filepath.Join("..", ".."),
		".",
		"./testdata/integration",
		want,
	)
	t.Run("selection", func(t *testing.T) {
		testAnalyzerSelectionModes(
			t,
			binary,
			filepath.Join("..", ".."),
			"./testdata/integration",
			want,
		)
	})
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
				[]string{
					"diagnostic/diagnostic.go:5:2: import of iter is forbidden by boringlint; accept dependency iterators without naming their type and materialize them at the boundary",
					"diagnostic/diagnostic.go:8:22: constraint example.com/target/dependency.Sequence[int] contains an iterator-shaped term, which is forbidden by boringlint; materialize dependency iterators at the call boundary",
					"diagnostic/diagnostic.go:10:6: iterator-shaped type func(yield func(int) bool) is forbidden by boringlint; materialize dependency iterators at the call boundary",
					"diagnostic/diagnostic.go:14:17: iterator-shaped type iter.Seq[int] is forbidden by boringlint; materialize dependency iterators at the call boundary",
					"diagnostic/diagnostic.go:19:6: range over a function value (iter.Seq[int]) is forbidden by boringlint; iterate concrete data or materialize at the dependency boundary",
				},
			)
		})
	}
}

func TestCommandRejectsHeterogeneousIteratorConstraint(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, "go.mod"), "module example.com/heterogeneous\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(directory, "dependency", "dependency.go"), `package dependency

type Sequence interface {
	~func(func(int) bool) | ~func(func(string) bool)
}
`)
	writeTestFile(t, filepath.Join(directory, "project", "project.go"), `package project

import "example.com/heterogeneous/dependency"

func Use[T dependency.Sequence]() {}
`)
	writeTestFile(t, filepath.Join(directory, "clean", "clean.go"), `package clean

func Values() []int { return []int{1, 2, 3} }
`)

	testCommandModes(
		t,
		buildCommand(t),
		directory,
		"./clean",
		"./project",
		[]string{
			"project/project.go:5:10: iterator-shaped type T is forbidden by boringlint; materialize dependency iterators at the call boundary",
		},
	)
}

func TestCommandReportsEachHiddenConstraintTerm(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, "go.mod"), "module example.com/hiddenterms\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(directory, "dependency", "dependency.go"), `package dependency

type SequenceOrSlice interface {
	~func(func(int) bool) | ~[]int
}

type SequenceOrMap interface {
	~func(func(int) bool) | ~map[int]int
}
`)
	writeTestFile(t, filepath.Join(directory, "project", "project.go"), `package project

import "example.com/hiddenterms/dependency"

type Pair[T interface {
	dependency.SequenceOrSlice
	dependency.SequenceOrMap
}] struct {
	Value T
}
`)
	writeTestFile(t, filepath.Join(directory, "clean", "clean.go"), `package clean

func Values() []int { return []int{1, 2, 3} }
`)

	testCommandModes(
		t,
		buildCommand(t),
		directory,
		"./clean",
		"./project",
		[]string{
			"project/project.go:6:2: constraint example.com/hiddenterms/dependency.SequenceOrSlice contains an iterator-shaped term, which is forbidden by boringlint; materialize dependency iterators at the call boundary",
			"project/project.go:7:2: constraint example.com/hiddenterms/dependency.SequenceOrMap contains an iterator-shaped term, which is forbidden by boringlint; materialize dependency iterators at the call boundary",
		},
	)
}

func testCommandModes(
	t *testing.T,
	binary string,
	directory string,
	cleanTarget string,
	diagnosticTarget string,
	want []string,
) {
	t.Helper()

	tests := []struct {
		name                string
		executable          string
		diagnosticArguments []string
		cleanArguments      []string
	}{
		{
			name:                standaloneMode,
			executable:          binary,
			diagnosticArguments: []string{diagnosticTarget},
			cleanArguments:      []string{cleanTarget},
		},
		{
			name:                vettoolMode,
			executable:          "go",
			diagnosticArguments: []string{vetSubcommand, "-vettool=" + binary, diagnosticTarget},
			cleanArguments:      []string{vetSubcommand, "-vettool=" + binary, cleanTarget},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertDiagnostics(
				t,
				directory,
				test.executable,
				test.diagnosticArguments,
				nil,
				want,
			)
			assertSuccess(t, directory, test.executable, test.cleanArguments)
		})
	}
}

func testAnalyzerSelectionModes(
	t *testing.T,
	binary string,
	directory string,
	target string,
	want []string,
) {
	t.Helper()

	tests := []struct {
		name          string
		executable    string
		baseArguments []string
	}{
		{
			name:       standaloneMode,
			executable: binary,
		},
		{
			name:       vettoolMode,
			executable: "go",
			baseArguments: []string{
				vetSubcommand,
				"-vettool=" + binary,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			noIteratorArguments := append(slices.Clone(test.baseArguments), "-noiterator", target)
			assertDiagnostics(t, directory, test.executable, noIteratorArguments, nil, want)

			noGenericMethodArguments := append(slices.Clone(test.baseArguments), "-nogenericmethod", target)
			assertSuccess(t, directory, test.executable, noGenericMethodArguments)
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
	environment []string,
	want []string,
) {
	t.Helper()

	command := exec.CommandContext(t.Context(), executable, arguments...) // #nosec G204 -- all arguments are test-controlled.
	command.Dir = directory
	if environment != nil {
		command.Env = environment
	}
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("command succeeded; want diagnostics:\n%s", output)
	}
	got := diagnosticLines(output, directory)
	slices.Sort(got)
	want = slices.Clone(want)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("diagnostics =\n%s\nwant =\n%s\nraw output:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"), output)
	}
}

func assertDiagnosticAlternatives(
	t *testing.T,
	directory string,
	executable string,
	arguments []string,
	environment []string,
	want [][]string,
) {
	t.Helper()

	command := exec.CommandContext(t.Context(), executable, arguments...) // #nosec G204 -- all arguments are test-controlled.
	command.Dir = directory
	command.Env = environment
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("command succeeded; want diagnostics:\n%s", output)
	}
	got := diagnosticLines(output, directory)
	slices.Sort(got)
	for _, alternative := range want {
		alternative = slices.Clone(alternative)
		slices.Sort(alternative)
		if slices.Equal(got, alternative) {
			return
		}
	}
	t.Errorf("diagnostics =\n%s\nwant one of =\n%v\nraw output:\n%s", strings.Join(got, "\n"), want, output)
}

func diagnosticLines(output []byte, directory string) []string {
	absoluteDirectory, err := filepath.Abs(directory)
	if err != nil {
		absoluteDirectory = directory
	}
	prefix := filepath.ToSlash(absoluteDirectory) + "/"
	var diagnostics []string
	for line := range strings.Lines(string(output)) {
		line = filepath.ToSlash(strings.TrimSpace(line))
		if line == "" || strings.HasPrefix(line, "# ") {
			continue
		}
		line = strings.TrimPrefix(line, prefix)
		diagnostics = append(diagnostics, line)
	}
	return diagnostics
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
