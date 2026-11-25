package fixture_test

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestCompileRejections(t *testing.T) {
	for _, tc := range []struct{ path, actual, expected string }{
		{"foreign_owner", "fixture.Predicate[invoice]", "as fixture.Predicate[order] value in argument"},
		{"slot_type", "type fixture.Slot[quantity]", "does not match inferred type fixture.Slot[amount]"},
		{"result_type", "fixture.Query[orderID]", "as fixture.Query[int] value in variable declaration"},
		{"local_callback", "value of type func(order) bool", "as fixture.Predicate[order] value in argument"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), "go", "build", "./testdata/"+tc.path)
			cmd.Env = append(cmd.Environ(), "GOWORK=off", "GOPROXY=off", "GOTOOLCHAIN=local")
			output, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Fatalf("expected build exit 1, got %v\n%s", err, output)
			}
			for _, part := range []string{"invalid.go:", tc.actual, tc.expected} {
				if !strings.Contains(string(output), part) {
					t.Fatalf("missing compiler diagnostic %q\n%s", part, output)
				}
			}
		})
	}
}
