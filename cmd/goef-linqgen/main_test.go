package main

import (
	"bytes"
	"errors"
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

	if err := run(checkFirst, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "is missing") {
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
	if err := run(checkFirst, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "is stale") {
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
	if err := run(checkFirst, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "is stale") {
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
	if err := run([]string{"-help"}, io.Discard, &diagnostics); err != nil || !strings.Contains(diagnostics.String(), "-check") {
		t.Fatalf("expected help with exit success, got %q, %v", diagnostics.String(), err)
	}
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"bad flag", []string{"-unknown"}, "flag provided"},
		{"bad source name", []string{"-source", "source.txt", "-output", output}, "portable .go"},
		{"ignored file name", []string{"-source", source, "-output", "_ignored.go"}, "portable .go"},
		{"test file name", []string{"-source", source, "-output", "binding_test.go"}, "portable .go"},
		{"unexpected argument", []string{"-source", source, "-output", output, "extra"}, "portable .go"},
		{"platform source", []string{"-source", filepath.Join(dir, "query_linux_amd64.go"), "-output", output}, "portable .go"},
		{"platform output", []string{"-source", source, "-output", filepath.Join(dir, "binding_windows.go")}, "portable .go"},
		{"same file", []string{"-source", source, "-output", source}, "different files"},
		{"different directory", []string{"-source", source, "-output", filepath.Join(dir, "other", "out.go")}, "same directory"},
		{"source not found", []string{"-source", filepath.Join(dir, "absent.go"), "-output", output}, "absent.go"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := run(tt.args, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q, got %v", tt.want, err)
			}
		})
	}

	userSource := []byte("package sample\nvar KeepMe bool\n")
	writeFixture(t, output, userSource)
	args := []string{"-source", source, "-output", output}
	if err := run(args, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("expected protection for hand-written file, got %v", err)
	}
	if current := readFixture(t, output); !bytes.Equal(current, userSource) {
		t.Fatalf("ordinary file changed:\n%s", current)
	}

	directory := filepath.Join(dir, "directory.go")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-source", directory, "-output", output}, io.Discard, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "source must be a regular file") {
		t.Fatalf("expected directory source rejection, got %v", err)
	}
	if err := run([]string{"-source", source, "-output", directory}, io.Discard, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "output must be a separate regular file") {
		t.Fatalf("expected directory output rejection, got %v", err)
	}
	linked := filepath.Join(dir, "linked.go")
	if err := os.Link(source, linked); err != nil {
		t.Skipf("requires hard link support: %v", err)
	}
	if err := run([]string{"-source", source, "-output", linked}, io.Discard, io.Discard); err == nil ||
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
	if err := run(args, io.Discard, io.Discard); err != nil {
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

func TestPortableGoFileNames(t *testing.T) {
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"query.go", true},
		{"query_gen.go", true},
		{"linux.go", true},
		{"query_linux_sample.go", true},
		{"query_windows.go", false},
		{"query_amd64.go", false},
		{"query_linux_amd64.go", false},
		{"query_js_wasm.go", false},
		{"query_test.go", false},
		{"_query.go", false},
		{".query.go", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := goFileName(tc.name); got != tc.want {
				t.Fatalf("portable name %q = %t; want %t", tc.name, got, tc.want)
			}
		})
	}
}

func TestInvalidSourceLeavesExistingBinding(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "query.go")
	output := filepath.Join(dir, "query_gen.go")
	writeFixture(t, source, []byte(querySource))
	args := []string{"-source", source, "-output", output}
	mustRun(t, args)
	original := readFixture(t, output)

	writeFixture(t, source, []byte(strings.Replace(querySource, "==", "!=", 1)))
	if err := run(args, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "predicate must compare") {
		t.Fatalf("expected source diagnostic, got %v", err)
	}
	if contents := readFixture(t, output); !bytes.Equal(contents, original) {
		t.Fatalf("generation rewrote the binding with invalid input:\n%s", contents)
	}
}

func TestRunReportsInvalidOutputPath(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "query.go")
	output := filepath.Join(dir, strings.Repeat("query", 100)+".go")
	writeFixture(t, source, []byte(querySource))

	_, pathErr := os.Lstat(output)
	if pathErr == nil || os.IsNotExist(pathErr) {
		t.Skip("filesystem does not report long filename errors from Lstat")
	}
	var expected, failure *os.PathError
	if !errors.As(pathErr, &expected) {
		t.Fatalf("expected a filesystem path error, got %v", pathErr)
	}
	err := run([]string{"-source", source, "-output", output}, io.Discard, io.Discard)
	if !errors.As(err, &failure) || failure.Op != expected.Op || failure.Path != output || !errors.Is(err, expected.Err) {
		t.Fatalf("expected the output path diagnostic, got %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "query.go" {
		t.Fatalf("generation left an artifact after rejecting the output path: %v, %v", entries, err)
	}
}

func TestRunPreservesUnreadableRegularFiles(t *testing.T) {
	for _, file := range []string{"source", "output"} {
		t.Run(file, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "query.go")
			output := filepath.Join(dir, "query_gen.go")
			writeFixture(t, source, []byte(querySource))
			args := []string{"-source", source, "-output", output}
			mustRun(t, args)

			protected := source
			if file == "output" {
				protected = output
			}
			before := readFixture(t, protected)
			if err := os.Chmod(protected, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chmod(protected, 0o600); err != nil {
					t.Error(err)
				}
			})
			if _, err := os.ReadFile(protected); !errors.Is(err, os.ErrPermission) {
				t.Skipf("filesystem permits reading this file mode: %v", err)
			}
			if err := run(args, io.Discard, io.Discard); !errors.Is(err, os.ErrPermission) {
				t.Fatalf("expected the read permission error, got %v", err)
			}
			if err := os.Chmod(protected, 0o600); err != nil {
				t.Fatal(err)
			}
			if contents := readFixture(t, protected); !bytes.Equal(contents, before) {
				t.Fatalf("generation changed the protected file:\n%s", contents)
			}
		})
	}
}

func TestCommandEntryPointGeneratesBinding(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "query.go")
	output := filepath.Join(dir, "query_gen.go")
	writeFixture(t, source, []byte(querySource))

	prior := os.Args
	t.Cleanup(func() { os.Args = prior })
	os.Args = []string{"goef-linqgen", "-source", source, "-output", output}
	main()
	mustRun(t, []string{"-source", source, "-output", output, "-check"})
}
