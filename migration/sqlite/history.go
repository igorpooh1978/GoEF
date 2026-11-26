package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/migration"
)

const createHistory = `CREATE TABLE IF NOT EXISTS main.__goef_migrations_history (
	position INTEGER PRIMARY KEY CHECK (position > 0),
	id TEXT NOT NULL UNIQUE,
	predecessor TEXT NOT NULL,
	digest BLOB NOT NULL CHECK (length(digest) = 32)
) STRICT`

const selectHistory = "SELECT position, id, predecessor, digest FROM main.__goef_migrations_history ORDER BY position"
const insertHistory = "INSERT INTO main.__goef_migrations_history (position, id, predecessor, digest) VALUES (?, ?, ?, ?)"

func readHistory(ctx context.Context, conn *sql.Conn, limit int) (history []migration.Entry, err error) {
	if _, err := conn.ExecContext(ctx, createHistory); err != nil {
		return nil, safeError(err)
	}
	rows, err := conn.QueryContext(ctx, selectHistory)
	if err != nil {
		return nil, safeError(err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			history = nil
			err = errors.Join(err, safeError(closeErr))
		}
	}()
	for rows.Next() {
		if len(history) == limit {
			return nil, goeferr.NewMigrationConflict()
		}
		var position int
		var entry migration.Entry
		var digest []byte
		if err := rows.Scan(&position, &entry.ID, &entry.Predecessor, &digest); err != nil {
			return nil, safeError(err)
		}
		if position != len(history)+1 || len(digest) != len(entry.Digest) {
			return nil, goeferr.NewMigrationConflict()
		}
		copy(entry.Digest[:], digest)
		history = append(history, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, safeError(err)
	}
	return history, nil
}
