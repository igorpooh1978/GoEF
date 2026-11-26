package driverlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/model"
)

const (
	decimalBindingTable   = "goef_decimal_binding_probe"
	decimalProbePrecision = 38
	decimalProbeScale     = 9
)

type decimalBindingCase struct {
	input, integral string
	fixed, whole    string
}

func TestDecimalBindingRoundTrip(t *testing.T) {
	if os.Getenv("GOEF_DECIMAL_BINDING") != "1" {
		t.Skip("enable the decimal binding scenario using the isolated lab runner")
	}
	provider, profile := os.Getenv("GOEF_DRIVERLAB_PROVIDER"), os.Getenv("GOEF_DRIVERLAB_PROFILE")
	target, ok := modelTypeTargets[provider]
	if !ok || profile == "" {
		t.Fatal("decimal binding requires a provider and profile")
	}

	var observedVersion string
	passed := t.Run(provider, func(t *testing.T) {
		fractional, integral := decimalBindingFacets(t, target.provider)
		inputs := decimalBindingInputs()
		encoded := encodeDecimalFixture(t, fractional, integral, inputs)
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		t.Cleanup(cancel)
		s := scenario{fixture: providers[provider](t), t: t, provider: provider, ctx: ctx}
		s.writer = s.lease()
		if provider != "sqlite" {
			s.verifyMarker(ctx, s.writer.QueryContext)
		}
		s.observer = s.lease()
		if provider != "sqlite" {
			s.verifyMarker(ctx, s.observer.QueryContext)
		}

		// These declarations have been validated against fixed fixture strings.
		create := "CREATE TABLE " + decimalBindingTable + " (id INTEGER NOT NULL PRIMARY KEY, amount " +
			fractional.StoreType() + " NOT NULL, whole_amount " + integral.StoreType() + " NOT NULL)"
		if provider == "mysql" {
			create += " ENGINE=InnoDB"
		}
		s.createTable(create, "DROP TABLE "+decimalBindingTable)

		verifyDecimalBindingCatalog(&s)
		insert := "INSERT INTO " + decimalBindingTable + " (id, amount, whole_amount) VALUES (" +
			s.bind(1) + ", " + decimalParameter(provider, s.bind(2), fractional.StoreType()) + ", " +
			decimalParameter(provider, s.bind(3), integral.StoreType()) + ")"
		for i, row := range encoded {
			result, err := s.writer.ExecContext(ctx, insert, i+1, row[0], row[1])
			if err != nil {
				t.Fatalf("decimal write %d failed (native code %q)", i+1, s.code(err))
			}
			if affected, err := result.RowsAffected(); err != nil || affected != 1 {
				t.Fatalf("decimal write %d did not affect one row", i+1)
			}
		}
		verifyDecimalBindingValues(&s, fractional, integral, inputs)
		observedVersion = s.facts[target.versionFact]
		if observedVersion == "" {
			t.Fatal("decimal fixture did not observe an engine version")
		}
	})
	if !passed || observedVersion == "" {
		return
	}
	result, err := json.Marshal(map[string]string{
		"provider": provider, "profile": profile, "scenario": "MODEL-TYPE-001-S01",
		"engine_version": observedVersion, "status": "PASS",
	})
	if err != nil {
		t.Fatal("could not encode decimal binding evidence")
	}
	fmt.Printf("GOEF_DECIMAL_BINDING_RESULT=%s\n", result)
}

func decimalBindingFacets(t *testing.T, provider model.TypeProvider) (model.DecimalBinding, model.DecimalBinding) {
	t.Helper()
	invalid := [][2]int{{0, 0}, {1, -1}, {1, 2}, {model.MaxDecimalDigits + 1, 0}}
	switch provider {
	case model.TypeSQLServer:
		invalid = append(invalid, [2]int{39, 0})
	case model.TypeMySQL:
		invalid = append(invalid, [2]int{66, 0}, [2]int{38, 31})
	}
	for _, facets := range invalid {
		binding, err := model.BindDecimal(provider, facets[0], facets[1])
		if !errors.Is(err, goeferr.ErrInvalidModel) || binding.StoreType() != "" {
			t.Fatalf("unsupported decimal precision %d scale %d produced a binding or no typed error", facets[0], facets[1])
		}
	}
	fractional := requireDecimalBinding(t, provider, decimalProbeScale)
	integral := requireDecimalBinding(t, provider, 0)
	for _, tc := range []struct {
		binding model.DecimalBinding
		text    string
	}{
		{fractional, "1" + strings.Repeat("0", decimalProbePrecision-decimalProbeScale)},
		{fractional, "0.0000000001"},
		{integral, "0.1"},
		{integral, "1" + strings.Repeat("0", decimalProbePrecision)},
	} {
		value, err := model.ParseDecimal(tc.text)
		if err != nil {
			t.Fatal("invalid decimal boundary fixture")
		}
		if text, err := tc.binding.Encode(value); err == nil || text != "" {
			t.Fatal("out-of-range decimal encoded before dispatch")
		}
	}
	return fractional, integral
}

func requireDecimalBinding(t *testing.T, provider model.TypeProvider, scale int) model.DecimalBinding {
	t.Helper()
	binding, err := model.BindDecimal(provider, decimalProbePrecision, scale)
	if err != nil {
		t.Fatal("could not bind supported decimal facets")
	}
	want := fmt.Sprintf("decimal(%d,%d)", decimalProbePrecision, scale)
	switch provider {
	case model.TypePostgreSQL:
		want = fmt.Sprintf("numeric(%d,%d)", decimalProbePrecision, scale)
	case model.TypeSQLite:
		want = "text"
	}
	if got := strings.ReplaceAll(strings.ToLower(binding.StoreType()), " ", ""); got != want {
		t.Fatalf("decimal scale %d bound the wrong storage type", scale)
	}
	return binding
}

func decimalBindingInputs() []decimalBindingCase {
	allDigits := strings.Repeat("9", decimalProbePrecision)
	limit := allDigits[:decimalProbePrecision-decimalProbeScale] + "." + allDigits[decimalProbePrecision-decimalProbeScale:]
	return []decimalBindingCase{
		{"9007199254740993.123456789", "9007199254740993", "9007199254740993.123456789", "9007199254740993"},
		{"-9007199254740993.987654321", "-9007199254740993", "-9007199254740993.987654321", "-9007199254740993"},
		{limit, allDigits, limit, allDigits},
		{"-" + limit, "-" + allDigits, "-" + limit, "-" + allDigits},
		{"-0.0", "-0", "0.000000000", "0"},
		{"1.20", "+00012", "1.200000000", "12"},
		{"0.000000001", "1", "0.000000001", "1"},
	}
}

func encodeDecimalFixture(t *testing.T, fractional, integral model.DecimalBinding, cases []decimalBindingCase) [][2]string {
	t.Helper()
	encoded := make([][2]string, len(cases))
	for i, tc := range cases {
		for j, binding := range [2]model.DecimalBinding{fractional, integral} {
			input := [2]string{tc.input, tc.integral}[j]
			want := [2]string{tc.fixed, tc.whole}[j]
			value, err := model.ParseDecimal(input)
			if err != nil {
				t.Fatalf("could not parse fixture decimal %d", i+1)
			}
			text, err := binding.Encode(value)
			if err != nil || text != want {
				t.Fatalf("could not encode fixture decimal %d at fixed scale", i+1)
			}
			encoded[i][j] = text
		}
	}
	return encoded
}

func decimalParameter(provider, placeholder, storeType string) string {
	if provider == "sqlite" {
		return placeholder
	}
	if provider == "postgres" {
		// Tell pgx the parameter is text so it never enters a numeric encoder.
		placeholder = "CAST(" + placeholder + " AS text)"
	}
	return "CAST(" + placeholder + " AS " + storeType + ")"
}

func verifyDecimalBindingCatalog(s *scenario) {
	s.t.Helper()
	var query, native string
	switch s.provider {
	case "sqlite":
		query = `SELECT name, lower(type), 0, 0 FROM pragma_table_info(?1)
			WHERE name = 'amount' OR name = 'whole_amount' ORDER BY cid`
		native = "text"
	case "sqlserver":
		query = "SELECT COLUMN_NAME, DATA_TYPE, NUMERIC_PRECISION, NUMERIC_SCALE FROM INFORMATION_SCHEMA.COLUMNS " +
			"WHERE TABLE_SCHEMA = SCHEMA_NAME() AND TABLE_NAME = " + s.bind(1) +
			" AND (COLUMN_NAME = 'amount' OR COLUMN_NAME = 'whole_amount') ORDER BY ORDINAL_POSITION"
		native = "decimal"
	default:
		schema := map[string]string{"postgres": "current_schema()", "mysql": "DATABASE()"}
		query = "SELECT column_name, data_type, numeric_precision, numeric_scale FROM information_schema.columns " +
			"WHERE table_schema = " + schema[s.provider] + " AND table_name = " + s.bind(1) +
			" AND (column_name = 'amount' OR column_name = 'whole_amount') ORDER BY ordinal_position"
		native = "decimal"
		if s.provider == "postgres" {
			native = "numeric"
		}
	}
	rows, err := s.observer.QueryContext(s.ctx, query, decimalBindingTable)
	if err != nil {
		s.t.Fatalf("could not inspect decimal catalog (native code %q)", s.code(err))
	}
	defer rows.Close()
	for i, column := range [...]string{"amount", "whole_amount"} {
		var name, store string
		var precision, scale int
		wantPrecision, wantScale := decimalProbePrecision, [2]int{decimalProbeScale, 0}[i]
		if s.provider == "sqlite" {
			// SQLite has TEXT affinity. The encoding boundary supplies facets.
			wantPrecision, wantScale = 0, 0
		}
		if !rows.Next() || rows.Scan(&name, &store, &precision, &scale) != nil ||
			name != column || store != native || precision != wantPrecision || scale != wantScale {
			s.t.Fatalf("catalog declaration differs for decimal fixture column %s", column)
		}
	}
	if rows.Next() || rows.Err() != nil {
		s.t.Fatal("unexpected extra decimal catalog rows or read failure")
	}
}

func verifyDecimalBindingValues(s *scenario, fractional, integral model.DecimalBinding, cases []decimalBindingCase) {
	s.t.Helper()
	convertToText := map[string]string{"postgres": "text", "sqlserver": "varchar(80)", "mysql": "CHAR"}
	columns := "amount, whole_amount, typeof(amount), typeof(whole_amount)"
	if s.provider != "sqlite" {
		typ := convertToText[s.provider]
		columns = "CAST(amount AS " + typ + "), CAST(whole_amount AS " + typ + ")"
	}
	query := "SELECT id, " + columns + " FROM " + decimalBindingTable + " ORDER BY id"
	rows, err := s.observer.QueryContext(s.ctx, query)
	if err != nil {
		s.t.Fatalf("could not read decimal values as text (native code %q)", s.code(err))
	}
	defer rows.Close()
	for i, tc := range cases {
		var id int
		var amount, whole, storedAmount, storedWhole string
		dest := []any{&id, &amount, &whole}
		if s.provider == "sqlite" {
			dest = append(dest, &storedAmount, &storedWhole)
		}
		if !rows.Next() || rows.Scan(dest...) != nil || id != i+1 || amount != tc.fixed || whole != tc.whole {
			s.t.Fatalf("decimal row %d changed during storage or text conversion", i+1)
		}
		if s.provider == "sqlite" && (storedAmount != "text" || storedWhole != "text") {
			s.t.Fatalf("SQLite decimal row %d is not stored as text", i+1)
		}
		for j, binding := range [2]model.DecimalBinding{fractional, integral} {
			raw := [2]string{amount, whole}[j]
			wantScale := [2]int{decimalProbeScale, 0}[j]
			decoded, err := binding.Decode(raw)
			if err != nil || decoded.String() != raw || decoded.Scale() != wantScale {
				s.t.Fatalf("could not decode decimal fixture row %d with its exact scale", i+1)
			}
		}
	}
	if rows.Next() || rows.Err() != nil {
		s.t.Fatal("unexpected extra decimal value rows or read failure")
	}
}
