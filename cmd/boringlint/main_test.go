package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCommand(t *testing.T) {
	t.Parallel()

	binary := buildCommand(t)
	root := filepath.Join("..", "..")

	tests := []struct {
		name       string
		executable string
		arguments  []string
	}{
		{
			name:       "standalone",
			executable: binary,
			arguments:  []string{"./testdata/integration"},
		},
		{
			name:       "vettool",
			executable: "go",
			arguments:  []string{"vet", "-vettool=" + binary, "./testdata/integration"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertDiagnostics(
				t,
				root,
				test.executable,
				test.arguments,
				"import of iter is forbidden",
				"range over a function value",
			)
		})
	}
}

func buildCommand(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "boringlint")
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
