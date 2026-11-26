package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/provider"
	"github.com/igorpooh1978/GoEF/providers/sqlite"
)

func ExampleOpen() {
	file, err := os.CreateTemp("", "goef-adapter-*.db")
	if err != nil {
		panic(err)
	}
	defer os.Remove(file.Name())
	if err := file.Close(); err != nil {
		panic(err)
	}

	backend, err := sqlite.Open(file.Name())
	if err != nil {
		panic(err)
	}
	defer backend.DB().Close()
	ctx := context.Background()
	lease, err := backend.Acquire(ctx)
	if err != nil {
		panic(err)
	}
	defer lease.Release(ctx, provider.Reuse)

	command := provider.Command{
		SQL:     "SELECT ?",
		Slots:   []provider.Slot{{Kind: provider.KindText}},
		Columns: []provider.Column{{Kind: provider.KindText}},
	}
	rows, err := lease.Query(ctx, command, provider.Bindings{provider.Text("bound value")})
	if err != nil {
		panic(err)
	}
	row, err := rows.Next()
	if err != nil {
		panic(err)
	}
	fmt.Println(row[0])
	if _, err := rows.Next(); !errors.Is(err, io.EOF) {
		panic("expected one row")
	}
	if err := rows.Close(ctx); err != nil {
		panic(err)
	}

	command.Columns[0].Kind = provider.KindInteger
	rows, err = lease.Query(ctx, command, provider.Bindings{provider.Text("bound value")})
	if err != nil {
		panic(err)
	}
	defer rows.Close(ctx)
	_, err = rows.Next()
	fmt.Println(errors.Is(err, goeferr.ErrMaterialization))

	// Output:
	// bound value
	// true
}
