package middleware

import (
	"sync"
	"time"
)

// RateLimiter is a token-bucket limiter with one bucket per key (we use the
// client IP as the key).
//
// How a token bucket works:
//   - Each key has a bucket that holds up to `burst` tokens.
//   - Tokens refill at `perSecond` tokens per second.
//   - Each request takes one token. If the bucket is empty, the request is
//     rejected.
//
// So a client may send a short burst, but its long-run rate is capped.
//
// Many requests run at the same time, each in its own goroutine, and they
// all read and write the buckets map. A Go map is not safe for that, so the
// mutex makes sure only one goroutine touches it at a time.
type RateLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*bucket
	perSecond float64
	burst     float64
	now       func() time.Time // replaceable in tests
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// NewRateLimiter creates a limiter. perSecond <= 0 disables limiting.
func NewRateLimiter(perSecond float64, burst int) *RateLimiter {
	if burst < 1 {
		burst = 1
	}
	return &RateLimiter{
		buckets:   make(map[string]*bucket),
		perSecond: perSecond,
		burst:     float64(burst),
		now:       time.Now,
	}
}

// Allow reports whether a request from key may proceed, and takes one token if so.
func (l *RateLimiter) Allow(key string) bool {
	if l.perSecond <= 0 {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		// First request from this key: start with a full bucket.
		b = &bucket{tokens: l.burst, lastSeen: now}
		l.buckets[key] = b
	}

	// Refill for the time that has passed, up to the bucket size.
	elapsed := now.Sub(b.lastSeen).Seconds()
	b.tokens += elapsed * l.perSecond
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.lastSeen = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Cleanup removes buckets that have not been used for maxIdle, so the map
// does not grow forever. Call it periodically.
func (l *RateLimiter) Cleanup(maxIdle time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := l.now().Add(-maxIdle)
	for key, b := range l.buckets {
		if b.lastSeen.Before(cutoff) {
			delete(l.buckets, key)
		}
	}
}
