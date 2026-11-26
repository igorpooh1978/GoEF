package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFreshness(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "query.go")
	writeFixture(t, source, []byte(querySource))
	first := filepath.Join(dir, "query_first_gen.go")
	second := filepath.Join(dir, "query_second_gen.go")
	generateFirst := []string{"-source", source, "-output", first}
	checkFirst := append(append([]string(nil), generateFirst...), "-check")

	if err := run(checkFirst, io.Discard); err == nil || !strings.Contains(err.Error(), "is missing") {
		t.Fatalf("expected missing output, got %v", err)
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatalf("missing output check created a file: %v", err)
	}
	mustRun(t, generateFirst)
	mustRun(t, []string{"-source", source, "-output", second})
	if actual, expected := readFixture(t, first), readFixture(t, second); !bytes.Equal(actual, expected) {
		t.Fatalf("outputs differ:\n%s\n%s", actual, expected)
	}

	mustRun(t, checkFirst)
	changed := append(readFixture(t, first), []byte("// edited manually\n")...)
	writeFixture(t, first, changed)
	if err := run(checkFirst, io.Discard); err == nil || !strings.Contains(err.Error(), "is stale") {
		t.Fatalf("changed output was not rejected: %v", err)
	}
	if actual := readFixture(t, first); !bytes.Equal(actual, changed) {
		t.Fatalf("check changed manually edited output:\n%s", actual)
	}
	mustRun(t, generateFirst)
	mustRun(t, checkFirst)

	// A legitimate declaration edit changes the fingerprint and the projection.
	writeFixture(t, source, []byte(strings.Replace(querySource, "row.Name", "row.City", 1)))
	previous := readFixture(t, first)
	if err := run(checkFirst, io.Discard); err == nil || !strings.Contains(err.Error(), "is stale") {
		t.Fatalf("a changed declaration was not rejected: %v", err)
	}
	if actual := readFixture(t, first); !bytes.Equal(actual, previous) {
		t.Fatalf("check rewrote a stale file:\n%s", actual)
	}
	mustRun(t, generateFirst)
	mustRun(t, checkFirst)
	if actual := readFixture(t, first); !bytes.Contains(actual, []byte(`Projection: "City"`)) {
		t.Fatalf("regeneration retained the old projection:\n%s", actual)
	}
	crlf := func(contents []byte) []byte { return bytes.ReplaceAll(contents, []byte("\n"), []byte("\r\n")) }
	writeFixture(t, source, crlf(readFixture(t, source)))
	writeFixture(t, first, crlf(readFixture(t, first)))
	mustRun(t, checkFirst)
}

func TestCheckedInBindingIsFresh(t *testing.T) {
	dir := filepath.Join("..", "..", "examples", "linq", "generated")
	mustRun(t, []string{"-source", filepath.Join(dir, "query.go"), "-output", filepath.Join(dir, "query_gen.go"), "-check"})
}

func TestRunRejectsUnsafePaths(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "query.go")
	writeFixture(t, source, []byte(querySource))
	output := filepath.Join(dir, "query_gen.go")
	var diagnostics bytes.Buffer
	if err := run([]string{"-help"}, &diagnostics); err != nil || !strings.Contains(diagnostics.String(), "-check") {
		t.Fatalf("expected help with exit success, got %q, %v", diagnostics.String(), err)
	}
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"bad flag", []string{"-unknown"}, "flag provided"},
		{"bad source name", []string{"-source", "source.txt", "-output", output}, "ordinary .go"},
		{"ignored file name", []string{"-source", source, "-output", "_ignored.go"}, "ordinary .go"},
		{"test file name", []string{"-source", source, "-output", "binding_test.go"}, "ordinary .go"},
		{"unexpected argument", []string{"-source", source, "-output", output, "extra"}, "ordinary .go"},
		{"same file", []string{"-source", source, "-output", source}, "different files"},
		{"different directory", []string{"-source", source, "-output", filepath.Join(dir, "other", "out.go")}, "same directory"},
		{"source not found", []string{"-source", filepath.Join(dir, "absent.go"), "-output", output}, "absent.go"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := run(tt.args, io.Discard); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q, got %v", tt.want, err)
			}
		})
	}

	userSource := []byte("package sample\nvar KeepMe bool\n")
	writeFixture(t, output, userSource)
	args := []string{"-source", source, "-output", output}
	if err := run(args, io.Discard); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("expected protection for hand-written file, got %v", err)
	}
	if current := readFixture(t, output); !bytes.Equal(current, userSource) {
		t.Fatalf("ordinary file changed:\n%s", current)
	}

	directory := filepath.Join(dir, "directory.go")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-source", directory, "-output", output}, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "source must be a regular file") {
		t.Fatalf("expected directory source rejection, got %v", err)
	}
	if err := run([]string{"-source", source, "-output", directory}, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "output must be a separate regular file") {
		t.Fatalf("expected directory output rejection, got %v", err)
	}
	linked := filepath.Join(dir, "linked.go")
	if err := os.Link(source, linked); err != nil {
		t.Skipf("requires hard link support: %v", err)
	}
	if err := run([]string{"-source", source, "-output", linked}, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "output must be a separate regular file") {
		t.Fatalf("expected hard link rejection, got %v", err)
	}
}

func TestWriteFailureLeavesNoTemporaryArtifact(t *testing.T) {
	dir := t.TempDir()
	if err := writeOutput(filepath.Join(dir, "missing", "out.go"), nil); err == nil {
		t.Fatal("write unexpectedly succeeded with absent directory")
	}
	directory := filepath.Join(dir, "directory.go")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeOutput(directory, []byte(querySource)); err == nil {
		t.Fatal("write unexpectedly replaced the existing directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "directory.go" {
		t.Fatalf("temporary file was retained: %v, %v", entries, err)
	}
}

func mustRun(t *testing.T, args []string) {
	t.Helper()
	if err := run(args, io.Discard); err != nil {
		t.Fatalf("goef-linqgen %q: %v", args, err)
	}
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

func writeFixture(t *testing.T, path string, contents []byte) {
	t.Helper()
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
}
