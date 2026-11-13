package middleware

import (
	"sync"
	"testing"
	"time"
)

func TestRateLimiterBurstThenRefill(t *testing.T) {
	clock := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	limiter := NewRateLimiter(2, 3) // 2 tokens per second, bucket of 3
	limiter.now = func() time.Time { return clock }

	// The bucket starts full: 3 requests pass, the 4th is rejected.
	for i := 1; i <= 3; i++ {
		if !limiter.Allow("1.1.1.1") {
			t.Fatalf("request %d should be allowed", i)
		}
	}
	if limiter.Allow("1.1.1.1") {
		t.Fatal("request 4 should be rejected: bucket is empty")
	}

	// Half a second later one token has refilled (2 per second).
	clock = clock.Add(500 * time.Millisecond)
	if !limiter.Allow("1.1.1.1") {
		t.Fatal("request should be allowed after refill")
	}
	if limiter.Allow("1.1.1.1") {
		t.Fatal("only one token should have refilled")
	}
}

func TestRateLimiterKeysAreIndependent(t *testing.T) {
	limiter := NewRateLimiter(1, 1)
	if !limiter.Allow("1.1.1.1") {
		t.Fatal("first client should be allowed")
	}
	if limiter.Allow("1.1.1.1") {
		t.Fatal("first client should now be limited")
	}
	if !limiter.Allow("2.2.2.2") {
		t.Fatal("a different client must have its own bucket")
	}
}

func TestRateLimiterDisabled(t *testing.T) {
	limiter := NewRateLimiter(0, 1)
	for i := 0; i < 100; i++ {
		if !limiter.Allow("1.1.1.1") {
			t.Fatal("a limiter with rate 0 must allow everything")
		}
	}
}

func TestRateLimiterCleanup(t *testing.T) {
	clock := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	limiter := NewRateLimiter(1, 1)
	limiter.now = func() time.Time { return clock }

	limiter.Allow("old")
	clock = clock.Add(10 * time.Minute)
	limiter.Allow("fresh")

	limiter.Cleanup(5 * time.Minute)
	if _, ok := limiter.buckets["old"]; ok {
		t.Error("idle bucket should have been removed")
	}
	if _, ok := limiter.buckets["fresh"]; !ok {
		t.Error("recent bucket should have been kept")
	}
}

// Run with `go test -race`: many goroutines hit the limiter at once.
// Without the mutex the race detector reports a data race on the map.
func TestRateLimiterConcurrentUse(t *testing.T) {
	limiter := NewRateLimiter(1000, 50)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				limiter.Allow("shared-key")
			}
		}()
	}
	wg.Wait()
}
