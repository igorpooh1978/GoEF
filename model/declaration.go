// Package model validates resolved logical model declarations.
package model

import (
	"fmt"
	"strings"

	"github.com/igorpooh1978/GoEF/goeferr"
)

// Declaration contains entities in declaration order.
type Declaration struct {
	Entities []EntityDecl
}

// EntityDecl identifies an entity and its resolved properties.
// PrimaryKey lists exact property names in key order. An empty list declares no key.
type EntityDecl struct {
	Name       string
	Properties []PropertyDecl
	PrimaryKey []string
}

// PropertyDecl identifies a property and its effective nullability.
type PropertyDecl struct {
	Name     string
	Nullable bool
}

// ValidationError locates the first invalid declaration. Positions are one-based.
// A zero property or key member position means the failure has no such position.
type ValidationError struct {
	EntityPosition    int
	PropertyPosition  int
	KeyMemberPosition int
	reason            string
	cause             error
}

// Error reports the declaration positions without including names.
func (e *ValidationError) Error() string {
	if e == nil || e.EntityPosition <= 0 {
		return "invalid model"
	}
	location := fmt.Sprintf("entity %d", e.EntityPosition)
	if e.PropertyPosition != 0 {
		location += fmt.Sprintf(", property %d", e.PropertyPosition)
	}
	if e.KeyMemberPosition != 0 {
		location += fmt.Sprintf(", primary key member %d", e.KeyMemberPosition)
	}
	return fmt.Sprintf("invalid model at %s (%s)", location, e.reason)
}

// Unwrap returns the portable invalid-model cause, if one was recorded.
func (e *ValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Validate checks entity identities, property identities and declared primary keys
// in declaration order without modifying the declaration or accessing a provider.
// Names are case sensitive. An absent key makes no assertion about trackability.
func Validate(declaration Declaration) error {
	names := make(map[string]bool, len(declaration.Entities))
	for index, entity := range declaration.Entities {
		position := index + 1
		if strings.TrimSpace(entity.Name) == "" {
			return invalidDeclaration(position, 0, 0, "empty entity identity")
		}
		if names[entity.Name] {
			return invalidDeclaration(position, 0, 0, "duplicate entity identity")
		}
		names[entity.Name] = true

		if err := validateEntity(position, entity); err != nil {
			return err
		}
	}
	return nil
}

func validateEntity(entityPosition int, entity EntityDecl) error {
	properties := make(map[string]int, len(entity.Properties))
	for index, property := range entity.Properties {
		position := index + 1
		if strings.TrimSpace(property.Name) == "" {
			return invalidDeclaration(entityPosition, position, 0, "empty property identity")
		}
		if properties[property.Name] != 0 {
			return invalidDeclaration(entityPosition, position, 0, "duplicate property identity")
		}
		properties[property.Name] = position
	}

	members := make(map[int]bool, len(entity.PrimaryKey))
	for index, name := range entity.PrimaryKey {
		keyPosition := index + 1
		propertyPosition := properties[name]
		if propertyPosition == 0 {
			return invalidDeclaration(entityPosition, 0, keyPosition, "unresolved primary key member")
		}
		if members[propertyPosition] {
			return invalidDeclaration(entityPosition, propertyPosition, keyPosition, "duplicate primary key member")
		}
		if entity.Properties[propertyPosition-1].Nullable {
			return invalidDeclaration(entityPosition, propertyPosition, keyPosition, "nullable primary key property")
		}
		members[propertyPosition] = true
	}
	return nil
}

func invalidDeclaration(entity, property, keyMember int, reason string) *ValidationError {
	return &ValidationError{
		EntityPosition:    entity,
		PropertyPosition:  property,
		KeyMemberPosition: keyMember,
		reason:            reason,
		cause:             goeferr.NewInvalidModel(),
	}
}
