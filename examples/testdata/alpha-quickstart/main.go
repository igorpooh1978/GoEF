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
