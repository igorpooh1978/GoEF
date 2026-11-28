package postgres

import (
	"strings"
	"unicode/utf8"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/linq/queryable"
	"github.com/igorpooh1978/GoEF/provider"
	"github.com/jackc/pgx/v5"
)

// EqualityCompiler lowers typed equality reads to PostgreSQL SQL. Schema is
// required and identifies the application-controlled schema holding the
// mapped table. Identifiers are interpreted exactly, including punctuation.
type EqualityCompiler struct {
	Schema string
}

var _ queryable.EqualityCompiler = EqualityCompiler{}

// Descriptor names the dialect's compatible execution provider.
func (EqualityCompiler) Descriptor() provider.Descriptor {
	return provider.Descriptor{ID: providerID, Major: provider.SPIMajor}
}

// CompileEquality quotes mapped identifiers and emits one argument slot.
// PostgreSQL equality uses the column's comparison semantics, including its
// collation. A NULL predicate column does not match a non-null argument.
func (c EqualityCompiler) CompileEquality(read queryable.EqualityRead) (string, error) {
	if !validIdentifier(c.Schema) || !validIdentifier(read.Table) ||
		!validIdentifier(read.Predicate.Name) || len(read.Projection) == 0 {
		return "", goeferr.NewQueryTranslation()
	}
	columns := make([]string, len(read.Projection))
	for i, column := range read.Projection {
		if !validIdentifier(column.Name) {
			return "", goeferr.NewQueryTranslation()
		}
		columns[i] = pgx.Identifier{column.Name}.Sanitize()
	}
	table := pgx.Identifier{c.Schema, read.Table}.Sanitize()
	predicate := pgx.Identifier{read.Predicate.Name}.Sanitize()
	return "SELECT " + strings.Join(columns, ", ") + " FROM " + table + " WHERE " + predicate + " = $1 LIMIT 2", nil
}

func validIdentifier(name string) bool {
	// PostgreSQL's standard build truncates names beyond 63 bytes.
	return len(name) > 0 && len(name) <= 63 && utf8.ValidString(name) && !strings.ContainsRune(name, 0)
}
