package ratelimiter

import (
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// tokenBucketLimiter implements rate limiting using the token bucket algorithm.
// It maintains a separate rate limiter for each key (typically IP address or user ID).
//
// The token bucket algorithm works by:
//  1. Starting with a full bucket of tokens (RequestsPerTimeFrame)
//  2. Each request consumes one token
//  3. Tokens are replenished at a steady rate (TimeFrame / RequestsPerTimeFrame)
//  4. If no tokens are available, the request is rejected
//
// This implementation is thread-safe and automatically cleans up inactive entries
// to prevent unbounded memory growth.
type tokenBucketLimiter struct {
	sync.RWMutex
	config  Config
	entries map[string]*tblEntry
}

// tblEntry represents a single rate limiter entry for a specific key.
type tblEntry struct {
	Limiter  *rate.Limiter // The actual rate limiter
	LastSeen int64         // Unix nanoseconds of last request time
}

// NewTokenBucketLimiter creates a new token bucket rate limiter with the given configuration.
// It starts a background goroutine to clean up inactive entries.
//
// Example:
//
//	config := ratelimiter.Config{
//	    TimeFrame:            time.Minute,
//	    RequestsPerTimeFrame: 60, // 60 requests per minute = 1 per second
//	}
//	limiter := ratelimiter.NewTokenBucketLimiter(config)
func NewTokenBucketLimiter(cfg Config) RateLimiter {
	rl := &tokenBucketLimiter{
		config:  cfg,
		entries: make(map[string]*tblEntry),
	}

	go rl.cleanUpEntries()

	return rl
}

// getEntry retrieves or creates a rate limiter entry for the given key.
// This method is safe for concurrent use.
func (rl *tokenBucketLimiter) getEntry(key string) *tblEntry {
	rl.RLock()
	entry, exists := rl.entries[key]
	rl.RUnlock()

	if !exists {
		limiter := rate.NewLimiter(rate.Every(rl.config.TimeFrame), rl.config.RequestsPerTimeFrame)

		rl.Lock()
		// Check again in case another goroutine created it
		if entry, exists = rl.entries[key]; !exists {
			entry = &tblEntry{
				Limiter:  limiter,
				LastSeen: time.Now().UnixNano(),
			}
			rl.entries[key] = entry
		}
		rl.Unlock()
	}

	atomic.StoreInt64(&entry.LastSeen, time.Now().UnixNano())

	return entry
}

// Allow checks if a request for the given key should be allowed based on the rate limit.
//
// Returns:
//   - allowed: true if the request should be allowed, false if rate limit is exceeded
//   - tokens: the number of tokens currently available (useful for rate limit headers)
//
// This method is safe for concurrent use.
func (rl *tokenBucketLimiter) Allow(key string) (bool, float64) {
	entry := rl.getEntry(key)

	allowed := entry.Limiter.Allow()
	tokens := entry.Limiter.Tokens()

	return allowed, tokens
}

// cleanUpEntries runs in a background goroutine and periodically removes
// rate limiter entries that haven't been used recently.
//
// This prevents memory leaks from accumulating entries for clients that
// are no longer active. Entries are removed after 3 minutes of inactivity.
func (rl *tokenBucketLimiter) cleanUpEntries() {
	maxAge := 3 * time.Minute
	timeInterval := time.Minute

	for {
		time.Sleep(timeInterval)

		cutoffNano := time.Now().Add(-maxAge).UnixNano()

		rl.Lock()
		for key, entry := range rl.entries {
			if atomic.LoadInt64(&entry.LastSeen) < cutoffNano {
				delete(rl.entries, key)
			}
		}
		rl.Unlock()
	}
}
