package provider_test

import (
	"testing"

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
