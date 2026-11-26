package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"slices"

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

const inspectHistoryTable = `SELECT
	EXISTS (SELECT 1 FROM pragma_table_list('__goef_migrations_history')
		WHERE "schema" = 'main' AND type = 'table' AND ncol = 4 AND strict = 1)
	AND NOT EXISTS (SELECT 1 FROM main.sqlite_schema
		WHERE type = 'trigger' AND tbl_name = '__goef_migrations_history' COLLATE NOCASE)
	AND NOT EXISTS (SELECT 1 FROM temp.sqlite_schema
		WHERE type = 'trigger' AND tbl_name = '__goef_migrations_history' COLLATE NOCASE)`

func verifyHistoryTable(ctx context.Context, conn *sql.Conn) error {
	var valid bool
	if err := conn.QueryRowContext(ctx, inspectHistoryTable).Scan(&valid); err != nil {
		return safeError(err)
	}
	if !valid {
		return goeferr.NewMigrationConflict()
	}
	return nil
}

func recordHistory(ctx context.Context, conn *sql.Conn, lineage []migration.Entry) error {
	// Check again in case the reviewed operations introduced a history trigger.
	if err := verifyHistoryTable(ctx, conn); err != nil {
		return err
	}
	entry := lineage[len(lineage)-1]
	result, err := conn.ExecContext(ctx, insertHistory, len(lineage), entry.ID, entry.Predecessor, entry.Digest[:])
	if err != nil {
		return safeError(err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return safeError(err)
	}
	if count != 1 {
		return goeferr.NewMigrationRecoveryRequired()
	}
	observed, err := readHistory(ctx, conn, len(lineage))
	if err != nil {
		return err
	}
	if !slices.Equal(observed, lineage) {
		return goeferr.NewMigrationRecoveryRequired()
	}
	return nil
}

func readHistory(ctx context.Context, conn *sql.Conn, limit int) (history []migration.Entry, err error) {
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
