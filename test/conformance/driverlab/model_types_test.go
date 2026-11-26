package driverlab

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/model"
)

const modelTypeTable = "goef_model_type_probe"

type modelTypeFixture struct {
	ID     int64
	Active bool
	Tiny   int8
	Small  int16
	Number int32
	Native int
	Text   *string
	Label  *string `goef:"maxLength=16"`
	Bytes  []byte
}

var modelTypeColumns = [...]struct {
	field, column string
	nullable      bool
	maxLength     int
}{
	{field: "ID", column: "mapped_id"},
	{field: "Active", column: "mapped_active"},
	{field: "Tiny", column: "mapped_tiny"},
	{field: "Small", column: "mapped_small"},
	{field: "Number", column: "mapped_number"},
	{field: "Native", column: "mapped_native"},
	{field: "Text", column: "mapped_text", nullable: true},
	{field: "Label", column: "mapped_label", nullable: true, maxLength: 16},
	{field: "Bytes", column: "mapped_bytes"},
}

type modelTypeTarget struct {
	provider      model.TypeProvider
	versionFact   string
	store, native [len(modelTypeColumns)]string
}

var modelTypeTargets = map[string]modelTypeTarget{
	"postgres": {
		provider:    model.TypePostgreSQL,
		versionFact: "engine_version",
		store:       [...]string{"bigint", "boolean", "smallint", "smallint", "integer", "bigint", "text", "varchar(16)", "bytea"},
		native:      [...]string{"bigint", "boolean", "smallint", "smallint", "integer", "bigint", "text", "character varying", "bytea"},
	},
	"sqlite": {
		provider:    model.TypeSQLite,
		versionFact: "engine_version",
		store:       [...]string{"INTEGER", "INTEGER", "INTEGER", "INTEGER", "INTEGER", "INTEGER", "TEXT", "TEXT", "BLOB"},
		native:      [...]string{"integer", "integer", "integer", "integer", "integer", "integer", "text", "text", "blob"},
	},
	"sqlserver": {
		provider:    model.TypeSQLServer,
		versionFact: "version",
		store:       [...]string{"bigint", "bit", "smallint", "smallint", "int", "bigint", "nvarchar(max)", "nvarchar(16)", "varbinary(max)"},
		native:      [...]string{"bigint", "bit", "smallint", "smallint", "int", "bigint", "nvarchar", "nvarchar", "varbinary"},
	},
	"mysql": {
		provider:    model.TypeMySQL,
		versionFact: "server_version",
		store:       [...]string{"bigint", "boolean", "tinyint", "smallint", "int", "bigint", "longtext CHARACTER SET utf8mb4", "varchar(16) CHARACTER SET utf8mb4", "longblob"},
		native:      [...]string{"bigint", "tinyint", "tinyint", "smallint", "int", "bigint", "longtext", "varchar", "longblob"},
	},
}

func TestModelTypeRoundTrip(t *testing.T) {
	if os.Getenv("GOEF_DRIVERLAB_MODEL_TYPES") != "1" {
		t.Skip("enable the model type scenario using the isolated lab runner")
	}
	provider, profile := os.Getenv("GOEF_DRIVERLAB_PROVIDER"), os.Getenv("GOEF_DRIVERLAB_PROFILE")
	target, ok := modelTypeTargets[provider]
	if !ok || profile == "" {
		t.Fatal("model type scenario requires a provider and profile")
	}

	var observedVersion string
	passed := t.Run(provider, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		t.Cleanup(cancel)
		s := scenario{fixture: providers[provider](t), t: t, provider: provider, ctx: ctx}
		s.writer = s.lease()
		if provider != "sqlite" {
			s.verifyMarker(ctx, s.writer.QueryContext)
		}
		columnNames := createModelTypeFixture(&s, target)
		verifyModelTypeCatalog(&s, target)
		verifyModelTypeValues(&s, columnNames)
		observedVersion = s.facts[target.versionFact]
		if observedVersion == "" {
			t.Fatal("model type fixture did not observe an engine version")
		}
	})
	if !passed || observedVersion == "" {
		return
	}
	result, err := json.Marshal(map[string]string{
		"provider": provider, "profile": profile, "scenario": "MODEL-TYPE-001-S02",
		"engine_version": observedVersion, "status": "PASS",
	})
	if err != nil {
		t.Fatal("could not encode model type evidence")
	}
	fmt.Printf("GOEF_MODEL_TYPES_RESULT=%s\n", result)
}

func createModelTypeFixture(s *scenario, target modelTypeTarget) []string {
	s.t.Helper()
	options := []model.MappingOption{model.ToTable(modelTypeTable)}
	for _, col := range modelTypeColumns {
		options = append(options, model.ColumnName(col.field, col.column))
	}
	mapped, err := model.Map(model.Struct[modelTypeFixture]("ModelTypeProbe", []string{"ID"}, options...))
	if err != nil {
		s.t.Fatal("could not map the model type fixture")
	}
	bound, err := model.BindTypes(mapped, target.provider)
	if err != nil {
		s.t.Fatal("could not bind the model type fixture")
	}
	entities := bound.Entities()
	if len(entities) != 1 || entities[0].TableName != modelTypeTable ||
		len(entities[0].Properties) != len(modelTypeColumns) ||
		len(entities[0].PrimaryKey) != 1 || entities[0].PrimaryKey[0] != "ID" {
		s.t.Fatal("the bound entity shape or key is wrong")
	}

	names := make([]string, 0, len(modelTypeColumns))
	definitions := make([]string, 0, len(modelTypeColumns)+1)
	for i, prop := range entities[0].Properties {
		want := modelTypeColumns[i]
		// Verify fixed names and trusted type strings before forming the test DDL.
		if prop.Name != want.field || prop.ColumnName != want.column || prop.StoreType != target.store[i] ||
			prop.Nullable != want.nullable || prop.MaxLength != want.maxLength {
			s.t.Fatalf("wrong bound metadata for fixture field %s", want.field)
		}
		definition := prop.ColumnName + " " + prop.StoreType
		if !prop.Nullable {
			definition += " NOT NULL"
		}
		definitions = append(definitions, definition)
		names = append(names, prop.ColumnName)
	}
	definitions = append(definitions, "PRIMARY KEY ("+names[0]+")")
	s.createTable("CREATE TABLE "+modelTypeTable+" ("+strings.Join(definitions, ", ")+")", "DROP TABLE "+modelTypeTable)
	return names
}

func verifyModelTypeCatalog(s *scenario, target modelTypeTarget) {
	s.t.Helper()
	var query string
	switch s.provider {
	case "sqlite":
		query = `SELECT name, lower(type), CASE "notnull" WHEN 0 THEN 'YES' ELSE 'NO' END, 0
			FROM pragma_table_info(?1) ORDER BY cid`
	case "sqlserver":
		// The fixture uses case-sensitive Latin1_General_100_BIN2.
		query = "SELECT COLUMN_NAME, DATA_TYPE, IS_NULLABLE, COALESCE(CHARACTER_MAXIMUM_LENGTH, 0) " +
			"FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = SCHEMA_NAME() " +
			"AND TABLE_NAME = " + s.bind(1) + " ORDER BY ORDINAL_POSITION"
	default:
		// The provisioning fixtures select and verify the target database/schema.
		schemas := map[string]string{"postgres": "current_schema()", "mysql": "DATABASE()"}
		query = "SELECT column_name, data_type, is_nullable, COALESCE(character_maximum_length, 0) " +
			"FROM information_schema.columns WHERE table_schema = " + schemas[s.provider] +
			" AND table_name = " + s.bind(1) + " ORDER BY ordinal_position"
	}
	rows, err := s.writer.QueryContext(s.ctx, query, modelTypeTable)
	if err != nil {
		s.t.Fatalf("could not inspect the model type catalog (native code %q)", s.code(err))
	}
	defer rows.Close()
	for i, want := range modelTypeColumns {
		var name, typ, nullable string
		var maxLength int64
		if !rows.Next() || rows.Scan(&name, &typ, &nullable, &maxLength) != nil {
			s.t.Fatalf("could not read catalog metadata for fixture field %s", want.field)
		}
		wantNullable := "NO"
		if want.nullable {
			wantNullable = "YES"
		}
		if name != want.column || typ != target.native[i] || nullable != wantNullable {
			s.t.Fatalf("wrong native type, name or nullability for fixture field %s", want.field)
		}
		if want.maxLength != 0 && s.provider != "sqlite" && maxLength != int64(want.maxLength) {
			s.t.Fatalf("wrong native character length for fixture field %s", want.field)
		}
	}
	if rows.Next() || rows.Err() != nil {
		s.t.Fatal("unexpected extra catalog rows or read failure")
	}
}

func verifyModelTypeValues(s *scenario, columnNames []string) {
	s.t.Helper()
	type values struct {
		id     int64
		active bool
		tiny   int8
		small  int16
		number int32
		native int
		text   sql.NullString
		label  sql.NullString
		binary []byte
	}
	fixtures := []values{
		{math.MinInt64, true, math.MinInt8, math.MinInt16, math.MinInt32, math.MinInt, sql.NullString{},
			sql.NullString{String: "\u0416\u6c34", Valid: true}, []byte{0, 0xff, 0x80}},
		{0, false, 0, 0, 0, 0, sql.NullString{String: "\u0427\u0438\u0441\u043b\u043e", Valid: true},
			sql.NullString{}, []byte{0xff, 0x7f}},
		{math.MaxInt64, false, math.MaxInt8, math.MaxInt16, math.MaxInt32, math.MaxInt, sql.NullString{Valid: true},
			sql.NullString{String: "1234567890abcdef", Valid: true}, []byte{0x7f, 0}},
	}
	columns := strings.Join(columnNames, ", ")
	bindings := make([]string, len(columnNames))
	for i := range bindings {
		bindings[i] = s.bind(i + 1)
	}
	insert := "INSERT INTO " + modelTypeTable + " (" + columns + ") VALUES (" + strings.Join(bindings, ", ") + ")"
	for i, want := range fixtures {
		var tiny any = want.tiny
		if s.provider == "sqlserver" {
			// go-mssqldb sends int8 as a one-byte TDS integer, which SQL Server
			// treats as unsigned. Match the bound signed smallint column.
			tiny = int16(want.tiny)
		}
		if _, err := s.writer.ExecContext(s.ctx, insert, want.id, want.active, tiny, want.small, want.number, want.native, want.text, want.label, want.binary); err != nil {
			s.t.Fatalf("could not write model type row %d", i+1)
		}
	}
	rows, err := s.writer.QueryContext(s.ctx, "SELECT "+columns+" FROM "+modelTypeTable+" ORDER BY "+columnNames[0])
	if err != nil {
		s.t.Fatal("could not query model type rows")
	}
	defer rows.Close()
	for i, want := range fixtures {
		var got values
		if !rows.Next() || rows.Scan(&got.id, &got.active, &got.tiny, &got.small, &got.number, &got.native, &got.text, &got.label, &got.binary) != nil {
			s.t.Fatalf("could not read model type row %d", i+1)
		}
		if got.id != want.id || got.active != want.active || got.tiny != want.tiny || got.small != want.small ||
			got.number != want.number || got.native != want.native || got.text != want.text || got.label != want.label || !bytes.Equal(got.binary, want.binary) {
			s.t.Fatalf("model type round trip changed fixture row %d", i+1)
		}
	}
	if rows.Next() || rows.Err() != nil {
		s.t.Fatal("unexpected extra model type rows or read failure")
	}
}
