package model

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
)

// renderCustomerSource exercises field and key order for one synthetic schema.
// Its allowlist binds each supported field name and nullability to a Go type.
func renderCustomerSource(customer EntityDecl) ([]byte, error) {
	if err := Validate(Declaration{Entities: []EntityDecl{customer}}); err != nil {
		return nil, err
	}
	if customer.Name != "Customer" {
		return nil, fmt.Errorf("unsupported fixture entity")
	}

	var source bytes.Buffer
	source.WriteString("package fixture\n\ntype Customer struct {\n")
	for _, field := range customer.Properties {
		var goType string
		switch {
		case (field.Name == "ID" || field.Name == "TenantID") && !field.Nullable:
			goType = "int64"
		case field.Name == "Note" && field.Nullable:
			goType = "*string"
		default:
			return nil, fmt.Errorf("unsupported fixture property or nullability")
		}
		fmt.Fprintf(&source, "%s %s\n", field.Name, goType)
	}
	source.WriteString("}\n\nvar customerPrimaryKey = [...]string{")
	for _, name := range customer.PrimaryKey {
		fmt.Fprintf(&source, "%q,", name)
	}
	source.WriteString("}\n")
	return format.Source(source.Bytes())
}

// This exercises the file and ordering assertions of MODEL-MAP-001-S01 on
// resolved declarations. The four provider scenarios also require inspection.
func TestCustomerSourceAndSnapshotAcrossDirectories(t *testing.T) {
	t.Parallel()
	customer := EntityDecl{
		Name:       "Customer",
		Properties: []PropertyDecl{{Name: "Note", Nullable: true}, {Name: "ID"}, {Name: "TenantID"}},
		PrimaryKey: []string{"TenantID", "ID"},
	}

	// Distinct orderings prevent a fixed source template from satisfying the test.
	for _, variant := range []struct {
		name        string
		entity      EntityDecl
		fields, key []string
	}{
		{
			name: "original", entity: customer,
			fields: []string{"Note *string", "ID int64", "TenantID int64"}, key: []string{"TenantID", "ID"},
		},
		{
			name: "reordered",
			entity: EntityDecl{
				Name:       "Customer",
				Properties: []PropertyDecl{customer.Properties[2], customer.Properties[0], customer.Properties[1]},
				PrimaryKey: []string{"ID", "TenantID"},
			},
			fields: []string{"TenantID int64", "Note *string", "ID int64"}, key: []string{"ID", "TenantID"},
		},
	} {
		t.Run(variant.name, func(t *testing.T) {
			var previousSource, previousSnapshot []byte
			for pass := range 2 {
				dir := t.TempDir()
				snapshot, err := canonicalSnapshot(Declaration{Entities: []EntityDecl{variant.entity}})
				if err != nil {
					t.Fatal(err)
				}
				source, err := renderCustomerSource(variant.entity)
				if err != nil {
					t.Fatal(err)
				}
				gotSource := writeAndReadFixture(t, dir, "customer.go", source)
				gotSnapshot := writeAndReadFixture(t, dir, "snapshot.json", snapshot)
				if pass == 0 {
					previousSource, previousSnapshot = gotSource, gotSnapshot
					checkCustomerAST(t, gotSource, variant.fields, variant.key)
					cmd := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-o", filepath.Join(t.TempDir(), "customer.a"), "customer.go")
					cmd.Dir = dir
					cmd.Env = append(cmd.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "GOPROXY=off")
					if output, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("build generated source: %v\n%s", err, output)
					}
				} else if !bytes.Equal(gotSource, previousSource) || !bytes.Equal(gotSnapshot, previousSnapshot) {
					t.Fatal("independent directories produced different source or snapshot bytes")
				}
			}
		})
	}
}

func writeAndReadFixture(t *testing.T, dir, name string, data []byte) []byte {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	result, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func checkCustomerAST(t *testing.T, source []byte, wantFields, wantKey []string) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "customer.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	var fields, key []string
	for node := range ast.Preorder(file) {
		switch spec := node.(type) {
		case *ast.TypeSpec:
			if spec.Name.Name != "Customer" {
				continue
			}
			structure, ok := spec.Type.(*ast.StructType)
			if !ok {
				t.Fatal("Customer is not a struct")
			}
			for _, field := range structure.Fields.List {
				for _, name := range field.Names {
					fields = append(fields, name.Name+" "+types.ExprString(field.Type))
				}
			}
		case *ast.ValueSpec:
			if len(spec.Names) != 1 || spec.Names[0].Name != "customerPrimaryKey" || len(spec.Values) != 1 {
				continue
			}
			list, ok := spec.Values[0].(*ast.CompositeLit)
			if !ok {
				t.Fatal("primary key is not a composite literal")
			}
			for _, expression := range list.Elts {
				literal, ok := expression.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					t.Fatal("primary key contains a non-string member")
				}
				name, err := strconv.Unquote(literal.Value)
				if err != nil {
					t.Fatal(err)
				}
				key = append(key, name)
			}
		}
	}
	if !slices.Equal(fields, wantFields) || !slices.Equal(key, wantKey) {
		t.Fatalf("generated field order %v, key order %v; want fields %v, key %v", fields, key, wantFields, wantKey)
	}
}

func TestCustomerSourceRejectsUnsupportedInputs(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name     string
		customer EntityDecl
	}{
		{"unsupported entity", EntityDecl{Name: "Customer{}"}},
		{"invalid field identifier", EntityDecl{Name: "Customer", Properties: []PropertyDecl{{Name: "ID TenantID"}}}},
		{"unsupported field nullability", EntityDecl{Name: "Customer", Properties: []PropertyDecl{{Name: "Note"}}}},
		{"unresolved key", EntityDecl{Name: "Customer", Properties: []PropertyDecl{{Name: "ID"}}, PrimaryKey: []string{"unmapped"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if generated, err := renderCustomerSource(test.customer); err == nil || generated != nil {
				t.Fatal("unsupported fixture produced source")
			}
		})
	}
}
