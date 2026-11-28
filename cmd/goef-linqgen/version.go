package main

import (
	"fmt"
	"io"
	"runtime/debug"
	"strings"
)

func writeVersion(output io.Writer, info *debug.BuildInfo) error {
	version := "(devel)"
	var revision, modified string
	if info != nil {
		module := &info.Main
		if module.Replace != nil {
			module = module.Replace
		}
		if module.Version != "" {
			version = module.Version
		}
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				revision = setting.Value
			case "vcs.modified":
				modified = setting.Value
			}
		}
	}

	fields := []string{"goef-linqgen", version}
	if revision != "" {
		fields = append(fields, "revision="+revision)
	}
	if modified != "" {
		fields = append(fields, "dirty="+modified)
	}
	_, err := fmt.Fprintln(output, strings.Join(fields, " "))
	return err
}
