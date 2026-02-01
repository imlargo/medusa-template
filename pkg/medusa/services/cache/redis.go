// Package cache provides a Redis-based caching layer with a clean, type-safe interface.
//
// The cache service supports common caching patterns including:
//   - Basic get/set operations with TTL
//   - Cache-aside pattern with Remember()
//   - Batch operations for efficiency
//   - Atomic counters
//   - Pattern-based deletion
//
// All values are automatically serialized to/from JSON, providing type-safe caching
// for any JSON-serializable Go type.
//
// Example usage:
//
//	// Create cache client
//	redisClient := redis.NewClient(&redis.Options{
//	    Addr: "localhost:6379",
//	})
//	cache := cache.NewRedisCache(redisClient)
//
//	// Basic caching
//	user := User{ID: 1, Name: "John"}
//	cache.Set(ctx, "user:1", user, 1*time.Hour)
//
//	var cached User
//	if err := cache.Get(ctx, "user:1", &cached); err != nil {
//	    if err == cache.ErrKeyNotFound {
//	        // Handle cache miss
//	    }
//	}
//
//	// Cache-aside pattern
//	var user User
//	err := cache.Remember(ctx, "user:1", 1*time.Hour, &user, func() (interface{}, error) {
//	    return fetchUserFromDB(1)
//	})
//
// Thread Safety:
// All operations are thread-safe and can be called concurrently from multiple goroutines.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisCache implements the Service interface using Redis as the backend.
type redisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new Redis cache service with the provided client.
// The client should be pre-configured with connection settings, pooling, etc.
//
// Example:
//
//	client := redis.NewClient(&redis.Options{
//	    Addr:     "localhost:6379",
//	    Password: "", // no password
//	    DB:       0,  // default DB
//	})
//	cache := cache.NewRedisCache(client)
func NewRedisCache(client *redis.Client) Cache {
	return &redisCache{
		client: client,
	}
}

// Get retrieves a value from cache and deserializes it into dest.
// The dest parameter must be a pointer to the target type.
//
// Returns ErrKeyNotFound if the key doesn't exist.
// Returns ErrNilValue if dest is nil.
// Returns an error if deserialization fails.
//
// Example:
//
//	var user User
//	if err := cache.Get(ctx, "user:123", &user); err != nil {
//	    if err == cache.ErrKeyNotFound {
//	        // Key doesn't exist
//	    }
//	}
func (r *redisCache) Get(ctx context.Context, key string, dest interface{}) error {
	if dest == nil {
		return ErrNilValue
	}

	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return ErrKeyNotFound
		}
		return fmt.Errorf("failed to get key %s: %w", key, err)
	}

	if err := json.Unmarshal([]byte(val), dest); err != nil {
		return fmt.Errorf("failed to unmarshal value for key %s: %w", key, err)
	}

	return nil
}

// Set stores a value in cache with an optional TTL.
// The value is automatically serialized to JSON.
//
// Use ttl=0 for no expiration (key persists until explicitly deleted).
//
// Example:
//
//	user := User{ID: 1, Name: "John"}
//	cache.Set(ctx, "user:1", user, 1*time.Hour)     // Expires in 1 hour
//	cache.Set(ctx, "config:app", config, 0)         // Never expires
func (r *redisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value for key %s: %w", key, err)
	}

	if err := r.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set key %s: %w", key, err)
	}

	return nil
}

// Delete removes a key from the cache.
// Returns nil if the key doesn't exist (idempotent operation).
func (r *redisCache) Delete(ctx context.Context, key string) error {
	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete key %s: %w", key, err)
	}
	return nil
}

// Exists checks if a key exists in the cache.
// Returns true if the key exists, false otherwise.
func (r *redisCache) Exists(ctx context.Context, key string) (bool, error) {
	count, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check existence of key %s: %w", key, err)
	}
	return count > 0, nil
}

// Clear flushes the entire cache database.
//
// WARNING: This is a destructive operation that removes ALL keys in the current database.
// Use with extreme caution, especially in production environments.
func (r *redisCache) Clear(ctx context.Context) error {
	if err := r.client.FlushDB(ctx).Err(); err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}
	return nil
}

// Remember implements the cache-aside pattern.
// It tries to get the value from cache first. If not found, it executes fn,
// stores the result in cache, and returns it in dest.
//
// This is useful for expensive computations or database queries:
//
//	var user User
//	err := cache.Remember(ctx, "user:123", 1*time.Hour, &user, func() (interface{}, error) {
//	    return db.FindUser(123)
//	})
//
// If fn returns an error, the cache is not updated and the error is returned.
func (r *redisCache) Remember(ctx context.Context, key string, ttl time.Duration, dest interface{}, fn func() (interface{}, error)) error {
	if dest == nil {
		return ErrNilValue
	}

	// Try to get from cache first
	err := r.Get(ctx, key, dest)
	if err == nil {
		return nil // Value found in cache
	}

	if err != ErrKeyNotFound {
		return err // Real error occurred
	}

	// Value not in cache, execute function
	value, err := fn()
	if err != nil {
		return fmt.Errorf("function execution failed: %w", err)
	}

	// Store in cache
	if err := r.Set(ctx, key, value, ttl); err != nil {
		return err
	}

	// Copy value to dest
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal function result: %w", err)
	}

	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("failed to unmarshal function result: %w", err)
	}

	return nil
}

// GetOrSet retrieves a value from cache, or sets and returns a default value if the key doesn't exist.
// This is atomic from the client's perspective (though not a true Redis atomic operation).
//
// Example:
//
//	var counter int
//	cache.GetOrSet(ctx, "page:views", 0, 24*time.Hour, &counter)
func (r *redisCache) GetOrSet(ctx context.Context, key string, defaultValue interface{}, ttl time.Duration, dest interface{}) error {
	if dest == nil {
		return ErrNilValue
	}

	err := r.Get(ctx, key, dest)
	if err == nil {
		return nil // Value found
	}

	if err != ErrKeyNotFound {
		return err // Real error occurred
	}

	// Key not found, set default value
	if err := r.Set(ctx, key, defaultValue, ttl); err != nil {
		return err
	}

	// Copy default value to dest
	data, err := json.Marshal(defaultValue)
	if err != nil {
		return fmt.Errorf("failed to marshal default value: %w", err)
	}

	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("failed to unmarshal default value: %w", err)
	}

	return nil
}

// SetMultiple stores multiple key-value pairs efficiently using Redis pipelining.
// All operations are sent to Redis in a single network round-trip.
//
// Example:
//
//	items := map[string]interface{}{
//	    "user:1": User{ID: 1, Name: "John"},
//	    "user:2": User{ID: 2, Name: "Jane"},
//	}
//	cache.SetMultiple(ctx, items, 1*time.Hour)
func (r *redisCache) SetMultiple(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	if len(items) == 0 {
		return nil
	}

	pipe := r.client.Pipeline()

	for key, value := range items {
		data, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal value for key %s: %w", key, err)
		}
		pipe.Set(ctx, key, data, ttl)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to execute pipeline: %w", err)
	}

	return nil
}

// GetMultiple retrieves multiple values efficiently using Redis pipelining.
// Missing keys are silently skipped and not included in the result map.
//
// Example:
//
//	keys := []string{"user:1", "user:2", "user:3"}
//	values, err := cache.GetMultiple(ctx, keys)
//	// values contains only keys that exist in cache
func (r *redisCache) GetMultiple(ctx context.Context, keys []string) (map[string]interface{}, error) {
	if len(keys) == 0 {
		return make(map[string]interface{}), nil
	}

	pipe := r.client.Pipeline()
	cmds := make(map[string]*redis.StringCmd, len(keys))

	for _, key := range keys {
		cmds[key] = pipe.Get(ctx, key)
	}

	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to execute pipeline: %w", err)
	}

	result := make(map[string]interface{})
	for key, cmd := range cmds {
		val, err := cmd.Result()
		if err == redis.Nil {
			continue // Skip missing keys
		}
		if err != nil {
			return nil, fmt.Errorf("failed to get key %s: %w", key, err)
		}

		var data interface{}
		if err := json.Unmarshal([]byte(val), &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal value for key %s: %w", key, err)
		}
		result[key] = data
	}

	return result, nil
}

// DeletePattern removes all keys matching the given pattern.
// Uses SCAN internally to avoid blocking Redis on large datasets.
//
// Pattern syntax follows Redis glob-style patterns:
//   - * matches any sequence of characters
//   - ? matches a single character
//   - [abc] matches a, b, or c
//
// Example:
//
//	// Delete all user cache entries
//	count, err := cache.DeletePattern(ctx, "user:*")
//
//	// Delete all session keys for a specific user
//	count, err := cache.DeletePattern(ctx, "session:user123:*")
//
// Returns the number of keys deleted.
func (r *redisCache) DeletePattern(ctx context.Context, pattern string) (int64, error) {
	var cursor uint64
	var deletedCount int64
	const batchSize = 100

	for {
		keys, newCursor, err := r.client.Scan(ctx, cursor, pattern, batchSize).Result()
		if err != nil {
			return deletedCount, fmt.Errorf("failed to scan keys with pattern %s: %w", pattern, err)
		}

		if len(keys) > 0 {
			deleted, err := r.client.Del(ctx, keys...).Result()
			if err != nil {
				return deletedCount, fmt.Errorf("failed to delete keys: %w", err)
			}
			deletedCount += deleted
		}

		cursor = newCursor
		if cursor == 0 {
			break
		}
	}

	return deletedCount, nil
}

// Increment atomically increments a counter by the specified amount.
// If the key doesn't exist, it's created with the increment amount as the initial value.
//
// Example:
//
//	// Increment page views
//	views, err := cache.Increment(ctx, "page:home:views", 1)
//
//	// Add multiple points
//	score, err := cache.Increment(ctx, "user:123:score", 10)
//
// Returns the value after the increment.
func (r *redisCache) Increment(ctx context.Context, key string, amount int64) (int64, error) {
	result, err := r.client.IncrBy(ctx, key, amount).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment key %s: %w", key, err)
	}
	return result, nil
}

// Decrement atomically decrements a counter by the specified amount.
// If the key doesn't exist, it's created with the negative of the decrement amount as the initial value.
//
// Example:
//
//	// Decrement remaining quota
//	remaining, err := cache.Decrement(ctx, "user:123:quota", 1)
//
// Returns the value after the decrement.
func (r *redisCache) Decrement(ctx context.Context, key string, amount int64) (int64, error) {
	result, err := r.client.DecrBy(ctx, key, amount).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to decrement key %s: %w", key, err)
	}
	return result, nil
}

// TTL returns the remaining time to live for a key.
// Returns -1 if the key exists but has no expiration.
// Returns -2 if the key doesn't exist.
//
// Example:
//
//	ttl, err := cache.TTL(ctx, "session:abc123")
//	if ttl < 5*time.Minute {
//	    // Refresh session
//	}
func (r *redisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := r.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL for key %s: %w", key, err)
	}
	return ttl, nil
}

// Expire sets or updates the expiration time of a key.
// If the key doesn't exist, this operation has no effect.
//
// Example:
//
//	// Extend session expiration
//	cache.Expire(ctx, "session:abc123", 30*time.Minute)
//
//	// Set expiration on a persistent key
//	cache.Expire(ctx, "config:app", 24*time.Hour)
func (r *redisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if err := r.client.Expire(ctx, key, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set expiration for key %s: %w", key, err)
	}
	return nil
}
