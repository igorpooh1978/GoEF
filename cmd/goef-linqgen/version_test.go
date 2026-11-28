package main

import (
	"bytes"
	"runtime/debug"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	for _, flag := range []string{"-version", "--version"} {
		t.Run(flag, func(t *testing.T) {
			var stdout, diagnostics bytes.Buffer
			if err := run([]string{flag}, &stdout, &diagnostics); err != nil {
				t.Fatal(err)
			}
			line, ok := strings.CutPrefix(stdout.String(), "goef-linqgen ")
			if !ok || len(strings.Fields(line)) == 0 || strings.Count(line, "\n") != 1 || !strings.HasSuffix(line, "\n") || diagnostics.Len() != 0 {
				t.Fatalf("stdout = %q, stderr = %q", stdout.String(), diagnostics.String())
			}
		})
	}
	for _, args := range [][]string{{"-version", "-check"}, {"-version", "-source", "query.go"}, {"-version", "query.go"}} {
		var stdout, diagnostics bytes.Buffer
		if err := run(args, &stdout, &diagnostics); err == nil || stdout.Len() != 0 {
			t.Fatalf("run(%q) = %v, stdout = %q", args, err, stdout.String())
		}
	}
}

func TestWriteVersion(t *testing.T) {
	const revision = "388e85cb27d3bebe9a4674b922e235658f647570"
	cases := []struct {
		name string
		info *debug.BuildInfo
		want string
	}{
		{"missing build information", nil, "(devel)"},
		{"tagged installation",
			&debug.BuildInfo{Main: debug.Module{Version: "v0.2.0-alpha.1"}}, "v0.2.0-alpha.1"},
		{"dirty checkout", &debug.BuildInfo{
			Main: debug.Module{Version: "v0.1.0-alpha.1.0.20261126184244-388e85cb27d3+dirty"},
			Settings: []debug.BuildSetting{
				{Key: "vcs.modified", Value: "true"},
				{Key: "vcs.revision", Value: revision},
				{Key: "-ldflags", Value: "unused"},
			},
		}, "v0.1.0-alpha.1.0.20261126184244-388e85cb27d3+dirty revision=" + revision + " dirty=true"},
		{"local replacement", &debug.BuildInfo{
			Main: debug.Module{Version: "v0.1.0-alpha.1", Replace: &debug.Module{Path: "../goef"}},
		}, "(devel)"},
		{"versioned replacement", &debug.BuildInfo{
			Main: debug.Module{Version: "v0.1.0-alpha.1", Replace: &debug.Module{Version: "v0.2.0-alpha.1"}},
		}, "v0.2.0-alpha.1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := writeVersion(&output, tc.info); err != nil {
				t.Fatal(err)
			}
			if want := "goef-linqgen " + tc.want + "\n"; output.String() != want {
				t.Fatalf("got %q, want %q", output.String(), want)
			}
		})
	}
}
