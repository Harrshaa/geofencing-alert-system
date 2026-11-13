package api

import (
	"net/http"

	"geofence-alerts/internal/auth"
	"geofence-alerts/internal/middleware"
	"geofence-alerts/internal/realtime"
	"geofence-alerts/internal/store"
)

// API bundles what the handlers need.
type API struct {
	store        *store.Store
	hub          *realtime.Hub
	limiter      *middleware.RateLimiter // location updates
	loginLimiter *middleware.RateLimiter // login attempts (stricter)
	tokens       *auth.TokenManager
	authRequired bool
}

// Deps is everything New needs, named so the call site is readable.
type Deps struct {
	Store        *store.Store
	Hub          *realtime.Hub
	Limiter      *middleware.RateLimiter
	LoginLimiter *middleware.RateLimiter
	Tokens       *auth.TokenManager
	AuthRequired bool
}

func New(d Deps) *API {
	return &API{
		store:        d.Store,
		hub:          d.Hub,
		limiter:      d.Limiter,
		loginLimiter: d.LoginLimiter,
		tokens:       d.Tokens,
		authRequired: d.AuthRequired,
	}
}

// Routes registers every endpoint. Paths and methods match the brief exactly.
// "{vehicle_id}" is a path parameter (Go 1.22+ standard library routing).
//
// Every brief endpoint is wrapped in protected(...). With AUTH_REQUIRED=false
// (the default) that wrapper does nothing.
func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()

	// Geofences
	mux.HandleFunc("POST /geofences", Timed(a.protected(a.createGeofence)))
	mux.HandleFunc("GET /geofences", Timed(a.protected(a.listGeofences)))

	// Vehicles
	mux.HandleFunc("POST /vehicles", Timed(a.protected(a.createVehicle)))
	mux.HandleFunc("GET /vehicles", Timed(a.protected(a.listVehicles)))

	// Locations (the update endpoint is rate limited)
	mux.HandleFunc("POST /vehicles/location", Timed(a.rateLimited(a.limiter, a.protected(a.updateLocation))))
	mux.HandleFunc("GET /vehicles/location/{vehicle_id}", Timed(a.protected(a.getVehicleLocation)))

	// Alert rules
	mux.HandleFunc("POST /alerts/configure", Timed(a.protected(a.configureAlert)))
	mux.HandleFunc("GET /alerts", Timed(a.protected(a.listAlerts)))
	mux.HandleFunc("DELETE /alerts/{alert_id}", Timed(a.protected(a.deleteAlert))) // extra: lets the UI remove a rule

	// History
	mux.HandleFunc("GET /violations/history", Timed(a.protected(a.listViolations)))

	// Real-time alerts
	mux.HandleFunc("GET /ws/alerts", a.protectedWS(a.hub.ServeWS))

	// Authentication (extra). Login is public but rate limited against password guessing.
	mux.HandleFunc("POST /auth/login", Timed(a.rateLimited(a.loginLimiter, a.login)))
	mux.HandleFunc("GET /auth/me", Timed(a.me))
	mux.HandleFunc("GET /auth/status", Timed(a.authStatus))

	// Extra: health check for the hosting platform
	mux.HandleFunc("GET /health", Timed(a.health))

	// Anything else: JSON 404 (still with time_ns)
	mux.HandleFunc("/", Timed(func(r *http.Request) (int, Response) {
		return fail(http.StatusNotFound, "route not found: "+r.Method+" "+r.URL.Path)
	}))

	return mux
}

// rateLimited rejects a request with 429 when its client IP has used up its
// token bucket in the given limiter. It wraps a Handler, so the 429 response
// also gets time_ns.
func (a *API) rateLimited(limiter *middleware.RateLimiter, next Handler) Handler {
	return func(r *http.Request) (int, Response) {
		if !limiter.Allow(clientIP(r)) {
			return fail(http.StatusTooManyRequests, "rate limit exceeded; slow down and try again")
		}
		return next(r)
	}
}

func (a *API) health(r *http.Request) (int, Response) {
	return http.StatusOK, Response{"status": "ok"}
}
