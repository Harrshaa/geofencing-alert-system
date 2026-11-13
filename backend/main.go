// Geofencing and real-time alert API.
//
// Startup order: config -> database -> schema -> WebSocket hub -> HTTP server.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"geofence-alerts/internal/api"
	"geofence-alerts/internal/auth"
	"geofence-alerts/internal/config"
	"geofence-alerts/internal/database"
	"geofence-alerts/internal/middleware"
	"geofence-alerts/internal/realtime"
	"geofence-alerts/internal/store"
)

func main() {
	cfg := config.Load()

	// ctx is cancelled when the process is asked to stop (Ctrl+C or
	// `docker stop`). Everything long-running watches it.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool); err != nil {
		log.Fatalf("database: %v", err)
	}
	log.Println("database ready")

	// The hub runs in its own goroutine for the life of the server.
	hub := realtime.NewHub(cfg.AllowedOrigins)
	go hub.Run(ctx)

	limiter := middleware.NewRateLimiter(cfg.RateLimitPerSecond, cfg.RateLimitBurst)
	go cleanupLoop(ctx, limiter)

	// Login attempts: 1 per second per IP, bursts of 5. Slows password guessing.
	loginLimiter := middleware.NewRateLimiter(1, 5)
	go cleanupLoop(ctx, loginLimiter)

	st := store.New(pool)
	tokens := setupAuth(ctx, cfg, st)

	handler := api.New(api.Deps{
		Store:        st,
		Hub:          hub,
		Limiter:      limiter,
		LoginLimiter: loginLimiter,
		Tokens:       tokens,
		AuthRequired: cfg.AuthRequired,
	}).Routes()
	handler = middleware.CORS(cfg.AllowedOrigins)(handler)
	handler = middleware.Logging(handler)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	// Wait for the stop signal, then let in-flight requests finish.
	<-ctx.Done()
	log.Println("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

// setupAuth prepares JWT signing and the admin account.
//
//   - AUTH_REQUIRED=true needs JWT_SECRET; the server refuses to start without it.
//   - AUTH_REQUIRED=false with no JWT_SECRET: a random secret is generated, so
//     /auth/login still works, but tokens stop working after a restart.
//   - If ADMIN_EMAIL and ADMIN_PASSWORD are set, that account is created, or its
//     password updated, on every start.
func setupAuth(ctx context.Context, cfg config.Config, st *store.Store) *auth.TokenManager {
	secret := []byte(cfg.JWTSecret)
	if len(secret) == 0 {
		if cfg.AuthRequired {
			log.Fatal("auth: JWT_SECRET must be set when AUTH_REQUIRED=true")
		}
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			log.Fatalf("auth: generate secret: %v", err)
		}
	} else if len(secret) < 32 {
		log.Println("auth: warning: JWT_SECRET is shorter than 32 characters; use a long random value")
	}

	if cfg.AdminEmail != "" && cfg.AdminPassword != "" {
		if err := ensureAdmin(ctx, st, cfg.AdminEmail, cfg.AdminPassword); err != nil {
			log.Fatalf("auth: admin account: %v", err)
		}
		log.Printf("auth: admin account ready (%s)", cfg.AdminEmail)
	} else if cfg.AuthRequired {
		log.Println("auth: warning: ADMIN_EMAIL/ADMIN_PASSWORD not set; only existing accounts can log in")
	}

	if cfg.AuthRequired {
		log.Printf("auth: REQUIRED (tokens valid for %s)", cfg.TokenTTL)
	} else {
		log.Println("auth: optional (AUTH_REQUIRED=false); endpoints are open")
	}
	return auth.NewTokenManager(secret, cfg.TokenTTL)
}

func ensureAdmin(ctx context.Context, st *store.Store, email, password string) error {
	if !strings.Contains(email, "@") {
		return fmt.Errorf("ADMIN_EMAIL %q is not an email address", email)
	}
	if err := auth.ValidatePassword(password); err != nil {
		return fmt.Errorf("ADMIN_PASSWORD: %w", err)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	_, err = st.UpsertUser(ctx, email, hash)
	return err
}

// cleanupLoop removes idle rate-limit buckets once a minute.
func cleanupLoop(ctx context.Context, limiter *middleware.RateLimiter) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			limiter.Cleanup(5 * time.Minute)
		case <-ctx.Done():
			return
		}
	}
}
