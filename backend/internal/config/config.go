// Package config reads settings from environment variables.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port           string   // HTTP port to listen on
	DatabaseURL    string   // PostgreSQL connection string
	AllowedOrigins []string // frontend origins allowed by CORS and WebSocket; "*" allows all

	// Rate limit for POST /vehicles/location, per client IP.
	// RateLimitPerSecond <= 0 turns the limit off.
	RateLimitPerSecond float64
	RateLimitBurst     int

	// Authentication (JWT). Off by default, so every endpoint behaves exactly
	// as the assessment brief documents it. Set AUTH_REQUIRED=true to require
	// "Authorization: Bearer <token>" on every API call.
	AuthRequired  bool
	JWTSecret     string        // key that signs tokens; required when AuthRequired
	TokenTTL      time.Duration // how long a token stays valid
	AdminEmail    string        // account created (or updated) at startup
	AdminPassword string
}

func Load() Config {
	return Config{
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://geofence:geofence@localhost:5433/geofence?sslmode=disable"),
		AllowedOrigins:     splitList(getEnv("CORS_ALLOWED_ORIGINS", "*")),
		RateLimitPerSecond: getFloat("RATE_LIMIT_PER_SECOND", 20),
		RateLimitBurst:     getInt("RATE_LIMIT_BURST", 40),
		AuthRequired:       getBool("AUTH_REQUIRED", false),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		TokenTTL:           getDuration("JWT_TTL", 12*time.Hour),
		AdminEmail:         strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL"))),
		AdminPassword:      os.Getenv("ADMIN_PASSWORD"),
	}
}

func getBool(key string, fallback bool) bool {
	value, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(key)))
	if err != nil {
		return fallback
	}
	return value
}

// getDuration reads values such as "12h" or "30m".
func getDuration(key string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(strings.TrimSpace(os.Getenv(key)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getFloat(key string, fallback float64) float64 {
	value, err := strconv.ParseFloat(os.Getenv(key), 64)
	if err != nil {
		return fallback
	}
	return value
}

func getInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return value
}

// splitList turns "a, b,c" into ["a", "b", "c"].
func splitList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, strings.TrimRight(part, "/"))
		}
	}
	return out
}
