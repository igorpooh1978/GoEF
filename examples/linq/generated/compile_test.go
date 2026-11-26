package generated_test

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/igorpooh1978/GoEF/examples/linq/generated"
	"github.com/igorpooh1978/GoEF/linq/bindings"
)

func TestGeneratedShape(t *testing.T) {
	first := generated.FindCustomer("London")
	second := generated.FindCustomer("Berlin")
	want := bindings.EqualityShape{Field: "City", Parameter: "city", Projection: "Name"}
	firstShape := first.Shape()
	if firstShape != want || second.Shape() != want {
		t.Fatalf("generated shape should identify the declared members, got %#v and %#v", firstShape, second.Shape())
	}
	firstShape.Field = "Name"
	if first.Shape() != want || first.Value() != "London" || second.Value() != "Berlin" {
		t.Fatalf("projection metadata or invocation value was mutated, got %#v, %q, %q", first.Shape(), first.Value(), second.Value())
	}
}

func TestGeneratedTypesRejectedByCompiler(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "go", "build", "./testdata/wrong_types")
	cmd.Env = append(cmd.Environ(), "GOWORK=off", "GOPROXY=off", "GOTOOLCHAIN=local")
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("expected build exit 1, got %v\n%s", err, output)
	}
	for _, want := range []string{
		"invalid.go:", "cannot use 123", "as string value in argument",
		"as int value in variable declaration", "cannot use ForeignRow{}", "as generated.Customer value in argument",
	} {
		if !strings.Contains(string(output), want) {
			t.Fatalf("missing compiler diagnostic %q\n%s", want, output)
		}
	}
}
