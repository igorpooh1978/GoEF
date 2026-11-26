# GoEF

GoEF is a Go data access library in alpha. It uses Entity Framework Core as a behavioral reference. The preview includes Code First mapping, transactions, migration history validation, in-memory GoLINQ operators, and separate database adapters.

[SQLite](#quick-start-with-sqlite) · [PostgreSQL](#postgresql) · [SQL Server LocalDB](#sql-server-express-localdb) · [MySQL](#mysql) · [Documentation](docs/README.md) · [Runnable examples](examples/README.md#database-quickstarts)

## Install the alpha

Use Go 1.27.4 or later. Pin the core version from your application's Go module.

```sh
go mod edit '-require=github.com/igorpooh1978/GoEF@v0.1.0-alpha.1'
```

Pin the adapter for your database.

```sh
go mod edit '-require=github.com/igorpooh1978/GoEF/providers/postgres@v0.1.0-alpha.1'
go mod edit '-require=github.com/igorpooh1978/GoEF/providers/sqlite@v0.1.0-alpha.1'
go mod edit '-require=github.com/igorpooh1978/GoEF/providers/sqlserver@v0.1.0-alpha.1'
go mod edit '-require=github.com/igorpooh1978/GoEF/providers/mysql@v0.1.0-alpha.1'
```

After adding the imports to your code, run `go mod tidy` to download the pinned modules and update `go.sum`.

### GitHub prereleases

- [GoEF core](https://github.com/igorpooh1978/GoEF/releases/tag/v0.1.0-alpha.1)
- [PostgreSQL adapter](https://github.com/igorpooh1978/GoEF/releases/tag/providers/postgres/v0.1.0-alpha.1)
- [SQLite adapter](https://github.com/igorpooh1978/GoEF/releases/tag/providers/sqlite/v0.1.0-alpha.1)
- [SQL Server adapter](https://github.com/igorpooh1978/GoEF/releases/tag/providers/sqlserver/v0.1.0-alpha.1)
- [MySQL adapter](https://github.com/igorpooh1978/GoEF/releases/tag/providers/mysql/v0.1.0-alpha.1)

See the [provider matrix](docs/parity/PROVIDER_MATRIX.md) for verified capabilities and open gaps.

## Quick start with SQLite

In a new directory outside the GoEF checkout and any Go workspace, initialize a module and pin both published versions. The adapter embeds SQLite 3.51.3 through the pure-Go driver `modernc.org/sqlite` v1.47.0. Go resolves both modules directly with no `replace` directive.

```sh
go mod init example.com/goef-demo
go mod edit '-require=github.com/igorpooh1978/GoEF@v0.1.0-alpha.1' '-require=github.com/igorpooh1978/GoEF/providers/sqlite@v0.1.0-alpha.1'
```

Save this program as `main.go`. It runs a parameterized SQLite query against a temporary file and closes the result stream, connection lease, and pool before removing the file.

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/igorpooh1978/GoEF/provider"
	"github.com/igorpooh1978/GoEF/providers/sqlite"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() (err error) {
	file, err := os.CreateTemp("", "goef-*.db")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.Remove(file.Name())) }()
	if err := file.Close(); err != nil {
		return err
	}

	backend, err := sqlite.Open(file.Name())
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, backend.DB().Close()) }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cleanupCtx := context.WithoutCancel(ctx)

	lease, err := backend.Acquire(ctx)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, lease.Release(cleanupCtx, provider.Discard)) }()

	command := provider.Command{
		SQL:     "SELECT ?",
		Slots:   []provider.Slot{{Kind: provider.KindText}},
		Columns: []provider.Column{{Kind: provider.KindText}},
	}
	rows, err := lease.Query(ctx, command, provider.Bindings{provider.Text("hello from SQLite")})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, rows.Close(cleanupCtx)) }()

	row, err := rows.Next()
	if err != nil {
		return err
	}
	fmt.Println(row[0])
	return nil
}
```

```sh
go mod tidy
go run .
```

Expected output

```text
hello from SQLite
```

If a row cannot be decoded using the declared column kind, check for `goeferr.ErrMaterialization` with `errors.Is`. [Executable SQLite adapter example](providers/sqlite/example_test.go)

## Quick starts for PostgreSQL, SQL Server LocalDB, and MySQL

Choose one database. In a new directory outside the GoEF checkout and any Go workspace, initialize an application module. Pin the published core and the adapter from the section for your database. Go resolves the releases without a `replace` directive.

```sh
go mod init example.com/goef-demo
go mod edit '-require=github.com/igorpooh1978/GoEF@v0.1.0-alpha.1'
```

For the PostgreSQL and MySQL setup below, PowerShell users can set the connection string with `$env:GOEF_DSN = '...'`.

Save the [shared program](examples/testdata/alpha-quickstart/main.go) as `main.go`. It binds the integer `41`, gives the query and row read a five-second timeout, and closes the rows and connection lease before the adapter closes its pool.

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/igorpooh1978/GoEF/provider"
)

func main() {
	dsn := os.Getenv("GOEF_DSN")
	if dsn == "" {
		log.Fatal("set GOEF_DSN to your database connection string")
	}
	if err := run(dsn); err != nil {
		log.Fatal(err)
	}
}

func readValue(backend provider.Backend, command provider.Command) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cleanupCtx := context.WithoutCancel(ctx)

	lease, err := backend.Acquire(ctx)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, lease.Release(cleanupCtx, provider.Discard)) }()

	rows, err := lease.Query(ctx, command, provider.Bindings{provider.Integer(41)})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, rows.Close(cleanupCtx)) }()

	row, err := rows.Next()
	if err != nil {
		return err
	}
	fmt.Println(row[0])
	return nil
}
```

### PostgreSQL

Use a running PostgreSQL instance and an existing database. The sample connection string uses a disposable local instance. Set `GOEF_DSN` to the connection string for your instance.

```sh
go mod edit '-require=github.com/igorpooh1978/GoEF/providers/postgres@v0.1.0-alpha.1'
export GOEF_DSN='postgres://demo:password@127.0.0.1:5432/demo?sslmode=disable'
```

Save the [PostgreSQL example](examples/testdata/alpha-quickstart/postgres/adapter.go) as `adapter.go`.

```go
package main

import (
	"errors"

	"github.com/igorpooh1978/GoEF/provider"
	"github.com/igorpooh1978/GoEF/providers/postgres"
)

func run(dsn string) (err error) {
	backend, err := postgres.Open(dsn)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, backend.DB().Close()) }()

	return readValue(backend, provider.Command{
		SQL:     "SELECT $1::bigint + 1",
		Slots:   []provider.Slot{{Kind: provider.KindInteger}},
		Columns: []provider.Column{{Kind: provider.KindInteger}},
	})
}
```

### SQL Server Express LocalDB

On Windows, use an installed `MSSQLLocalDB` instance. Run these commands in PowerShell to start it and get its current named pipe. The example connects using your Windows account. [Microsoft LocalDB documentation](https://learn.microsoft.com/en-us/sql/database-engine/configure-windows/sql-server-express-localdb)

```powershell
go mod edit '-require=github.com/igorpooh1978/GoEF/providers/sqlserver@v0.1.0-alpha.1'
sqllocaldb start MSSQLLocalDB
$pipeMatch = sqllocaldb info MSSQLLocalDB | Select-String '\\\\\.\\pipe\\LOCALDB#[0-9A-F]+\\tsql\\query'
if (!$pipeMatch) { throw 'LocalDB did not report a named pipe.' }
$pipe = $pipeMatch.Matches[0].Value
$env:GOEF_DSN = "server=$pipe;protocol=np;database=master;authenticator=winsspi"
```

Save the [LocalDB example](examples/testdata/alpha-quickstart/sqlserver/adapter.go) as `adapter.go`. It imports the SQL Server driver's named-pipe support.

```go
package main

import (
	"errors"

	"github.com/igorpooh1978/GoEF/provider"
	"github.com/igorpooh1978/GoEF/providers/sqlserver"
	_ "github.com/microsoft/go-mssqldb/namedpipe"
)

func run(dsn string) (err error) {
	backend, err := sqlserver.Open(dsn)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, backend.DB().Close()) }()

	return readValue(backend, provider.Command{
		SQL:     "SELECT CAST(@value AS bigint) + 1",
		Slots:   []provider.Slot{{Name: "value", Kind: provider.KindInteger}},
		Columns: []provider.Column{{Kind: provider.KindInteger}},
	})
}
```

### MySQL

Use a running MySQL instance and an existing database. The sample connection string uses a disposable local instance. Set `GOEF_DSN` to the connection string for your instance.

```sh
go mod edit '-require=github.com/igorpooh1978/GoEF/providers/mysql@v0.1.0-alpha.1'
export GOEF_DSN='demo:password@tcp(127.0.0.1:3306)/demo'
```

Save the [MySQL example](examples/testdata/alpha-quickstart/mysql/adapter.go) as `adapter.go`.

```go
package main

import (
	"errors"

	"github.com/igorpooh1978/GoEF/provider"
	"github.com/igorpooh1978/GoEF/providers/mysql"
)

func run(dsn string) (err error) {
	backend, err := mysql.Open(dsn)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, backend.DB().Close()) }()

	return readValue(backend, provider.Command{
		SQL:     "SELECT CAST(? AS SIGNED) + 1",
		Slots:   []provider.Slot{{Kind: provider.KindInteger}},
		Columns: []provider.Column{{Kind: provider.KindInteger}},
	})
}
```

### Run your example

Once `main.go` and the `adapter.go` for your database are in place, run these commands from your application module.

```sh
go mod tidy
go run -mod=readonly .
```

Expected output

```text
42
```

If the declared column kind does not match the returned value, check for `goeferr.ErrMaterialization` with `errors.Is`. See the [published adapter report](docs/evidence/W002/PRODUCTION_ADAPTERS.md#tagged-adapter-previews) for the tested engine versions.

## Documentation

- [Documentation index](docs/README.md)
- [Code First mapping](docs/user/MODELS_CODE_FIRST.md)
- [Transactions](docs/user/TRANSACTIONS.md)
- [In-memory sequence operators](docs/user/LINQ_OBJECTS.md)
- [Master plan](docs/roadmap/MASTER_PLAN.md)
- [Project checkpoint](docs/roadmap/CHECKPOINT.md)
- [Contributing](CONTRIBUTING.md)
- [MIT License](LICENSE)
- [Bootstrap tracking issue](https://github.com/igorpooh1978/GoEF/issues/1)
