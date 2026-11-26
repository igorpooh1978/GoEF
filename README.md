# GoEF

GoEF is a Go data access library in alpha. It uses Entity Framework Core as a behavioral reference. The preview includes Code First mapping, transactions, migration history validation, in-memory GoLINQ operators, and separate database adapters.

## Install the alpha

Use Go 1.27.4 or later. Pin the core version from your application's Go module.

```sh
go mod edit -require=github.com/igorpooh1978/GoEF@v0.1.0-alpha.1
```

Pin the adapter for your database.

```sh
go mod edit -require=github.com/igorpooh1978/GoEF/providers/postgres@v0.1.0-alpha.1
go mod edit -require=github.com/igorpooh1978/GoEF/providers/sqlite@v0.1.0-alpha.1
go mod edit -require=github.com/igorpooh1978/GoEF/providers/sqlserver@v0.1.0-alpha.1
go mod edit -require=github.com/igorpooh1978/GoEF/providers/mysql@v0.1.0-alpha.1
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
go mod edit -require=github.com/igorpooh1978/GoEF@v0.1.0-alpha.1 -require=github.com/igorpooh1978/GoEF/providers/sqlite@v0.1.0-alpha.1
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
