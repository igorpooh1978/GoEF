package driverlab

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"
)

const sqliteWALCommittedExit = 67

func TestSQLiteWALRecoveryStep(t *testing.T) {
	stage, enabled := os.LookupEnv("GOEF_SQLITE_WAL_RECOVERY_STAGE")
	if !enabled {
		t.Skip("run the SQLite WAL recovery lab")
	}
	switch stage {
	case "seed", "commit-exit", "checkpoint", "reopen":
	default:
		t.Fatal("invalid SQLite WAL recovery stage")
	}

	marker := "GOEF_SQLITE_WAL_RESULT=" + sqliteVersion + "/" + stage
	if t.Run(stage, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()

		path := sqliteFilePath(t, stage == "seed")
		mode := "rw"
		if stage == "reopen" {
			mode = "ro"
		}
		db := openSQLitePool(t, path, mode)
		if stage == "seed" {
			setSQLiteJournalMode(t, ctx, db, "wal")
		}
		checkSQLiteConnections(t, ctx, db, "wal")
		if stage != "seed" {
			var appID int
			if err := db.QueryRowContext(ctx, "PRAGMA application_id").Scan(&appID); err != nil || appID != sqliteFileAppID {
				t.Fatal("unexpected application ID in SQLite WAL fixture")
			}
		}

		switch stage {
		case "seed":
			createSQLiteFileSchema(t, ctx, db)
			assertSQLiteFileState(t, ctx, db, false)
			checkpointSQLiteWAL(t, ctx, db)
		case "commit-exit":
			assertSQLiteFileState(t, ctx, db, false)
			commitSQLiteWAL(t, ctx, db)
			// Deliberately skip SQLite connection shutdown. The OS closes
			// the process handles without SQLite's last-connection checkpoint.
			fmt.Fprintln(os.Stderr, marker)
			os.Exit(sqliteWALCommittedExit)
		case "checkpoint":
			assertSQLiteFileState(t, ctx, db, true)
			checkpointSQLiteWAL(t, ctx, db)
			assertSQLiteFileState(t, ctx, db, true)
		case "reopen":
			assertSQLiteFileState(t, ctx, db, true)
		}
	}) {
		fmt.Println(marker)
	}
}

func commitSQLiteWAL(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal("could not acquire the SQLite WAL writer")
	}
	t.Cleanup(func() {
		if conn.Close() != nil {
			t.Error("could not release the SQLite WAL writer")
		}
	})
	var autoCheckpoint int
	if err := conn.QueryRowContext(ctx, "PRAGMA wal_autocheckpoint=0").Scan(&autoCheckpoint); err != nil || autoCheckpoint != 0 {
		t.Fatal("could not disable automatic checkpointing on the writer")
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA synchronous=FULL"); err != nil {
		t.Fatal("could not configure synchronous SQLite WAL commits")
	}
	var synchronous int
	if err := conn.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&synchronous); err != nil || synchronous != 2 {
		t.Fatal("SQLite WAL writer did not use synchronous FULL")
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal("could not begin the SQLite WAL writer transaction")
	}
	defer finishUnfinishedSQLiteFileTx(t, tx)
	changeSQLiteFile(t, ctx, tx)
	if tx.Commit() != nil {
		t.Fatal("could not commit the SQLite WAL writer transaction")
	}
}

func checkpointSQLiteWAL(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	var busy, pages, checkpointed int
	err := db.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &pages, &checkpointed)
	if err != nil || busy != 0 || pages != 0 || checkpointed != 0 {
		t.Fatal("the SQLite WAL truncate checkpoint did not complete")
	}
}
