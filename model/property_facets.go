package model

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/igorpooh1978/GoEF/goeferr"
)

type facetSource uint8

const (
	conventionSource facetSource = iota + 1
	tagSource
	fluentSource
)

// The caller assigns one-based input IDs and byte offsets before registration.
// Numeric positions keep source paths and mapping values out of diagnostics.
type facetPosition struct {
	input, offset int
}

func (p facetPosition) String() string {
	return fmt.Sprintf("input %d offset %d", p.input, p.offset)
}

type facetDeclaration[T comparable] struct {
	source facetSource
	at     facetPosition
	value  T
}

type propertyFacets struct {
	columnName string
	maxLength  int // Zero means that this property has no declared limit.
}

type facetConflict struct {
	facet         string
	first, second facetPosition
	cause         error
}

func (e *facetConflict) Error() string {
	return fmt.Sprintf("conflicting %s declarations at %s and %s", e.facet, e.first, e.second)
}

func (e *facetConflict) Unwrap() error { return e.cause }

// resolvePropertyFacets takes declarations for one property. Its input starts
// after conventions, tags and fluent calls have recorded their individual facets.
func resolvePropertyFacets(columns []facetDeclaration[string], lengths []facetDeclaration[int]) (propertyFacets, error) {
	column, err := resolveFacet("column name", columns)
	if err != nil {
		return propertyFacets{}, err
	}
	if len(columns) == 0 {
		return propertyFacets{}, fmt.Errorf("%w: missing column name", goeferr.NewInvalidModel())
	}
	if strings.TrimSpace(column.value) == "" || !utf8.ValidString(column.value) || strings.ContainsRune(column.value, 0) {
		return propertyFacets{}, fmt.Errorf("%w: invalid column name at %s", goeferr.NewInvalidModel(), column.at)
	}

	length, err := resolveFacet("maximum length", lengths)
	if err != nil {
		return propertyFacets{}, err
	}
	if len(lengths) > 0 && length.value <= 0 {
		return propertyFacets{}, fmt.Errorf("%w: invalid maximum length at %s", goeferr.NewInvalidModel(), length.at)
	}
	return propertyFacets{columnName: column.value, maxLength: length.value}, nil
}

func resolveFacet[T comparable](name string, declarations []facetDeclaration[T]) (facetDeclaration[T], error) {
	var selected facetDeclaration[T]
	// Check contradictions at every source, even when a higher source overrides
	// that facet. Source positions make the first conflicting pair reproducible.
	ordered := slices.Clone(declarations)
	slices.SortFunc(ordered, func(a, b facetDeclaration[T]) int {
		return cmp.Or(cmp.Compare(a.source, b.source), cmp.Compare(a.at.input, b.at.input), cmp.Compare(a.at.offset, b.at.offset))
	})
	for _, declaration := range ordered {
		if declaration.source < conventionSource || declaration.source > fluentSource || declaration.at.input < 1 || declaration.at.offset < 1 {
			return facetDeclaration[T]{}, fmt.Errorf("%w: invalid %s declaration origin at %s", goeferr.NewInvalidModel(), name, declaration.at)
		}
		if declaration.source != selected.source {
			selected = declaration
		} else if declaration.value != selected.value {
			return facetDeclaration[T]{}, &facetConflict{
				facet: name, first: selected.at, second: declaration.at, cause: goeferr.NewInvalidModel(),
			}
		}
	}
	return selected, nil
}
