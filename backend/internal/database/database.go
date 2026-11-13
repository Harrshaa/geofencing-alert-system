// Package database opens the PostgreSQL connection pool and creates the schema.
package database

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// schemaSQL is schema.sql compiled into the binary, so the Docker image
// does not need to ship the .sql file separately.
//
//go:embed schema.sql
var schemaSQL string

// Connect opens a connection pool. The database container can take a few
// seconds to accept connections, so it retries before giving up.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	const attempts = 30

	var lastErr error
	for i := 1; i <= attempts; i++ {
		pool, err := pgxpool.New(ctx, databaseURL)
		if err == nil {
			err = pool.Ping(ctx)
			if err == nil {
				return pool, nil
			}
			pool.Close()
		}
		lastErr = err
		log.Printf("database not ready (attempt %d/%d): %v", i, attempts, err)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return nil, fmt.Errorf("could not connect to database: %w", lastErr)
}

// Migrate creates the tables and indexes if they do not exist yet.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}
