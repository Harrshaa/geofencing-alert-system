package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// User is a login account. PasswordHash is never sent to clients.
type User struct {
	ID           string
	Email        string
	PasswordHash string
}

// UserByEmail finds an account by its (lowercased) email.
func (s *Store) UserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, password_hash
		FROM users
		WHERE email = $1`, email).Scan(&u.ID, &u.Email, &u.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UpsertUser creates the account, or updates its password if the email
// already exists. Used at startup to create the admin from ADMIN_EMAIL and
// ADMIN_PASSWORD, so changing those settings and restarting changes the login.
func (s *Store) UpsertUser(ctx context.Context, email, passwordHash string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash
		RETURNING id`, email, passwordHash).Scan(&id)
	return id, err
}
