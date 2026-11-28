package migration

import (
	"context"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/migration"
	"github.com/jackc/pgx/v5"
)

const historyDefinition = ` (
	position bigint PRIMARY KEY CHECK (position > 0),
	id text COLLATE pg_catalog."C" NOT NULL UNIQUE,
	predecessor text NOT NULL,
	digest bytea NOT NULL CHECK (pg_catalog.octet_length(digest) = 32)
)`

// Require a permanent, ordinary history table with the expected non-null
// columns. Refuse mechanisms that could hide or rewrite its recorded rows.
const inspectHistory = `SELECT coalesce((
	SELECT c.oid FROM pg_catalog.pg_class AS c
	WHERE c.oid = pg_catalog.to_regclass($1)
		AND c.relkind = 'r' AND c.relpersistence = 'p'
		AND NOT c.relrowsecurity AND NOT c.relispartition
		AND NOT EXISTS (SELECT FROM pg_catalog.pg_trigger WHERE tgrelid = c.oid)
		AND NOT EXISTS (SELECT FROM pg_catalog.pg_rewrite WHERE ev_class = c.oid)
		AND NOT EXISTS (SELECT FROM pg_catalog.pg_inherits WHERE inhparent = c.oid OR inhrelid = c.oid)
		AND (SELECT count(*) = 4 AND count(*) FILTER (WHERE attnotnull AND
				(attname, atttypid) IN (('position', 'pg_catalog.int8'::pg_catalog.regtype),
					('id', 'pg_catalog.text'::pg_catalog.regtype),
					('predecessor', 'pg_catalog.text'::pg_catalog.regtype),
					('digest', 'pg_catalog.bytea'::pg_catalog.regtype))) = 4
			FROM pg_catalog.pg_attribute
			WHERE attrelid = c.oid AND attnum > 0 AND NOT attisdropped)
), 0::oid)`

func historyOID(ctx context.Context, tx pgx.Tx, history string) (uint32, error) {
	var oid uint32
	if err := tx.QueryRow(ctx, inspectHistory, history).Scan(&oid); err != nil {
		return 0, safeError(err)
	}
	if oid == 0 {
		return 0, goeferr.NewMigrationConflict()
	}
	return oid, nil
}

func readHistory(ctx context.Context, tx pgx.Tx, history string, limit int) ([]migration.Entry, error) {
	rows, err := tx.Query(ctx, "SELECT position, id, predecessor, digest FROM "+history+" ORDER BY position LIMIT $1", int64(limit)+1)
	if err != nil {
		return nil, safeError(err)
	}
	defer rows.Close()

	var applied []migration.Entry
	for rows.Next() {
		if len(applied) == limit {
			return nil, goeferr.NewMigrationConflict()
		}
		var position int64
		var entry migration.Entry
		var digest []byte
		if err := rows.Scan(&position, &entry.ID, &entry.Predecessor, &digest); err != nil {
			return nil, safeError(err)
		}
		if position != int64(len(applied))+1 || len(digest) != len(entry.Digest) {
			return nil, goeferr.NewMigrationConflict()
		}
		copy(entry.Digest[:], digest)
		applied = append(applied, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, safeError(err)
	}
	return applied, nil
}

func recordHistory(ctx context.Context, tx pgx.Tx, history string, oid uint32, position int, entry migration.Entry) error {
	actual, err := historyOID(ctx, tx, history)
	if err != nil {
		return err
	}
	if actual != oid {
		return goeferr.NewMigrationRecoveryRequired()
	}
	_, err = tx.Exec(ctx, "INSERT INTO "+history+" (position, id, predecessor, digest) VALUES ($1, $2, $3, $4)",
		int64(position), entry.ID, entry.Predecessor, entry.Digest[:])
	return safeError(err)
}
