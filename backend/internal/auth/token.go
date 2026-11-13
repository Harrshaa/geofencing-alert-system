// Package auth creates and checks JSON Web Tokens (JWT) and hashes passwords.
//
// A JWT is three base64url-encoded parts joined by dots:
//
//	header.payload.signature
//
//	header    {"alg":"HS256","typ":"JWT"}            which algorithm signed it
//	payload   {"sub":"user_1","email":...,"exp":...}  the claims: who, and until when
//	signature HMAC-SHA256(header + "." + payload, secret key)
//
// The payload is only encoded, NOT encrypted: anyone can read it. The
// signature is what makes it trustworthy. Only the server knows the secret
// key, so only the server can produce a matching signature, and changing a
// single character of the header or payload makes the signature wrong.
//
// We only accept HS256. Rejecting every other "alg" value blocks the classic
// attack where a token claims "alg":"none" to skip the signature check.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrMalformed = errors.New("malformed token")
	ErrAlgorithm = errors.New("unsupported token algorithm")
	ErrSignature = errors.New("invalid token signature")
	ErrExpired   = errors.New("token expired")
)

// Claims is the payload of our tokens. The short names (sub, iat, exp) are
// the standard JWT claim names.
type Claims struct {
	Subject   string `json:"sub"`   // the user's ID
	Email     string `json:"email"` // for display
	IssuedAt  int64  `json:"iat"`   // when the token was made (Unix seconds)
	ExpiresAt int64  `json:"exp"`   // when it stops being valid (Unix seconds)
}

type header struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// The header never changes, so it is encoded once.
var encodedHeader = encodeJSON(header{Alg: "HS256", Typ: "JWT"})

// TokenManager issues and verifies tokens with one secret key.
type TokenManager struct {
	secret []byte
	ttl    time.Duration    // how long a new token stays valid
	now    func() time.Time // replaceable in tests
}

func NewTokenManager(secret []byte, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: secret, ttl: ttl, now: time.Now}
}

// Issue creates a signed token for a user and returns it with its expiry time.
func (m *TokenManager) Issue(userID, email string) (string, time.Time, error) {
	now := m.now()
	expiresAt := now.Add(m.ttl)

	payload, err := json.Marshal(Claims{
		Subject:   userID,
		Email:     email,
		IssuedAt:  now.Unix(),
		ExpiresAt: expiresAt.Unix(),
	})
	if err != nil {
		return "", time.Time{}, err
	}

	signingInput := encodedHeader + "." + base64.RawURLEncoding.EncodeToString(payload)
	token := signingInput + "." + m.sign(signingInput)
	return token, expiresAt, nil
}

// Verify checks a token and returns its claims. The checks run in this order:
//  1. it has exactly three parts
//  2. the header says HS256 (anything else is refused)
//  3. the signature matches (so nothing was changed)
//  4. only then is the payload trusted and read
//  5. it has not expired
func (m *TokenManager) Verify(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrMalformed
	}

	var h header
	if err := decodeJSON(parts[0], &h); err != nil {
		return nil, ErrMalformed
	}
	if h.Alg != "HS256" {
		return nil, ErrAlgorithm
	}

	// Compare signatures with hmac.Equal, which takes the same time whether
	// the first or the last byte differs. A normal comparison stops at the
	// first difference, and that timing difference can leak the signature.
	given, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrMalformed
	}
	expected, _ := base64.RawURLEncoding.DecodeString(m.sign(parts[0] + "." + parts[1]))
	if !hmac.Equal(given, expected) {
		return nil, ErrSignature
	}

	var claims Claims
	if err := decodeJSON(parts[1], &claims); err != nil || claims.Subject == "" {
		return nil, ErrMalformed
	}
	if m.now().Unix() >= claims.ExpiresAt {
		return nil, ErrExpired
	}
	return &claims, nil
}

// sign returns the base64url HMAC-SHA256 of the input, keyed with the secret.
func (m *TokenManager) sign(input string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func encodeJSON(v any) string {
	data, _ := json.Marshal(v) // only called with our own fixed structs
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeJSON(part string, dst any) error {
	data, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}
