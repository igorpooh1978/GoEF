package shape_test

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompilerRejections(t *testing.T) {
	cases := []struct{ fixture, diagnostic string }{
		{"interface_method", "interface method must have no type parameters"},
		{"satisfaction", "wrong type for method Write"},
		{"recursive", "instantiation cycle"},
		{"owner", "as shape.Field[account, bool] value in argument"},
		{"result", "as shape.Query[int] value in variable declaration"},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			args := []string{"build", "./testdata/" + tc.fixture}
			wantExit := 1
			if tc.fixture == "interface_method" {
				// This syntax error is stored as text so gofmt can walk the module.
				args = []string{"tool", "compile", "-o", filepath.Join(t.TempDir(), "invalid.o"), "testdata/interface_method/invalid.go.txt"}
				wantExit = 2
			}
			cmd := exec.CommandContext(t.Context(), "go", args...)
			output, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != wantExit ||
				!strings.Contains(string(output), tc.diagnostic) {
				t.Fatalf("expected compiler rejection %q, got %v\n%s", tc.diagnostic, err, output)
			}
		})
	}
}
