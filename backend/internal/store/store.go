// Package store is the only package that talks to the database.
// Handlers call these methods; they never write SQL themselves.
package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Errors the API layer turns into HTTP status codes.
var (
	ErrNotFound         = errors.New("not found")
	ErrDuplicate        = errors.New("already exists")
	ErrInvalidPolygon   = errors.New("invalid polygon")
	ErrGeofenceNotFound = errors.New("geofence not found")
	ErrVehicleNotFound  = errors.New("vehicle not found")
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Page is optional pagination for list queries. Limit 0 means "no limit".
type Page struct {
	Limit  int
	Offset int
}

// isUniqueViolation reports whether err is PostgreSQL's "duplicate key" error.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
