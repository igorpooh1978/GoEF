package driverlab

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
)

func openPostgres(t *testing.T) fixture {
	t.Helper()

	var expectedVersion, expectedVersionNum string
	switch os.Getenv("GOEF_DRIVERLAB_PROFILE") {
	case "postgres-19":
		expectedVersion, expectedVersionNum = "19.0", "190000"
	case "", "postgres-18":
		expectedVersion, expectedVersionNum = "18.7", "180007"
	case "postgres-17":
		expectedVersion, expectedVersionNum = "17.12", "170012"
	case "postgres-16":
		expectedVersion, expectedVersionNum = "16.16", "160016"
	default:
		t.Fatal("PostgreSQL lab profile must be postgres-19, postgres-18, postgres-17, or postgres-16")
	}

	password := os.Getenv("GOEF_DRIVERLAB_PASSWORD")
	if password == "" {
		t.Fatal("PostgreSQL lab requires GOEF_DRIVERLAB_PASSWORD")
	}
	// pgx resolves a named service before applying explicit connection settings.
	if os.Getenv("PGSERVICE") != "" {
		t.Fatal("PostgreSQL lab requires PGSERVICE to be unset")
	}

	// The placeholder prevents pgx from consulting a password file. Assign the
	// actual secret directly so parse errors cannot include it.
	const settings = "host=127.0.0.1 port=5432 dbname=goef_smoke user=goef_driver password=placeholder " +
		"connect_timeout=5 sslmode=disable sslcertmode=disable sslrootcert='' sslnegotiation=postgres " +
		"target_session_attrs=any min_protocol_version=3.0 max_protocol_version=3.0 " +
		"channel_binding=disable require_auth=scram-sha-256"
	config, err := pgx.ParseConfig(settings)
	if err != nil {
		t.Fatal("PostgreSQL lab connection configuration failed")
	}
	if config.Host != "127.0.0.1" || config.Port != 5432 || config.TLSConfig != nil || len(config.Fallbacks) != 0 {
		t.Fatal("PostgreSQL lab connection target did not match the fixed local endpoint")
	}
	config.Password = password
	config.DefaultQueryExecMode = pgx.QueryExecModeExec
	config.RuntimeParams = map[string]string{
		"application_name":              "goef_driverlab",
		"TimeZone":                      "UTC",
		"default_transaction_isolation": "read committed",
		"lock_timeout":                  "5s",
		"search_path":                   "public",
		"statement_timeout":             "15s",
	}

	db := stdlib.OpenDB(*config)
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error("PostgreSQL lab connection pool did not close")
		}
	})

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	const identity = `SELECT current_setting('server_version'), current_setting('server_version_num'),
		current_database(), current_user, current_setting('TimeZone'), current_setting('transaction_isolation'),
		current_setting('server_encoding'), current_setting('search_path'), datcollate
		FROM pg_catalog.pg_database WHERE datname = current_database()`
	var versionText, versionNum, database, user, timeZone, isolation, encoding, searchPath, collation string
	row := db.QueryRowContext(ctx, identity)
	if err := row.Scan(&versionText, &versionNum, &database, &user, &timeZone, &isolation, &encoding, &searchPath, &collation); err != nil {
		t.Fatal("PostgreSQL lab identity query failed")
	}
	version, _, _ := strings.Cut(versionText, " ")
	if version != expectedVersion || versionNum != expectedVersionNum || database != "goef_smoke" || user != "goef_driver" {
		t.Fatal("PostgreSQL lab version, database, or user did not match the pinned target")
	}
	if timeZone != "UTC" || isolation != "read committed" || encoding != "UTF8" || searchPath != "public" || collation != "C" {
		t.Fatal("PostgreSQL lab session or database settings did not match the pinned profile")
	}

	return fixture{
		db: db,
		bind: func(position int) string {
			return "$" + strconv.Itoa(position)
		},
		code:           postgresErrorCode,
		uniqueCode:     "23505",
		foreignKeyCode: "23503",
		facts: map[string]string{
			"engine_version":     version,
			"server_version_num": versionNum,
			"database":           database,
			"user":               user,
			"time_zone":          timeZone,
			"isolation":          isolation,
			"encoding":           encoding,
			"search_path":        searchPath,
			"collation":          collation,
			"query_mode":         config.DefaultQueryExecMode.String(),
		},
	}
}

func postgresErrorCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr != nil {
		return pgErr.SQLState()
	}
	return ""
}
