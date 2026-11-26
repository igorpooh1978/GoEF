package model_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/model"
)

func TestValidationErrorFallback(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		detail *model.ValidationError
	}{
		{name: "nil"},
		{name: "zero", detail: &model.ValidationError{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var err error = test.detail
			if errors.Is(err, goeferr.ErrInvalidModel) || errors.Unwrap(err) != nil {
				t.Fatal("validation error without a declaration reported an invalid-model cause")
			}
			var detail *model.ValidationError
			if !errors.As(err, &detail) || detail != test.detail {
				t.Fatal("error did not retain its validation error type")
			}
			if got := err.Error(); got != "invalid model" {
				t.Fatalf("expected the generic validation diagnostic, got %q", got)
			}
		})
	}
}

func TestValidateAcceptsRequiredCompositeKeyAndOptionalNonKey(t *testing.T) {
	t.Parallel()

	entity := model.EntityDecl{
		Name: "order",
		Properties: []model.PropertyDecl{
			{Name: "ID"},
			{Name: "Note", Nullable: true},
			{Name: "TenantID"},
		},
		PrimaryKey: []string{"TenantID", "ID"},
	}
	other := model.EntityDecl{
		Name:       "customer",
		Properties: []model.PropertyDecl{{Name: "ID"}},
		PrimaryKey: []string{"ID"},
	}
	undeclaredKey := model.EntityDecl{
		Name:       "summary",
		Properties: []model.PropertyDecl{{Name: "Total"}},
	}
	if err := model.Validate(model.Declaration{Entities: []model.EntityDecl{entity, other, undeclaredKey}}); err != nil {
		t.Fatalf("valid model failed validation: %v", err)
	}
}

func TestValidateRejectsInvalidDeclarations(t *testing.T) {
	t.Parallel()

	const privateName, privateProperty = "private-model-name", "private-model-property"
	for _, test := range []struct {
		name              string
		change            func(*model.Declaration)
		entityPosition    int
		propertyPosition  int
		keyMemberPosition int
	}{
		{
			name: "empty entity identity",
			change: func(decl *model.Declaration) {
				decl.Entities[0].Name = " "
			},
			entityPosition: 1,
		},
		{
			name: "duplicate entity identity",
			change: func(decl *model.Declaration) {
				decl.Entities = append(decl.Entities, decl.Entities[0])
			},
			entityPosition: 2,
		},
		{
			name: "empty property identity",
			change: func(decl *model.Declaration) {
				decl.Entities[0].Properties[1].Name = ""
			},
			entityPosition: 1, propertyPosition: 2,
		},
		{
			name: "duplicate property identity",
			change: func(decl *model.Declaration) {
				decl.Entities[0].Properties[1].Name = "ID"
			},
			entityPosition: 1, propertyPosition: 2,
		},
		{
			name: "empty primary key member",
			change: func(decl *model.Declaration) {
				decl.Entities[0].PrimaryKey = []string{"ID", ""}
			},
			entityPosition: 1, keyMemberPosition: 2,
		},
		{
			name: "unresolved primary key member",
			change: func(decl *model.Declaration) {
				decl.Entities[0].PrimaryKey = []string{"ID", privateName}
			},
			entityPosition: 1, keyMemberPosition: 2,
		},
		{
			name: "duplicate primary key member",
			change: func(decl *model.Declaration) {
				decl.Entities[0].PrimaryKey = []string{"ID", "ID"}
			},
			entityPosition: 1, propertyPosition: 1, keyMemberPosition: 2,
		},
		{
			// MODEL-VALID-001-S01 is exercised with resolved declarations only.
			name: "optional primary key property",
			change: func(decl *model.Declaration) {
				decl.Entities[0].PrimaryKey = []string{"ID", privateProperty}
			},
			entityPosition: 1, propertyPosition: 2, keyMemberPosition: 2,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			decl := model.Declaration{Entities: []model.EntityDecl{{
				Name:       privateName,
				Properties: []model.PropertyDecl{{Name: "ID"}, {Name: privateProperty, Nullable: true}},
				PrimaryKey: []string{"ID"},
			}}}
			test.change(&decl)

			err := model.Validate(decl)
			var detail *model.ValidationError
			if !errors.As(err, &detail) {
				t.Fatal("expected a model validation error")
			}
			var cause *goeferr.ErrorDetail
			if !errors.Is(err, goeferr.ErrInvalidModel) || !errors.As(err, &cause) || cause.ID() != goeferr.InvalidModelID {
				t.Fatal("model validation error did not retain the portable invalid-model cause")
			}
			if detail.EntityPosition != test.entityPosition || detail.PropertyPosition != test.propertyPosition ||
				detail.KeyMemberPosition != test.keyMemberPosition {
				t.Fatalf("unexpected declaration position (%d, %d, %d)", detail.EntityPosition, detail.PropertyPosition, detail.KeyMemberPosition)
			}
			if strings.Contains(err.Error(), privateName) || strings.Contains(err.Error(), privateProperty) {
				t.Fatal("validation diagnostic exposed a declaration name")
			}
		})
	}
}
