package main

import (
	"bytes"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

const querySource = `package sample

type Customer struct { City string; Name string }

//goef:query FindCustomer
func findCustomer(row Customer, city string) (bool, string) {
	return row.City == city, row.Name
}
`

func TestGenerateTypedBindings(t *testing.T) {
	const input = querySource + `
type goefValue int32
type goefRow bool
type goefbindings struct {
	Estimate goefValue
	Active goefRow
}

//goef:query FilterEstimate
func filterEstimate(record goefbindings, estimate goefValue) (bool, goefRow) {
	return ((record).Estimate == (estimate)), record.Active
}
`
	first, err := generate("relative/query.go", []byte(input))
	if err != nil {
		t.Fatal(err)
	}
	second, err := generate("different/query.go", []byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("identical source produced different output")
	}
	for _, want := range []string{
		generatedHeader, "// Source SHA-256: ",
		"func FindCustomer(goefValue_ string) goefbindings_.Equality[Customer, string, string]",
		"func FilterEstimate(goefValue_ goefValue) goefbindings_.Equality[goefbindings, goefValue, goefRow]",
		`EqualityShape{Field: "Estimate", Parameter: "estimate", Projection: "Active"}`,
		"func(goefRow_ goefbindings) goefRow { return goefRow_.Active }",
	} {
		if !strings.Contains(string(first), want) {
			t.Errorf("output missing %q\n%s", want, first)
		}
	}
}

func TestRejectedDeclarations(t *testing.T) {
	const noMarker = "package sample\n"
	for _, tt := range []struct {
		name   string
		source string
		want   string
	}{
		{"parse", "package", "expected"},
		{"missing marker", noMarker, "expected at least one"},
		{"unattached marker", noMarker + "//goef:query M\nvar sample = 1", "must document a function"},
		{"bad entry name", strings.Replace(querySource, "FindCustomer", "find-customer", 1), "one exported entry point"},
		{"missing entry name", strings.Replace(querySource, " FindCustomer", "", 1), "one exported entry point"},
		{"private entry name", strings.Replace(querySource, "FindCustomer", "findCustomer", 1), "one exported entry point"},
		{"duplicate declaration", querySource + "\n//goef:query FindCustomer\nfunc duplicate() {}\n", "duplicate query marker"},
		{"duplicate marker", strings.Replace(querySource, "//goef:query FindCustomer", "//goef:query FindCustomer\n//goef:query FindOther", 1), "duplicate query marker"},
		{"entry conflict", querySource + "\ntype FindCustomer int\n", "entry point already exists"},
		{"import", strings.Replace(querySource, "package sample", `package sample
import "fmt"`, 1), "contain no imports"},
		{"build tag", "//go:build linux\n\n" + querySource, "available in every build"},
		{"legacy build tag", "// +build linux\n\n" + querySource, "available in every build"},
		{"type error", strings.Replace(querySource, "city string", "city int", 1), "mismatched types"},
		{"result count", noMarker + "//goef:query Lookup\nfunc lookup() {}\n", "expected func("},
		{"nominal bool result", strings.Replace(querySource, "(bool, string)", "(Flag, string)", 1) + "\ntype Flag bool\n", "expected func("},
		{"method", strings.Replace(querySource, "func findCustomer", "func (customer Customer) findCustomer", 1), "expected func("},
		{"generic function", strings.Replace(querySource, "func findCustomer(", "func findCustomer[T any](", 1), "expected func("},
		{"pointer receiver row", strings.Replace(querySource, "row Customer", "row *Customer", 1), "named struct in the query source"},
		{"nonscalar projection", strings.Replace(strings.Replace(querySource, "Name string", "Name []byte", 1), "(bool, string)", "(bool, []byte)", 1), "supported scalar types"},
		{"nonscalar argument", strings.ReplaceAll(querySource, "string", "float64"), "supported scalar types"},
		{"pointer projection", strings.Replace(querySource, "(bool, string)", "(bool, *string)", 1), "cannot use"},
		{"named nonstruct", noMarker + "type Row int\n//goef:query Lookup\nfunc lookup(row Row, n int) (bool, int) {return row == 0, n}", "named non-generic struct"},
		{"generic struct", strings.Replace(strings.Replace(querySource, "struct { City", "[T any] struct { City", 1), "row Customer", "row Customer[bool]", 1), "named non-generic struct"},
		{"blank argument", strings.Replace(strings.Replace(querySource, "city string", "_ string", 1), "== city", `== "London"`, 1), "must have names"},
		{"multi-statement", strings.Replace(querySource, "return row.City", "_ = row.Name; return row.City", 1), "one return statement"},
		{"empty return", strings.Replace(strings.Replace(querySource, "(bool, string)", "(ok bool, name string)", 1), "return row.City == city, row.Name", "return", 1), "return a predicate and a projected row field"},
		{"not equality", strings.Replace(querySource, "==", "!=", 1), "compare a direct row field"},
		{"local callback", strings.Replace(querySource, "row.City == city", "locally(row.City, city)", 1) + "func locally(a, b string) bool {return a == b}\n", "query.go:7:9: predicate must compare"},
		{"argument first", strings.Replace(querySource, "row.City == city", "city == row.City", 1), "expected row.Field"},
		{"ambient capture", strings.Replace(querySource, "== city", "== currentCity", 1) + "\nvar currentCity string\n", "expected row.Field"},
		{"method value", strings.Replace(querySource, "row.Name", "row.fullName()", 1) + "\nfunc (row Customer) fullName() string { return row.Name }\n", "projection must be a direct row field"},
		{"other owner", strings.Replace(querySource, "row.Name", "other.Name", 1) + "\nvar other Customer\n", "projection must be a direct row field"},
		{"promoted field", strings.Replace(querySource, "City string; Name string", "City string; NameHolder", 1) + "\ntype NameHolder struct { Name string }\n", "projection must be a direct row field"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			output, err := generate("query.go", []byte(tt.source))
			if err == nil || !strings.Contains(err.Error(), "query.go:") || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected source diagnostic containing %q, got %v\n%s", tt.want, err, output)
			}
			if len(output) != 0 {
				t.Fatalf("received partial output: %s", output)
			}
		})
	}
}

func FuzzGenerate(f *testing.F) {
	f.Add([]byte(querySource))
	f.Add([]byte("package p\n//goef:query Lookup\nfunc _(row int, arg int) (bool, int) { return row == arg, row }\n"))
	f.Add([]byte("package"))
	f.Fuzz(func(t *testing.T, source []byte) {
		first, err := generate("query.go", source)
		if err != nil {
			if len(first) != 0 {
				t.Fatalf("invalid input produced partial output:\n%s", first)
			}
			return
		}
		second, err := generate("elsewhere/query.go", source)
		if err != nil || !bytes.Equal(first, second) {
			t.Fatalf("generation was not deterministic: %v", err)
		}
		if _, err := parser.ParseFile(token.NewFileSet(), "query_gen.go", first, 0); err != nil {
			t.Fatalf("generated source did not parse: %v\n%s", err, first)
		}
	})
}
