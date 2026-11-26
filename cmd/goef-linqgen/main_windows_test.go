package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestRunPreservesExclusivelyOpenedFiles(t *testing.T) {
	for _, file := range []string{"source", "output"} {
		t.Run(file, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "query.go")
			output := filepath.Join(dir, "query_gen.go")
			writeFixture(t, source, []byte(querySource))
			args := []string{"-source", source, "-output", output}
			mustRun(t, args)

			locked := source
			if file == "output" {
				locked = output
			}
			before := readFixture(t, locked)
			func() {
				name, err := syscall.UTF16PtrFromString(locked)
				if err != nil {
					t.Fatal(err)
				}
				// A Windows editor or build tool can open a file without
				// sharing reads, while directory metadata is still visible.
				handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := syscall.CloseHandle(handle); err != nil {
						t.Error(err)
					}
				}()
				_, readErr := os.ReadFile(locked)
				var expected *os.PathError
				if !errors.As(readErr, &expected) {
					t.Fatalf("expected the exclusive open to prevent reading, got %v", readErr)
				}
				if err := run(args, io.Discard); !errors.Is(err, expected.Err) {
					t.Fatalf("expected the file sharing violation, got %v", err)
				}
			}()
			if contents := readFixture(t, locked); !bytes.Equal(contents, before) {
				t.Fatalf("generation changed the exclusively opened file:\n%s", contents)
			}
		})
	}
}
