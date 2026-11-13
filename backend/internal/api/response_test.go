package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// Every response, success or error, must carry time_ns as a string of digits.
func TestTimedAddsTimeNS(t *testing.T) {
	handlers := map[string]Handler{
		"success": func(r *http.Request) (int, Response) {
			return http.StatusOK, Response{"id": "geo_1"}
		},
		"error": func(r *http.Request) (int, Response) {
			return fail(http.StatusBadRequest, "name is required")
		},
		"nil body": func(r *http.Request) (int, Response) {
			return http.StatusOK, nil
		},
	}

	for name, handler := range handlers {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			Timed(handler)(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

			if got := recorder.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}

			var body map[string]any
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("response is not JSON: %v", err)
			}
			timeNS, ok := body["time_ns"].(string)
			if !ok {
				t.Fatalf("time_ns must be a string, got %T (%v)", body["time_ns"], body["time_ns"])
			}
			if n, err := strconv.ParseInt(timeNS, 10, 64); err != nil || n < 0 {
				t.Fatalf("time_ns must be a non-negative whole number, got %q", timeNS)
			}
		})
	}
}

func TestTimedKeepsStatusAndBody(t *testing.T) {
	recorder := httptest.NewRecorder()
	Timed(func(r *http.Request) (int, Response) {
		return fail(http.StatusNotFound, "vehicle veh_9 not found")
	})(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"error":"vehicle veh_9 not found"`) {
		t.Errorf("body missing error message: %s", recorder.Body.String())
	}
}

func TestDecodeJSON(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{"valid", `{"name":"Zone"}`, ""},
		{"empty", ``, "empty"},
		{"not json", `{name:`, "not valid JSON"},
		{"wrong type", `{"name":123}`, `field "name" has the wrong type`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dst struct {
				Name string `json:"name"`
			}
			err := decodeJSON(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body)), &dst)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("got error %v, want one containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestParsePage(t *testing.T) {
	tests := []struct {
		query      string
		wantLimit  int
		wantOffset int
		wantErr    bool
	}{
		{"", 0, 0, false},
		{"limit=10&offset=20", 10, 20, false},
		{"limit=9999", maxPageLimit, 0, false},
		{"limit=0", 0, 0, true},
		{"limit=abc", 0, 0, true},
		{"offset=-1", 0, 0, true},
	}
	for _, tt := range tests {
		page, err := parsePage(httptest.NewRequest(http.MethodGet, "/?"+tt.query, nil))
		if (err != nil) != tt.wantErr {
			t.Errorf("parsePage(%q) error = %v, wantErr %v", tt.query, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && (page.Limit != tt.wantLimit || page.Offset != tt.wantOffset) {
			t.Errorf("parsePage(%q) = %+v, want limit %d offset %d", tt.query, page, tt.wantLimit, tt.wantOffset)
		}
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:5555"
	if got := clientIP(r); got != "10.0.0.1" {
		t.Errorf("clientIP = %q, want 10.0.0.1", got)
	}

	r.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	if got := clientIP(r); got != "203.0.113.7" {
		t.Errorf("clientIP behind proxy = %q, want 203.0.113.7", got)
	}
}
