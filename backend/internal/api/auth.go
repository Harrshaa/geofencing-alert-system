package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"geofence-alerts/internal/auth"
	"geofence-alerts/internal/store"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// POST /auth/login
// Checks email + password and returns a signed JWT.
func (a *API) login(r *http.Request) (int, Response) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		return fail(http.StatusBadRequest, err.Error())
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		return fail(http.StatusBadRequest, "email is required")
	}
	if req.Password == "" {
		return fail(http.StatusBadRequest, "password is required")
	}

	user, err := a.store.UserByEmail(r.Context(), email)
	if errors.Is(err, store.ErrNotFound) {
		// Same work and same message as a wrong password, so the response
		// does not reveal whether the email has an account.
		auth.SpendPasswordCheckTime(req.Password)
		return fail(http.StatusUnauthorized, "invalid email or password")
	}
	if err != nil {
		return serverError(r, err)
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		return fail(http.StatusUnauthorized, "invalid email or password")
	}

	token, expiresAt, err := a.tokens.Issue(user.ID, user.Email)
	if err != nil {
		return serverError(r, err)
	}
	return http.StatusOK, Response{
		"token":      token,
		"token_type": "Bearer",
		"expires_at": expiresAt.UTC(),
		"user":       Response{"id": user.ID, "email": user.Email},
	}
}

// GET /auth/me
// Returns who the token belongs to. Always needs a valid token.
func (a *API) me(r *http.Request) (int, Response) {
	claims, message := a.checkToken(auth.BearerToken(r))
	if message != "" {
		return fail(http.StatusUnauthorized, message)
	}
	return http.StatusOK, Response{
		"user":       Response{"id": claims.Subject, "email": claims.Email},
		"expires_at": time.Unix(claims.ExpiresAt, 0).UTC(),
	}
}

// GET /auth/status
// Tells the frontend whether it must show the login page.
func (a *API) authStatus(r *http.Request) (int, Response) {
	return http.StatusOK, Response{"auth_required": a.authRequired}
}

// protected guards a REST endpoint. With AUTH_REQUIRED=false it does nothing,
// so the endpoints behave exactly as the brief documents. With
// AUTH_REQUIRED=true a request without a valid token gets 401 (with time_ns,
// because protected sits inside Timed).
func (a *API) protected(next Handler) Handler {
	return func(r *http.Request) (int, Response) {
		if a.authRequired {
			if _, message := a.checkToken(auth.BearerToken(r)); message != "" {
				return fail(http.StatusUnauthorized, message)
			}
		}
		return next(r)
	}
}

// protectedWS guards the WebSocket endpoint. Browsers cannot add an
// Authorization header to a WebSocket, so the token may also come as
// /ws/alerts?token=... The check happens once, at the handshake.
// (The request log prints only the path, so the token is not written to logs.)
func (a *API) protectedWS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.authRequired {
			token := r.URL.Query().Get("token")
			if token == "" {
				token = auth.BearerToken(r)
			}
			if _, message := a.checkToken(token); message != "" {
				Timed(func(*http.Request) (int, Response) {
					return fail(http.StatusUnauthorized, message)
				})(w, r)
				return
			}
		}
		next(w, r)
	}
}

// checkToken verifies a token. It returns the claims, or an error message
// that is safe to show the client.
func (a *API) checkToken(token string) (*auth.Claims, string) {
	if token == "" {
		return nil, "authentication required: send the header Authorization: Bearer <token>"
	}
	claims, err := a.tokens.Verify(token)
	switch {
	case errors.Is(err, auth.ErrExpired):
		return nil, "token has expired; log in again"
	case err != nil:
		return nil, "invalid token"
	}
	return claims, ""
}
