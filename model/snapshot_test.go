package model

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/igorpooh1978/GoEF/goeferr"
)

func TestCanonicalSnapshotStableBytesAndOwnership(t *testing.T) {
	t.Parallel()

	const want = `{"version":1,"entities":[{"name":"catalog","properties":[],"primaryKey":[]},` +
		`{"name":"order","properties":[{"name":"TenantID","nullable":false},{"name":"Note","nullable":true},{"name":"ID","nullable":false}],` +
		`"primaryKey":["ID","TenantID"]}]}`
	original := snapshotFixture()
	reordered := Declaration{Entities: []EntityDecl{original.Entities[1], original.Entities[0]}}

	first, err := canonicalSnapshot(original)
	if err != nil || string(first) != want {
		t.Fatalf("unexpected trial snapshot %s, error %v", first, err)
	}
	second, err := canonicalSnapshot(reordered)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatal("entity registration order changed the snapshot bytes")
	}
	if !reflect.DeepEqual(original, snapshotFixture()) {
		t.Fatal("snapshot changed the input declaration or nested slices")
	}

	// Both declarations share nested slices, while encoding results are independent.
	original.Entities[0].Properties[0].Name = "edited property"
	original.Entities[0].PrimaryKey[0] = "edited key"
	second[0] = '!'
	if string(first) != want {
		t.Fatal("editing input or another result changed the original bytes")
	}
}

func TestCanonicalSnapshotRetainsPropertyAndKeyOrder(t *testing.T) {
	t.Parallel()

	original, err := canonicalSnapshot(snapshotFixture())
	if err != nil {
		t.Fatalf("snapshot of fixture failed: %v", err)
	}
	for _, test := range []struct {
		name    string
		reorder func(*EntityDecl)
	}{
		{"properties", func(entity *EntityDecl) {
			entity.Properties[0], entity.Properties[2] = entity.Properties[2], entity.Properties[0]
		}},
		{"primary key", func(entity *EntityDecl) {
			entity.PrimaryKey[0], entity.PrimaryKey[1] = entity.PrimaryKey[1], entity.PrimaryKey[0]
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			decl := snapshotFixture()
			test.reorder(&decl.Entities[0])
			reordered, err := canonicalSnapshot(decl)
			if err != nil || bytes.Equal(original, reordered) {
				t.Fatal("declared order was lost in the snapshot")
			}
		})
	}
}

func TestCanonicalSnapshotRejectsInvalidDeclarations(t *testing.T) {
	t.Parallel()

	const invalidName = "private-\xff"
	for _, test := range []struct {
		name     string
		change   func(*EntityDecl)
		property int
		key      int
	}{
		{"nullable key", func(entity *EntityDecl) {
			entity.PrimaryKey[0] = "Note"
		}, 2, 1},
		{"invalid UTF-8 entity", func(entity *EntityDecl) {
			entity.Name = invalidName
		}, 0, 0},
		{"invalid UTF-8 property", func(entity *EntityDecl) {
			entity.Properties[2].Name = invalidName
			entity.PrimaryKey[0] = invalidName
		}, 3, 0},
		{"invalid UTF-8 key reference", func(entity *EntityDecl) {
			entity.PrimaryKey[0] = invalidName
		}, 0, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			decl := snapshotFixture()
			test.change(&decl.Entities[0])
			got, err := canonicalSnapshot(decl)
			if got != nil || !errors.Is(err, goeferr.ErrInvalidModel) {
				t.Fatal("expected only an invalid-model error")
			}
			var detail *ValidationError
			if !errors.As(err, &detail) || detail.EntityPosition != 1 ||
				detail.PropertyPosition != test.property || detail.KeyMemberPosition != test.key {
				t.Fatal("snapshot failure did not identify the original declaration position")
			}
			if strings.Contains(err.Error(), "private") {
				t.Fatal("snapshot failure disclosed an identity")
			}
		})
	}
}

func TestCanonicalSnapshotNormalizesEmptyLists(t *testing.T) {
	t.Parallel()

	for _, decl := range []Declaration{{}, {Entities: []EntityDecl{}}} {
		got, err := canonicalSnapshot(decl)
		if err != nil || string(got) != `{"version":1,"entities":[]}` {
			t.Fatalf("unexpected empty snapshot %s, error %v", got, err)
		}
	}
}

func snapshotFixture() Declaration {
	return Declaration{Entities: []EntityDecl{
		{
			Name: "order",
			Properties: []PropertyDecl{
				{Name: "TenantID"},
				{Name: "Note", Nullable: true},
				{Name: "ID"},
			},
			PrimaryKey: []string{"ID", "TenantID"},
		},
		{Name: "catalog"},
	}}
}
