package driverlab

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

func openMySQL(t *testing.T) fixture {
	t.Helper()

	const (
		database  = "goef_smoke"
		login     = "goef_driver"
		edition   = "MySQL Community Server - GPL"
		engine    = "InnoDB"
		isolation = "READ-COMMITTED"
		charset   = "utf8mb4"
		collation = "utf8mb4_bin"
		timeZone  = "+00:00"
		sqlMode   = "STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION"
	)
	const identityColumns = ", @@version_comment, DATABASE(), SUBSTRING_INDEX(USER(), '@', 1), CURRENT_USER()"
	var version, identitySQL string
	switch os.Getenv("GOEF_DRIVERLAB_PROFILE") {
	case "", "mysql-8.4":
		version, identitySQL = "8.4.13.1", "SELECT VERSION_WITH_UPDATE()"+identityColumns
	case "mysql-8.0":
		version, identitySQL = "8.0.46", "SELECT VERSION()"+identityColumns
	default:
		t.Fatal("invalid GOEF_DRIVERLAB_PROFILE for MySQL fixture")
	}
	password := os.Getenv("GOEF_DRIVERLAB_PASSWORD")
	if password == "" {
		t.Fatal("MySQL requires GOEF_DRIVERLAB_PASSWORD")
	}

	cfg := mysql.NewConfig()
	cfg.Net = "tcp"
	cfg.Addr = "127.0.0.1:3306"
	cfg.DBName = database
	cfg.User = login
	cfg.Passwd = password
	cfg.Timeout = 5 * time.Second
	cfg.ReadTimeout = 10 * time.Second
	cfg.WriteTimeout = 10 * time.Second
	cfg.TLSConfig = "false"
	cfg.Logger = log.New(io.Discard, "", 0)
	if err := cfg.Apply(mysql.Charset(charset, collation)); err != nil {
		t.Fatal("configure MySQL connection character set")
	}
	// The driver applies these fixed SQL literals to each new physical connection.
	cfg.Params = map[string]string{
		"SESSION default_storage_engine": "'" + engine + "'",
		"SESSION transaction_isolation":  "'" + isolation + "'",
		"SESSION time_zone":              "'" + timeZone + "'",
		"SESSION sql_mode":               "'" + sqlMode + "'",
		"SESSION autocommit":             "1",
		"SESSION foreign_key_checks":     "1",
	}
	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		t.Fatal("configure MySQL driver connector")
	}
	db := sql.OpenDB(connector)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error("close MySQL fixture pool")
		}
	})
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	var gotVersion, gotEdition, gotDatabase, gotLogin, gotAccount string
	if err := db.QueryRowContext(ctx, identitySQL).
		Scan(&gotVersion, &gotEdition, &gotDatabase, &gotLogin, &gotAccount); err != nil {
		t.Fatal("read MySQL fixture identity")
	}
	if gotVersion != version || gotEdition != edition || gotDatabase != database ||
		gotLogin != login || gotAccount != login+"@%" {
		t.Fatal("MySQL fixture version, edition, database, or authenticated account did not match")
	}

	var gotEngine, gotIsolation, gotCharset, gotCollation, gotTimeZone, gotSQLMode string
	var supportsTransactions, autocommit, foreignKeyChecks bool
	if err := db.QueryRowContext(ctx, `
		SELECT @@session.default_storage_engine, @@session.transaction_isolation,
		       @@session.character_set_connection, @@session.collation_connection,
		       @@session.time_zone, @@session.sql_mode, @@session.autocommit,
		       @@session.foreign_key_checks,
		       EXISTS (SELECT 1 FROM information_schema.ENGINES
		               WHERE ENGINE = 'InnoDB' AND SUPPORT IN ('YES', 'DEFAULT')
		                 AND TRANSACTIONS = 'YES')
	`).Scan(&gotEngine, &gotIsolation, &gotCharset, &gotCollation, &gotTimeZone, &gotSQLMode,
		&autocommit, &foreignKeyChecks, &supportsTransactions); err != nil {
		t.Fatal("read MySQL fixture session profile")
	}
	if gotEngine != engine || gotIsolation != isolation || gotCharset != charset ||
		gotCollation != collation || gotTimeZone != timeZone || gotSQLMode != sqlMode ||
		!autocommit || !foreignKeyChecks || !supportsTransactions {
		t.Fatal("MySQL fixture session or InnoDB profile did not match")
	}

	return fixture{
		db:             db,
		bind:           func(int) string { return "?" },
		code:           mysqlDriverCode,
		uniqueCode:     "1062/23000",
		foreignKeyCode: "1452/23000",
		facts: map[string]string{
			"driver":                   "github.com/go-sql-driver/mysql v1.10.0",
			"server_version":           gotVersion,
			"variant":                  gotEdition,
			"storage_engine":           gotEngine,
			"transaction_isolation":    gotIsolation,
			"character_set_connection": gotCharset,
			"collation_connection":     gotCollation,
			"time_zone":                gotTimeZone,
			"sql_mode":                 gotSQLMode,
		},
	}
}

func mysqlDriverCode(err error) string {
	var native *mysql.MySQLError
	if !errors.As(err, &native) || string(native.SQLState[:]) != "23000" {
		return ""
	}
	switch native.Number {
	case 1062:
		return "1062/23000"
	case 1452:
		return "1452/23000"
	default:
		return ""
	}
}
