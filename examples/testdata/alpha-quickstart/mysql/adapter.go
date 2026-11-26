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
