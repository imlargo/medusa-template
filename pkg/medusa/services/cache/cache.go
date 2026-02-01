package cache

import (
	"context"
	"time"
)

// Cache defines the interface for Redis cache operations
type Cache interface {
	// Get retrieves a value from cache and deserializes it into dest
	Get(ctx context.Context, key string, dest interface{}) error

	// Set stores a value in cache with optional TTL (0 = no expiration)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// Delete removes a key from cache
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in cache
	Exists(ctx context.Context, key string) (bool, error)

	// Clear flushes the entire cache (use with caution)
	Clear(ctx context.Context) error

	// Remember gets from cache or executes fn, stores and returns the result
	Remember(ctx context.Context, key string, ttl time.Duration, dest interface{}, fn func() (interface{}, error)) error

	// GetOrSet retrieves a value or sets a default if it doesn't exist
	GetOrSet(ctx context.Context, key string, defaultValue interface{}, ttl time.Duration, dest interface{}) error

	// SetMultiple stores multiple key-value pairs
	SetMultiple(ctx context.Context, items map[string]interface{}, ttl time.Duration) error

	// GetMultiple retrieves multiple values
	GetMultiple(ctx context.Context, keys []string) (map[string]interface{}, error)

	// DeletePattern removes all keys matching the pattern
	DeletePattern(ctx context.Context, pattern string) (int64, error)

	// Increment increments a counter
	Increment(ctx context.Context, key string, amount int64) (int64, error)

	// Decrement decrements a counter
	Decrement(ctx context.Context, key string, amount int64) (int64, error)

	// TTL gets the remaining time to live of a key
	TTL(ctx context.Context, key string) (time.Duration, error)

	// Expire sets or updates the expiration time of a key
	Expire(ctx context.Context, key string, ttl time.Duration) error
}
