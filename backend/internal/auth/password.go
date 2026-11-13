package auth

import (
	"errors"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Passwords are never stored. We store a bcrypt hash: a one-way scramble
// that includes a random salt and is deliberately slow to compute, so a
// stolen database cannot be turned back into passwords quickly.

const (
	MinPasswordLength = 8
	MaxPasswordLength = 72 // bcrypt only looks at the first 72 bytes
)

func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return errors.New("password must be at least 8 characters")
	}
	if len(password) > MaxPasswordLength {
		return errors.New("password must be at most 72 bytes")
	}
	return nil
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// CheckPassword reports whether password matches the stored hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// dummyHash is checked against when a login uses an unknown email. Without
// it, "unknown email" would answer instantly and "wrong password" would take
// as long as a bcrypt check, and that difference would reveal which emails
// have accounts.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("not-a-real-password"), bcrypt.DefaultCost)

// SpendPasswordCheckTime does the same work as CheckPassword and discards the result.
func SpendPasswordCheckTime(password string) {
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
}

// BearerToken returns the token from an "Authorization: Bearer <token>"
// header, or "" if there is none.
func BearerToken(r *http.Request) string {
	scheme, token, found := strings.Cut(r.Header.Get("Authorization"), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}
