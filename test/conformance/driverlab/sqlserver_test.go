package driverlab

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	mssql "github.com/microsoft/go-mssqldb"
)

const (
	sqlServerHost    = "127.0.0.1"
	sqlServerAddress = sqlServerHost + ":1433"
	sqlServerCatalog = "goef_smoke"
	sqlServerLogin   = "goef_driver"

	sqlServerSessionSQL = `
SET TRANSACTION ISOLATION LEVEL READ COMMITTED;
SET XACT_ABORT ON;`
	sqlServerProfileSQL = `
SELECT CONVERT(varchar(20), SERVERPROPERTY('ProductVersion')),
       CONVERT(varchar(10), SERVERPROPERTY('ProductUpdateLevel')),
       CONVERT(varchar(40), SERVERPROPERTY('Edition')),
       DB_NAME(), DB_ID(), ORIGINAL_LOGIN(), SCHEMA_NAME(),
       d.compatibility_level, d.collation_name,
       d.snapshot_isolation_state, d.is_read_committed_snapshot_on,
       s.transaction_isolation_level, @@OPTIONS & 16384,
       CONVERT(varchar(40), CONNECTIONPROPERTY('net_transport'))
FROM sys.databases AS d
JOIN sys.dm_exec_sessions AS s ON s.session_id = @@SPID
WHERE d.database_id = DB_ID();`
)

type sqlServerPin struct {
	version            string
	updateLevel        string
	compatibilityLevel int
}

// sqlServerDialer keeps driver-directed routing inside the isolated endpoint.
type sqlServerDialer struct{}

func (sqlServerDialer) HostName() string { return sqlServerHost }

func (sqlServerDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if network != "tcp" || address != sqlServerAddress {
		return nil, errors.New("unexpected SQL Server fixture endpoint")
	}
	var dialer net.Dialer
	return dialer.DialContext(ctx, network, address)
}

func openSQLServer(t *testing.T) fixture {
	t.Helper()

	profile := os.Getenv("GOEF_DRIVERLAB_PROFILE")
	if profile == "" {
		profile = "sqlserver-2022"
	}
	expected, ok := map[string]sqlServerPin{
		"sqlserver-2017": {version: "14.0.3570.2", updateLevel: "CU31", compatibilityLevel: 140},
		"sqlserver-2019": {version: "15.0.4510.2", updateLevel: "CU32", compatibilityLevel: 150},
		"sqlserver-2022": {version: "16.0.4325.4", updateLevel: "CU28", compatibilityLevel: 160},
	}[profile]
	if !ok {
		t.Fatal("unknown SQL Server fixture profile")
	}

	password := os.Getenv("GOEF_DRIVERLAB_PASSWORD")
	if password == "" {
		t.Fatal("SQL Server fixture password is missing")
	}
	dsn := url.URL{
		Scheme: "sqlserver",
		User:   url.UserPassword(sqlServerLogin, password),
		Host:   sqlServerAddress,
		RawQuery: url.Values{
			"database": {sqlServerCatalog},
			"encrypt":  {"true"},
			"protocol": {"tcp"},
			// The isolated container uses an ephemeral self-signed certificate.
			"TrustServerCertificate": {"true"},
		}.Encode(),
	}
	connector, err := mssql.NewConnector(dsn.String())
	if err != nil {
		t.Fatal("could not configure SQL Server fixture")
	}
	connector.Dialer = sqlServerDialer{}
	connector.SessionInitSQL = sqlServerSessionSQL

	db := sql.OpenDB(connector)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error("could not close SQL Server fixture")
		}
	})
	db.SetMaxOpenConns(2)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	var version, update, edition, catalog, login, schema, collation, transport string
	var databaseID, compatibility, snapshot, isolation, xactAbort int
	var rcsi bool
	err = db.QueryRowContext(ctx, sqlServerProfileSQL).Scan(
		&version, &update, &edition, &catalog, &databaseID, &login, &schema, &compatibility,
		&collation, &snapshot, &rcsi, &isolation, &xactAbort, &transport,
	)
	if err != nil {
		t.Fatal("could not read SQL Server fixture settings")
	}
	if version != expected.version || update != expected.updateLevel || edition != "Developer Edition (64-bit)" {
		t.Fatal("SQL Server fixture build or edition differs from the pin")
	}
	if catalog != sqlServerCatalog || databaseID <= 4 || compatibility != expected.compatibilityLevel ||
		collation != "Latin1_General_100_BIN2" || snapshot != 0 || rcsi {
		t.Fatal("SQL Server fixture catalog or database settings differ from the profile")
	}
	if login != sqlServerLogin || schema != "goef_fixture" ||
		isolation != 2 || xactAbort != 16384 || transport != "TCP" {
		t.Fatal("SQL Server fixture session settings differ from the profile")
	}

	return fixture{
		db:             db,
		bind:           func(n int) string { return "@p" + strconv.Itoa(n) },
		code:           sqlServerNativeCode,
		uniqueCode:     "2627",
		foreignKeyCode: "547",
		facts: map[string]string{
			"version":                 version,
			"product_update_level":    update,
			"edition":                 edition,
			"target":                  catalog,
			"compatibility_level":     strconv.Itoa(compatibility),
			"collation":               collation,
			"snapshot_isolation":      "off",
			"read_committed_snapshot": "off",
			"isolation":               "READ COMMITTED",
			"xact_abort":              "on",
			"default_schema":          schema,
			"transport":               transport,
			"mars":                    "off", // go-mssqldb v1.11.0 sends zero in its TDS prelogin MARS field.
		},
	}
}

func sqlServerNativeCode(err error) string {
	var native mssql.Error
	if !errors.As(err, &native) {
		return ""
	}
	// The driver exposes the last diagnostic as Number. SQL Server can follow
	// a constraint violation with 3621 ("The statement has been terminated").
	if native.Number == 3621 {
		for _, diagnostic := range native.All {
			switch diagnostic.Number {
			case 2627, 547:
				return strconv.FormatInt(int64(diagnostic.Number), 10)
			}
		}
	}
	return strconv.FormatInt(int64(native.Number), 10)
}
