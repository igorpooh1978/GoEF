package model

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/igorpooh1978/GoEF/goeferr"
)

func TestPropertyFacetsApplyPrecedencePerFacet(t *testing.T) {
	t.Parallel()
	columns := []facetDeclaration[string]{
		{tagSource, facetPosition{2, 16}, "tag_name"},
		{fluentSource, facetPosition{3, 12}, "Display Name"},
		{conventionSource, facetPosition{1, 4}, "Name"},
	}
	lengths := []facetDeclaration[int]{
		{tagSource, facetPosition{2, 28}, 32},
		{conventionSource, facetPosition{1, 4}, 128},
		{tagSource, facetPosition{2, 32}, 32},
	}
	for _, test := range []struct {
		name    string
		columns []facetDeclaration[string]
		lengths []facetDeclaration[int]
		want    propertyFacets
	}{
		{"convention only", columns[2:], nil, propertyFacets{"Name", 0}},
		{"tag overrides convention", []facetDeclaration[string]{columns[0], columns[2]}, lengths, propertyFacets{"tag_name", 32}},
		{"fluent name retains tag length", columns, lengths, propertyFacets{"Display Name", 32}},
		{"fluent overrides length", columns, append(slices.Clone(lengths), facetDeclaration[int]{fluentSource, facetPosition{3, 18}, 64}), propertyFacets{"Display Name", 64}},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, l := slices.Clone(test.columns), slices.Clone(test.lengths)
			for order := range 2 {
				if order == 1 {
					slices.Reverse(c)
					slices.Reverse(l)
				}
				beforeC, beforeL := slices.Clone(c), slices.Clone(l)
				got, err := resolvePropertyFacets(c, l)
				if err != nil || got != test.want {
					t.Fatalf("order %d got %+v, %v; want %+v", order, got, err, test.want)
				}
				if !slices.Equal(c, beforeC) || !slices.Equal(l, beforeL) {
					t.Fatal("resolution mutated the input order")
				}
			}
		})
	}
}

func TestPropertyFacetConflictsRetainStablePositions(t *testing.T) {
	t.Parallel()
	first, duplicate, second := facetPosition{2, 4}, facetPosition{2, 9}, facetPosition{3, 1}
	for _, test := range []struct {
		name, facet string
		columns     []facetDeclaration[string]
		lengths     []facetDeclaration[int]
	}{
		{"shadowed column conflict", "column name", []facetDeclaration[string]{
			{fluentSource, facetPosition{4, 1}, "override"},
			{tagSource, second, "private-second"}, {tagSource, duplicate, "private-first"}, {tagSource, first, "private-first"},
		}, nil},
		{"length conflict", "maximum length", []facetDeclaration[string]{{conventionSource, facetPosition{1, 1}, "Name"}}, []facetDeclaration[int]{
			{fluentSource, second, 64}, {fluentSource, duplicate, 32}, {fluentSource, first, 32},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, l := slices.Clone(test.columns), slices.Clone(test.lengths)
			for order := range 2 {
				if order == 1 {
					slices.Reverse(c)
					slices.Reverse(l)
				}
				got, err := resolvePropertyFacets(c, l)
				var conflict *facetConflict
				var detail *goeferr.ErrorDetail
				if got != (propertyFacets{}) || !errors.As(err, &conflict) || !errors.Is(err, goeferr.ErrInvalidModel) || !errors.As(err, &detail) || detail.ID() != goeferr.InvalidModelID {
					t.Fatalf("order %d did not return a typed conflict with an empty result: %v", order, err)
				}
				wantMessage := "conflicting " + test.facet + " declarations at input 2 offset 4 and input 3 offset 1"
				if conflict.facet != test.facet || conflict.first != first || conflict.second != second || err.Error() != wantMessage {
					t.Fatalf("order %d returned inconsistent conflict detail: %v", order, err)
				}
			}
		})
	}
}

func TestPropertyFacetsRejectInvalidEffectiveValuesAndOrigins(t *testing.T) {
	t.Parallel()
	at := facetPosition{1, 2}
	valid := []facetDeclaration[string]{{conventionSource, at, "Name"}}
	for _, test := range []struct {
		name    string
		columns []facetDeclaration[string]
		lengths []facetDeclaration[int]
		want    string
	}{
		{"missing column", nil, nil, "missing column name"},
		{"blank column", []facetDeclaration[string]{{tagSource, at, " \t"}}, nil, "invalid column name at input 1 offset 2"},
		{"NUL column", []facetDeclaration[string]{{fluentSource, at, "private\x00name"}}, nil, "invalid column name at input 1 offset 2"},
		{"malformed UTF-8", []facetDeclaration[string]{{tagSource, at, "private\xff"}}, nil, "invalid column name at input 1 offset 2"},
		{"zero limit", valid, []facetDeclaration[int]{{tagSource, at, 0}}, "invalid maximum length at input 1 offset 2"},
		{"negative limit", valid, []facetDeclaration[int]{{fluentSource, at, -4}}, "invalid maximum length at input 1 offset 2"},
		{"unknown source", valid, []facetDeclaration[int]{{facetSource(8), at, 32}}, "invalid maximum length declaration origin at input 1 offset 2"},
		{"missing position", []facetDeclaration[string]{{tagSource, facetPosition{1, 0}, "private"}}, nil, "invalid column name declaration origin at input 1 offset 0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolvePropertyFacets(test.columns, test.lengths)
			if got != (propertyFacets{}) || !errors.Is(err, goeferr.ErrInvalidModel) || !strings.HasSuffix(err.Error(), test.want) || strings.Contains(err.Error(), "private") {
				t.Fatalf("unexpected result %+v, diagnostic %v", got, err)
			}
		})
	}
}
