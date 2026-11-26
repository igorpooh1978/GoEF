package migration_test

import (
	"crypto/sha256"
	"errors"
	"slices"
	"testing"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/migration"
)

func TestPendingMIG_DRIFT_001S03(t *testing.T) {
	t.Parallel()

	first := migration.Entry{ID: "001.Initial", Digest: sha256.Sum256([]byte("synthetic first artifact"))}
	second := migration.Entry{ID: " 003 ajout café ", Predecessor: first.ID, Digest: sha256.Sum256([]byte("synthetic second artifact"))}
	third := migration.Entry{ID: "002-rename", Predecessor: second.ID, Digest: sha256.Sum256([]byte("synthetic third artifact"))}
	registered := []migration.Entry{first, second, third}

	duplicate := third
	duplicate.ID = first.ID
	wrongFirst := first
	wrongFirst.Predecessor = third.ID
	wrongSuccessor := third
	wrongSuccessor.Predecessor = first.ID
	missingDigest := third
	missingDigest.Digest = [sha256.Size]byte{}
	alteredDigest := first
	alteredDigest.Digest = second.Digest
	unknown := first
	unknown.ID = "private-migration-name"
	changedCase := first
	changedCase.ID = "001.initial"
	malformed := first
	malformed.ID = " 	\u00a0"
	badEncoding := first
	badEncoding.ID = "private\xffid"
	changedSpacing := second
	changedSpacing.ID = "003 ajout café"

	tests := []struct {
		name       string
		registered []migration.Entry
		applied    []migration.Entry
		want       []migration.Entry
		conflict   bool
	}{
		{name: "empty lineage"},
		{name: "empty history", registered: registered, want: registered},
		{name: "complete history", registered: registered, applied: registered},
		{name: "partial history", registered: registered, applied: registered[:1], want: registered[1:]},
		{name: "whitespace-only ID", registered: []migration.Entry{malformed}, conflict: true},
		{name: "invalid UTF-8 ID", registered: []migration.Entry{badEncoding}, conflict: true},
		{name: "empty ID", registered: []migration.Entry{{Digest: first.Digest}}, conflict: true},
		{name: "duplicate ID", registered: []migration.Entry{first, second, duplicate}, conflict: true},
		{name: "first entry has predecessor", registered: []migration.Entry{wrongFirst}, conflict: true},
		{name: "broken predecessor in suffix", registered: []migration.Entry{first, second, wrongSuccessor}, applied: registered[:1], conflict: true},
		{name: "missing digest in suffix", registered: []migration.Entry{first, second, missingDigest}, applied: registered[:1], conflict: true},
		{name: "unknown applied entry", registered: registered, applied: []migration.Entry{unknown}, conflict: true},
		{name: "empty registry with applied entry", applied: registered[:1], conflict: true},
		{name: "history exceeds registry", registered: registered[:1], applied: registered, conflict: true},
		{name: "gap in applied history", registered: registered, applied: []migration.Entry{first, third}, conflict: true},
		{name: "reordered history", registered: registered, applied: []migration.Entry{second, first}, conflict: true},
		{name: "case change in applied ID", registered: registered, applied: []migration.Entry{changedCase}, conflict: true},
		{name: "trimmed applied ID", registered: registered, applied: []migration.Entry{first, changedSpacing}, conflict: true},
		{name: "altered predecessor", registered: registered, applied: []migration.Entry{wrongFirst}, conflict: true},
		{name: "altered digest", registered: registered, applied: []migration.Entry{alteredDigest}, conflict: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			registry := slices.Clone(test.registered)
			history := slices.Clone(test.applied)
			got, err := migration.Pending(registry, history)
			if !slices.Equal(registry, test.registered) || !slices.Equal(history, test.applied) {
				t.Fatal("Pending modified the input")
			}
			if test.conflict {
				if got != nil {
					t.Fatal("Pending returned a partial plan on conflict")
				}
				checkConflict(t, err)
				return
			}
			if err != nil {
				t.Fatalf("Pending returned an unexpected error: %v", err)
			}
			if !slices.Equal(got, test.want) || (test.want == nil && got != nil) {
				t.Fatalf("Pending returned %v, want %v", got, test.want)
			}
			if len(got) > 0 {
				got[0] = migration.Entry{}
				if !slices.Equal(registry, test.registered) || !slices.Equal(history, test.applied) {
					t.Fatal("Pending result shares storage with an input")
				}
			}
		})
	}
}

func checkConflict(t *testing.T, err error) {
	t.Helper()
	var detail *goeferr.ErrorDetail
	if !errors.Is(err, goeferr.ErrMigrationConflict) || !errors.As(err, &detail) {
		t.Fatalf("Pending error %v does not preserve the portable conflict", err)
	}
	if detail.ID() != "GOEF-MIG-0002" || detail.Phase() != "verify-history" ||
		detail.RetryClass() != goeferr.RetryNever || detail.Outcome() != goeferr.OutcomeNoCommitAttempted {
		t.Fatalf("unexpected conflict detail: %q, %q, %q, %q", detail.ID(), detail.Phase(), detail.RetryClass(), detail.Outcome())
	}
	if err.Error() != "Migration history conflicts with the registered set." || errors.Unwrap(err) != goeferr.ErrMigrationConflict {
		t.Fatalf("unexpected public error chain: %v", err)
	}
}
