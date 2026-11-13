package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"geofence-alerts/internal/auth"
)

// These tests need no database: they only exercise the token checks.

func testAPI(authRequired bool) *API {
	return &API{
		tokens:       auth.NewTokenManager([]byte("test-secret-at-least-32-bytes-long!!"), time.Hour),
		authRequired: authRequired,
	}
}

var okHandler Handler = func(r *http.Request) (int, Response) {
	return http.StatusOK, Response{"ok": true}
}

func call(h Handler, r *http.Request) (*httptest.ResponseRecorder, map[string]any) {
	recorder := httptest.NewRecorder()
	Timed(h)(recorder, r)
	var body map[string]any
	json.Unmarshal(recorder.Body.Bytes(), &body)
	return recorder, body
}

// With AUTH_REQUIRED=false the endpoints must behave exactly as before.
func TestProtectedDoesNothingWhenAuthIsOff(t *testing.T) {
	a := testAPI(false)
	recorder, _ := call(a.protected(okHandler), httptest.NewRequest("GET", "/geofences", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
}

func TestProtectedRequiresToken(t *testing.T) {
	a := testAPI(true)
	token, _, _ := a.tokens.Issue("user_1", "admin@example.com")

	tests := []struct {
		name       string
		header     string
		wantStatus int
		wantError  string
	}{
		{"no token", "", 401, "authentication required"},
		{"garbage token", "Bearer not.a.token", 401, "invalid token"},
		{"wrong scheme", "Basic " + token, 401, "authentication required"},
		{"valid token", "Bearer " + token, 200, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/geofences", nil)
			if tt.header != "" {
				r.Header.Set("Authorization", tt.header)
			}
			recorder, body := call(a.protected(okHandler), r)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			if _, ok := body["time_ns"].(string); !ok {
				t.Error("401 responses must still include time_ns")
			}
			if tt.wantError != "" {
				if msg, _ := body["error"].(string); !strings.Contains(msg, tt.wantError) {
					t.Errorf("error = %q, want it to contain %q", msg, tt.wantError)
				}
			}
		})
	}
}

func TestExpiredTokenMessage(t *testing.T) {
	a := testAPI(true)
	a.tokens = auth.NewTokenManager([]byte("test-secret-at-least-32-bytes-long!!"), time.Nanosecond)
	token, _, _ := a.tokens.Issue("user_1", "a@b.c")
	time.Sleep(1100 * time.Millisecond) // expiry is checked to the second

	r := httptest.NewRequest("GET", "/geofences", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	_, body := call(a.protected(okHandler), r)
	if msg, _ := body["error"].(string); !strings.Contains(msg, "expired") {
		t.Errorf("error = %q, want an expiry message", msg)
	}
}

func TestProtectedWSAcceptsQueryToken(t *testing.T) {
	a := testAPI(true)
	token, _, _ := a.tokens.Issue("user_1", "a@b.c")

	reached := false
	next := func(w http.ResponseWriter, r *http.Request) { reached = true }

	recorder := httptest.NewRecorder()
	a.protectedWS(next)(recorder, httptest.NewRequest("GET", "/ws/alerts", nil))
	if reached || recorder.Code != http.StatusUnauthorized {
		t.Fatalf("no token: reached=%v status=%d, want refused with 401", reached, recorder.Code)
	}

	recorder = httptest.NewRecorder()
	a.protectedWS(next)(recorder, httptest.NewRequest("GET", "/ws/alerts?token="+token, nil))
	if !reached {
		t.Fatal("valid ?token= should reach the WebSocket handler")
	}
}

func TestProtectedWSOpenWhenAuthIsOff(t *testing.T) {
	reached := false
	testAPI(false).protectedWS(func(w http.ResponseWriter, r *http.Request) { reached = true })(
		httptest.NewRecorder(), httptest.NewRequest("GET", "/ws/alerts", nil))
	if !reached {
		t.Fatal("with auth off, the WebSocket must stay open to everyone")
	}
}

func TestMeNeedsValidToken(t *testing.T) {
	a := testAPI(false) // /auth/me checks the token even when auth is optional
	token, _, _ := a.tokens.Issue("user_7", "ops@example.com")

	recorder, _ := call(a.me, httptest.NewRequest("GET", "/auth/me", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("no token: status = %d, want 401", recorder.Code)
	}

	r := httptest.NewRequest("GET", "/auth/me", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	recorder, body := call(a.me, r)
	user, _ := body["user"].(map[string]any)
	if recorder.Code != http.StatusOK || user["id"] != "user_7" || user["email"] != "ops@example.com" {
		t.Fatalf("status = %d body = %v", recorder.Code, body)
	}
}

func TestAuthStatus(t *testing.T) {
	for _, required := range []bool{true, false} {
		_, body := call(testAPI(required).authStatus, httptest.NewRequest("GET", "/auth/status", nil))
		if body["auth_required"] != required {
			t.Errorf("auth_required = %v, want %v", body["auth_required"], required)
		}
	}
}
