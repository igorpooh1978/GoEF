// Package migration validates registered migration lineages and plans pending entries.
package migration

import (
	"crypto/sha256"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/igorpooh1978/GoEF/goeferr"
)

// Entry identifies one immutable migration artifact in an ordered lineage.
type Entry struct {
	// ID is a case-sensitive UTF-8 string containing at least one
	// non-whitespace character. It is compared byte for byte.
	ID string

	// Predecessor is empty for the first entry and equals the immediately
	// preceding entry's ID for every later entry.
	Predecessor string

	// Digest is the nonzero SHA-256 digest of the canonical artifact, supplied
	// by the caller.
	Digest [sha256.Size]byte
}

// Pending validates one ordered, linear registry and verifies that applied is
// its exact prefix by ID, predecessor and digest. It returns an independent
// copy of the remaining entries, or nil when there are none.
//
// Pending reads the supplied slices in memory. The caller must keep them
// stable during the call. A malformed registry or mismatched history returns
// [goeferr.ErrMigrationConflict] with no plan.
func Pending(registered, applied []Entry) ([]Entry, error) {
	seen := make(map[string]struct{}, len(registered))
	predecessor := ""
	for _, entry := range registered {
		if !utf8.ValidString(entry.ID) || strings.TrimSpace(entry.ID) == "" ||
			entry.Predecessor != predecessor || entry.Digest == ([sha256.Size]byte{}) {
			return nil, goeferr.NewMigrationConflict()
		}
		if _, ok := seen[entry.ID]; ok {
			return nil, goeferr.NewMigrationConflict()
		}
		seen[entry.ID] = struct{}{}
		predecessor = entry.ID
	}

	if len(applied) > len(registered) || !slices.Equal(registered[:len(applied)], applied) {
		return nil, goeferr.NewMigrationConflict()
	}
	if len(applied) == len(registered) {
		return nil, nil
	}
	return slices.Clone(registered[len(applied):]), nil
}
