package provider_test

import (
	"strings"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/provider"
)

func TestCompatibility(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		desc provider.Descriptor
		want bool
	}{
		{name: "v1", desc: provider.Descriptor{ID: "test", Major: provider.SPIMajor}, want: true},
		{name: "empty ID", desc: provider.Descriptor{Major: provider.SPIMajor}},
		{name: "missing version", desc: provider.Descriptor{ID: "test"}},
		{name: "unsupported version", desc: provider.Descriptor{ID: "test", Major: provider.SPIMajor + 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.desc.Compatible(); got != tc.want {
				t.Errorf("Compatible() = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestPortableValues(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		value provider.Value
		kind  provider.Kind
	}{
		{value: provider.Null{}, kind: provider.KindNull},
		{value: provider.Integer(0), kind: provider.KindInteger},
		{value: provider.Float(0), kind: provider.KindFloat},
		{value: provider.Boolean(false), kind: provider.KindBoolean},
		{value: provider.Text(""), kind: provider.KindText},
		{value: provider.Binary(nil), kind: provider.KindBinary},
		{value: provider.Timestamp{}, kind: provider.KindTimestamp},
	} {
		if got := tc.value.Kind(); got != tc.kind {
			t.Errorf("%T.Kind() = %v, want %v", tc.value, got, tc.kind)
		}
	}
}

func TestValidateBindingsAcceptsTransportValues(t *testing.T) {
	t.Parallel()

	command := provider.Command{Slots: []provider.Slot{
		{Kind: provider.KindInteger},
		{Kind: provider.KindFloat},
		{Kind: provider.KindBoolean},
		{Kind: provider.KindText},
		{Kind: provider.KindBinary},
		{Kind: provider.KindBinary},
		{Kind: provider.KindTimestamp},
		{Kind: provider.KindText, Nullable: true},
	}}
	bindings := provider.Bindings{
		provider.Integer(-42),
		provider.Float(1.25),
		provider.Boolean(true),
		provider.Text("yak"),
		provider.Binary{7, 9},
		provider.Binary(nil),
		provider.Timestamp(time.Date(2026, time.November, 26, 9, 0, 0, 0, time.UTC)),
		provider.Null{},
	}
	if err := command.ValidateBindings(bindings); err != nil {
		t.Fatalf("ValidateBindings() rejected a valid ordered layout: %v", err)
	}
	if err := (provider.Command{}).ValidateBindings(nil); err != nil {
		t.Fatalf("ValidateBindings(nil) rejected a parameterless command: %v", err)
	}
}

func TestValidateBindingsRejectsInvalidLayoutWithSafeDiagnostics(t *testing.T) {
	t.Parallel()

	const (
		sql      = "synthetic SQL marker"
		slotName = "synthetic slot marker"
		payload  = "synthetic binding marker"
	)
	text := provider.Text(payload)
	type wrappedText struct{ provider.Text }
	textSlot := provider.Slot{Name: slotName, Kind: provider.KindText}
	nullableText := provider.Slot{Name: slotName, Kind: provider.KindText, Nullable: true}

	for _, tc := range []struct {
		name     string
		slot     provider.Slot
		bindings provider.Bindings
		message  string
	}{
		{"too few values", textSlot, provider.Bindings{provider.Integer(1)}, "got 1 values for 2 slots"},
		{"too many values", textSlot, provider.Bindings{provider.Integer(1), text, text}, "got 3 values for 2 slots"},
		{"invalid null slot kind", provider.Slot{Name: slotName, Kind: provider.KindNull, Nullable: true}, provider.Bindings{provider.Integer(1), provider.Null{}}, "slot 2 has an invalid kind"},
		{"unknown slot kind", provider.Slot{Name: slotName, Kind: provider.KindTimestamp + 1}, provider.Bindings{provider.Integer(1), text}, "slot 2 has an invalid kind"},
		{"null in required slot", textSlot, provider.Bindings{provider.Integer(1), provider.Null{}}, "slot 2 does not accept null"},
		{"out of order", textSlot, provider.Bindings{text, provider.Integer(1)}, "slot 1 has the wrong value kind"},
		{"wrong concrete kind", provider.Slot{Name: slotName, Kind: provider.KindBoolean}, provider.Bindings{provider.Integer(1), text}, "slot 2 has the wrong value kind"},
		{"nil interface", nullableText, provider.Bindings{provider.Integer(1), nil}, "slot 2 has an unsupported value type"},
		{"typed nil pointer", nullableText, provider.Bindings{provider.Integer(1), (*provider.Text)(nil)}, "slot 2 has an unsupported value type"},
		{"non-nil pointer", textSlot, provider.Bindings{provider.Integer(1), &text}, "slot 2 has an unsupported value type"},
		{"embedded transport value", textSlot, provider.Bindings{provider.Integer(1), wrappedText{text}}, "slot 2 has an unsupported value type"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := provider.Command{
				SQL:   sql,
				Slots: []provider.Slot{{Kind: provider.KindInteger}, tc.slot},
			}
			err := command.ValidateBindings(tc.bindings)
			if err == nil {
				t.Fatal("ValidateBindings() accepted an invalid layout")
			}
			message := err.Error()
			if !strings.Contains(message, tc.message) {
				t.Errorf("ValidateBindings() error = %q, want %q", message, tc.message)
			}
			for _, private := range []string{sql, slotName, payload} {
				if strings.Contains(message, private) {
					t.Errorf("ValidateBindings() error exposes %q: %q", private, message)
				}
			}
		})
	}
}
