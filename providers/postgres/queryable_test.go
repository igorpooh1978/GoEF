package postgres_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/linq/queryable"
	"github.com/igorpooh1978/GoEF/provider"
	"github.com/igorpooh1978/GoEF/providers/postgres"
)

func TestEqualityCompiler(t *testing.T) {
	compiler := postgres.EqualityCompiler{Schema: "app.schema"}
	read := queryable.EqualityRead{Table: "t\"able", Predicate: queryable.ReadColumn{Name: "к люч"}, Projection: []queryable.ReadColumn{{Name: "Case"}, {Name: "x\";--"}}}
	if compiler.Descriptor() != (provider.Descriptor{ID: "postgres", Major: provider.SPIMajor}) {
		t.Fatal("incorrect execution descriptor")
	}
	command, err := compiler.CompileEquality(read)
	if want := `SELECT "Case", "x"";--" FROM "app.schema"."t""able" WHERE "к люч" = $1 LIMIT 2`; err != nil || command != want {
		t.Fatalf("compiled SQL = %q, err = %v, want %q", command, err, want)
	}

	for _, tc := range []struct {
		name   string
		schema string
		read   queryable.EqualityRead
	}{
		{"empty schema", "", read},
		{"empty table", "public", queryable.EqualityRead{Predicate: queryable.ReadColumn{Name: "key"}, Projection: []queryable.ReadColumn{{Name: "value"}}}},
		{"empty predicate", "public", queryable.EqualityRead{Table: "table", Projection: []queryable.ReadColumn{{Name: "value"}}}},
		{"empty projection", "public", queryable.EqualityRead{Table: "table", Predicate: queryable.ReadColumn{Name: "key"}}},
		{"empty column", "public", queryable.EqualityRead{Table: "table", Predicate: queryable.ReadColumn{Name: "key"}, Projection: []queryable.ReadColumn{{Name: ""}}}},
		{"too many bytes", strings.Repeat("ю", 32), read},
		{"invalid utf8", "public", queryable.EqualityRead{Table: "table", Predicate: queryable.ReadColumn{Name: "key"}, Projection: []queryable.ReadColumn{{Name: "\xff"}}}},
		{"nul", "public", queryable.EqualityRead{Table: "table", Predicate: queryable.ReadColumn{Name: "k\x00ey"}, Projection: []queryable.ReadColumn{{Name: "value"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			compiler := postgres.EqualityCompiler{Schema: tc.schema}
			if sql, err := compiler.CompileEquality(tc.read); sql != "" || !errors.Is(err, goeferr.ErrQueryTranslation) {
				t.Fatal("invalid identifier or layout did not fail translation")
			}
		})
	}
}
