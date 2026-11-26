package model

import "fmt"

type scalarProfile struct {
	types         map[ScalarKind]string
	boundedText   string
	maxTextLength int
}

var scalarProfiles = map[TypeProvider]scalarProfile{
	TypePostgreSQL: {
		types: map[ScalarKind]string{
			ScalarBool: "boolean", ScalarInt: "bigint", ScalarInt8: "smallint", ScalarInt16: "smallint",
			ScalarInt32: "integer", ScalarInt64: "bigint", ScalarString: "text", ScalarBytes: "bytea",
		},
		boundedText:   "varchar(%d)",
		maxTextLength: 10_485_760,
	},
	TypeSQLite: {
		types: map[ScalarKind]string{
			ScalarBool: "INTEGER", ScalarInt: "INTEGER", ScalarInt8: "INTEGER", ScalarInt16: "INTEGER",
			ScalarInt32: "INTEGER", ScalarInt64: "INTEGER", ScalarString: "TEXT", ScalarBytes: "BLOB",
		},
	},
	TypeSQLServer: {
		types: map[ScalarKind]string{
			ScalarBool: "bit", ScalarInt: "bigint", ScalarInt8: "smallint", ScalarInt16: "smallint",
			ScalarInt32: "int", ScalarInt64: "bigint", ScalarString: "nvarchar(max)", ScalarBytes: "varbinary(max)",
		},
		boundedText:   "nvarchar(%d)",
		maxTextLength: 4_000,
	},
	TypeMySQL: {
		types: map[ScalarKind]string{
			ScalarBool: "boolean", ScalarInt: "bigint", ScalarInt8: "tinyint", ScalarInt16: "smallint",
			ScalarInt32: "int", ScalarInt64: "bigint", ScalarString: "longtext CHARACTER SET utf8mb4", ScalarBytes: "longblob",
		},
		boundedText: "varchar(%d) CHARACTER SET utf8mb4",
		// Four bytes per character, less the two-byte VARCHAR prefix, against the 65,535-byte row limit.
		maxTextLength: (65_535 - 2) / 4,
	},
}

func (p scalarProfile) storeType(kind ScalarKind, length int) (string, string) {
	store, ok := p.types[kind]
	switch {
	case !ok:
		return "", "field type has no supported scalar binding"
	case length == 0:
		return store, ""
	case kind != ScalarString:
		return "", "maximum length requires a text field"
	case p.boundedText == "":
		return store, ""
	case length > p.maxTextLength:
		return "", "maximum length exceeds the provider limit"
	default:
		return fmt.Sprintf(p.boundedText, length), ""
	}
}
