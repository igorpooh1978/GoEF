package migration_test

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/migration"
)

func ExamplePending() {
	registered := []migration.Entry{
		{ID: "001_customers", Digest: sha256.Sum256([]byte("synthetic initial artifact"))},
		{
			ID:          "002_customer_notes",
			Predecessor: "001_customers",
			Digest:      sha256.Sum256([]byte("synthetic notes artifact")),
		},
	}
	applied := []migration.Entry{registered[0]}

	pending, err := migration.Pending(registered, applied)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(pending[0].ID)

	applied[0].Digest = sha256.Sum256([]byte("changed synthetic artifact"))
	if _, err := migration.Pending(registered, applied); errors.Is(err, goeferr.ErrMigrationConflict) {
		fmt.Println("review the applied migration history")
	}

	// Output:
	// 002_customer_notes
	// review the applied migration history
}
