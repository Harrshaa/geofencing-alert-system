package auth

import (
	"encoding/base64"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var testSecret = []byte("test-secret-at-least-32-bytes-long!!")

func newTestManager(clock *time.Time) *TokenManager {
	m := NewTokenManager(testSecret, time.Hour)
	m.now = func() time.Time { return *clock }
	return m
}

func TestIssueAndVerify(t *testing.T) {
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	m := newTestManager(&clock)

	token, expiresAt, err := m.Issue("user_1", "admin@example.com")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if parts := strings.Split(token, "."); len(parts) != 3 {
		t.Fatalf("token should have 3 parts (header.payload.signature), got %d", len(parts))
	}
	if want := clock.Add(time.Hour); !expiresAt.Equal(want) {
		t.Errorf("expiresAt = %v, want %v", expiresAt, want)
	}

	claims, err := m.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Subject != "user_1" || claims.Email != "admin@example.com" {
		t.Errorf("claims = %+v", claims)
	}
}

func TestHeaderIsHS256(t *testing.T) {
	clock := time.Now()
	token, _, _ := newTestManager(&clock).Issue("user_1", "a@b.c")
	raw, _ := base64.RawURLEncoding.DecodeString(strings.Split(token, ".")[0])
	if string(raw) != `{"alg":"HS256","typ":"JWT"}` {
		t.Errorf("header = %s", raw)
	}
}

func TestExpiredToken(t *testing.T) {
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	m := newTestManager(&clock)
	token, _, _ := m.Issue("user_1", "a@b.c")

	clock = clock.Add(59 * time.Minute)
	if _, err := m.Verify(token); err != nil {
		t.Fatalf("token should still be valid after 59 minutes: %v", err)
	}
	clock = clock.Add(time.Minute)
	if _, err := m.Verify(token); !errors.Is(err, ErrExpired) {
		t.Fatalf("token should be expired after 60 minutes, got %v", err)
	}
}

// Changing the payload (e.g. to pretend to be another user) must break the signature.
func TestTamperedPayloadIsRejected(t *testing.T) {
	clock := time.Now()
	m := newTestManager(&clock)
	token, _, _ := m.Issue("user_1", "a@b.c")
	parts := strings.Split(token, ".")

	forged := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"user_2","email":"x@y.z","iat":0,"exp":9999999999}`))
	if _, err := m.Verify(parts[0] + "." + forged + "." + parts[2]); !errors.Is(err, ErrSignature) {
		t.Fatalf("forged payload: got %v, want ErrSignature", err)
	}
}

func TestWrongSecretIsRejected(t *testing.T) {
	clock := time.Now()
	token, _, _ := newTestManager(&clock).Issue("user_1", "a@b.c")

	other := NewTokenManager([]byte("a-completely-different-secret-key!!!"), time.Hour)
	if _, err := other.Verify(token); !errors.Is(err, ErrSignature) {
		t.Fatalf("got %v, want ErrSignature", err)
	}
}

// The "alg":"none" attack: a token that claims to need no signature.
func TestAlgNoneIsRejected(t *testing.T) {
	clock := time.Now()
	m := newTestManager(&clock)
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	p := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"user_1","exp":9999999999}`))

	for _, token := range []string{h + "." + p + ".", h + "." + p + ".anything"} {
		if _, err := m.Verify(token); !errors.Is(err, ErrAlgorithm) {
			t.Errorf("Verify(%q) = %v, want ErrAlgorithm", token, err)
		}
	}
}

func TestMalformedTokens(t *testing.T) {
	clock := time.Now()
	m := newTestManager(&clock)
	for _, token := range []string{"", "abc", "a.b", "a.b.c.d", "!!!.###.$$$"} {
		if _, err := m.Verify(token); !errors.Is(err, ErrMalformed) {
			t.Errorf("Verify(%q) = %v, want ErrMalformed", token, err)
		}
	}
}

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "correct horse" || !strings.HasPrefix(hash, "$2") {
		t.Fatalf("hash does not look like bcrypt: %q", hash)
	}
	if !CheckPassword(hash, "correct horse") {
		t.Error("correct password should match")
	}
	if CheckPassword(hash, "wrong horse") {
		t.Error("wrong password should not match")
	}

	again, _ := HashPassword("correct horse")
	if again == hash {
		t.Error("two hashes of the same password should differ (random salt)")
	}
}

func TestValidatePassword(t *testing.T) {
	if ValidatePassword("short") == nil {
		t.Error("7-character password should be rejected")
	}
	if ValidatePassword("long enough") != nil {
		t.Error("11-character password should be accepted")
	}
	if ValidatePassword(strings.Repeat("x", 73)) == nil {
		t.Error("73-byte password should be rejected")
	}
}

func TestBearerToken(t *testing.T) {
	tests := map[string]string{
		"":                 "",
		"Bearer abc.def.g": "abc.def.g",
		"bearer abc":       "abc",
		"Basic abc":        "",
		"Bearer":           "",
	}
	for header, want := range tests {
		r := httptest.NewRequest("GET", "/", nil)
		if header != "" {
			r.Header.Set("Authorization", header)
		}
		if got := BearerToken(r); got != want {
			t.Errorf("BearerToken(%q) = %q, want %q", header, got, want)
		}
	}
}
