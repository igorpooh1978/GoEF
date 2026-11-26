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
