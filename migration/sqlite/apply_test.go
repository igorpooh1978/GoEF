package sqlite_test

import (
	"errors"
	"testing"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/migration/sqlite"
)

func TestApplyRejectsNilPool(t *testing.T) {
	applied, err := sqlite.Apply(t.Context(), nil, sqlite.Plan{})
	if applied || !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("Apply = %t, %v; want invalid state", applied, err)
	}
}
