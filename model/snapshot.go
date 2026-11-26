package model

import (
	"encoding/json"
	"slices"
	"strings"
	"unicode/utf8"
)

type snapshotProperty struct {
	Name     string `json:"name"`
	Nullable bool   `json:"nullable"`
}

type snapshotEntity struct {
	Name       string             `json:"name"`
	Properties []snapshotProperty `json:"properties"`
	PrimaryKey []string           `json:"primaryKey"`
}

// canonicalSnapshot encodes resolved declarations in a trial JSON format.
// Entity registration order is irrelevant. Properties and key members retain
// declaration order.
func canonicalSnapshot(decl Declaration) ([]byte, error) {
	if err := Validate(decl); err != nil {
		return nil, err
	}

	snapshot := struct {
		Version  int              `json:"version"`
		Entities []snapshotEntity `json:"entities"`
	}{
		Version:  1,
		Entities: make([]snapshotEntity, len(decl.Entities)),
	}
	for index, entity := range decl.Entities {
		if !utf8.ValidString(entity.Name) {
			return nil, invalidDeclaration(index+1, 0, 0, "invalid UTF-8 entity identity")
		}
		encoded := snapshotEntity{
			Name:       entity.Name,
			Properties: make([]snapshotProperty, len(entity.Properties)),
			PrimaryKey: make([]string, len(entity.PrimaryKey)),
		}
		for propertyIndex, property := range entity.Properties {
			// JSON substitutes replacement characters for invalid UTF-8.
			if !utf8.ValidString(property.Name) {
				return nil, invalidDeclaration(index+1, propertyIndex+1, 0, "invalid UTF-8 property identity")
			}
			encoded.Properties[propertyIndex] = snapshotProperty{
				Name:     property.Name,
				Nullable: property.Nullable,
			}
		}
		copy(encoded.PrimaryKey, entity.PrimaryKey)
		snapshot.Entities[index] = encoded
	}
	slices.SortFunc(snapshot.Entities, func(left, right snapshotEntity) int {
		return strings.Compare(left.Name, right.Name)
	})
	return json.Marshal(snapshot)
}
