// Package api holds the HTTP handlers: it reads and validates requests,
// calls the store, and shapes the JSON responses.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"geofence-alerts/internal/store"
)

// Response is a JSON object. Handlers fill in their fields; Timed adds time_ns.
type Response map[string]any

// Handler is our handler shape. Instead of writing to the ResponseWriter,
// a handler returns a status code and a body. That lets one wrapper (Timed)
// add time_ns to EVERY response, including errors, so no handler can forget it.
type Handler func(r *http.Request) (int, Response)

// Timed turns a Handler into a normal http.HandlerFunc. It measures how long
// the handler ran and adds it to the body as "time_ns": nanoseconds, as a string.
func Timed(h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		status, body := h(r)
		if body == nil {
			body = Response{}
		}
		body["time_ns"] = strconv.FormatInt(time.Since(start).Nanoseconds(), 10)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false) // keep "<" and ">" readable in messages instead of <
		if err := encoder.Encode(body); err != nil {
			log.Printf("write response: %v", err)
		}
	}
}

// fail builds an error response: {"error": "...", "time_ns": "..."}.
func fail(status int, message string) (int, Response) {
	return status, Response{"error": message}
}

// serverError logs the real error and returns a generic 500, so internal
// details (SQL, table names) never reach the client.
func serverError(r *http.Request, err error) (int, Response) {
	log.Printf("ERROR %s %s: %v", r.Method, r.URL.Path, err)
	return fail(http.StatusInternalServerError, "internal server error")
}

const maxBodyBytes = 1 << 20 // 1 MB is far more than any valid request needs

// decodeJSON reads the request body into dst and returns a message that is
// safe and useful to show the client when the body is not valid.
func decodeJSON(r *http.Request, dst any) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
	if err != nil {
		return errors.New("could not read request body")
	}
	if len(body) > maxBodyBytes {
		return errors.New("request body is too large")
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return errors.New("request body is empty; expected a JSON object")
	}

	if err := json.Unmarshal(body, dst); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) && typeErr.Field != "" {
			return fmt.Errorf("field %q has the wrong type", typeErr.Field)
		}
		return errors.New("request body is not valid JSON")
	}
	return nil
}

const maxPageLimit = 500

// parsePage reads the optional ?limit= and ?offset= query parameters.
// Without ?limit= a list endpoint returns everything, as the brief describes.
func parsePage(r *http.Request) (store.Page, error) {
	var page store.Page
	query := r.URL.Query()

	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 {
			return page, errors.New("limit must be a positive whole number")
		}
		if limit > maxPageLimit {
			limit = maxPageLimit
		}
		page.Limit = limit
	}
	if raw := query.Get("offset"); raw != "" {
		offset, err := strconv.Atoi(raw)
		if err != nil || offset < 0 {
			return page, errors.New("offset must be zero or a positive whole number")
		}
		page.Offset = offset
	}
	return page, nil
}

// clientIP returns the caller's IP address, used as the rate-limit key.
// Behind a proxy (Render, a load balancer) the real client is the first
// entry of X-Forwarded-For; RemoteAddr would be the proxy itself.
func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		first, _, _ := strings.Cut(forwarded, ",")
		return strings.TrimSpace(first)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
