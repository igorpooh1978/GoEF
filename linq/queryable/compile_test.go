package queryable_test

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestTypedParametersAndOwners(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "go", "build", "./testdata/wrong_types")
	cmd.Env = append(cmd.Environ(), "GOWORK=off", "GOPROXY=off", "GOTOOLCHAIN=local")
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("expected compile rejection, got %v\n%s", err, output)
	}
	for _, want := range []string{"as expressions.Equality[Customer, string]", "cannot use 42", "as string value in argument to plan.Fetch"} {
		if !strings.Contains(string(output), want) {
			t.Fatalf("expected compiler diagnostic %q\n%s", want, output)
		}
	}
}
