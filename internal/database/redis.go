package database

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis client defaults.
const (
	DefaultRedisPoolSize     = 10
	DefaultRedisMinIdleConns = 5
	DefaultRedisMaxIdleConns = 10

	DefaultRedisDialTimeout  = 10 * time.Second
	DefaultRedisReadTimeout  = 3 * time.Second
	DefaultRedisWriteTimeout = 3 * time.Second
	DefaultRedisPoolTimeout  = 4 * time.Second

	// defaultRedisPingTimeout bounds the connectivity check at startup.
	defaultRedisPingTimeout = 5 * time.Second
)

// RedisConfig tunes the client and its connection pool.
type RedisConfig struct {
	DB           int
	PoolSize     int
	MinIdleConns int
	MaxIdleConns int

	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolTimeout  time.Duration
}

// RedisOption configures the client.
type RedisOption func(*RedisConfig)

// WithRedisDB selects the logical database index.
func WithRedisDB(db int) RedisOption {
	return func(c *RedisConfig) { c.DB = db }
}

// WithRedisPoolSize caps concurrent connections.
func WithRedisPoolSize(n int) RedisOption {
	return func(c *RedisConfig) { c.PoolSize = n }
}

// WithRedisTimeouts overrides the dial, read and write timeouts together.
func WithRedisTimeouts(dial, read, write time.Duration) RedisOption {
	return func(c *RedisConfig) {
		c.DialTimeout = dial
		c.ReadTimeout = read
		c.WriteTimeout = write
	}
}

// NewRedisClient parses a redis:// or rediss:// URL and returns a connected
// client, verifying connectivity before it returns.
//
// It logs nothing. A constructor that prints has no way to reach the
// application's logger, so the previous version used fmt.Printf and log.Println
// straight to stdout — outside the structured log stream, and announcing whether
// a password was configured. The caller owns that decision and already logs the
// outcome.
func NewRedisClient(ctx context.Context, redisURL string, opts ...RedisOption) (*redis.Client, error) {
	cfg := RedisConfig{
		PoolSize:     DefaultRedisPoolSize,
		MinIdleConns: DefaultRedisMinIdleConns,
		MaxIdleConns: DefaultRedisMaxIdleConns,
		DialTimeout:  DefaultRedisDialTimeout,
		ReadTimeout:  DefaultRedisReadTimeout,
		WriteTimeout: DefaultRedisWriteTimeout,
		PoolTimeout:  DefaultRedisPoolTimeout,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	parsed, err := url.Parse(redisURL)
	if err != nil {
		// The URL carries the password, so it must never reach the message.
		return nil, fmt.Errorf("parse redis url: %w", errWithoutURL(err))
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("parse redis url: no host in the connection string")
	}

	password, _ := parsed.User.Password()

	options := &redis.Options{
		Addr:     parsed.Host,
		Username: parsed.User.Username(),
		Password: password,
		DB:       cfg.DB,

		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxIdleConns: cfg.MaxIdleConns,

		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolTimeout:  cfg.PoolTimeout,
	}

	// rediss:// means TLS. The zero config is deliberate: it uses the system
	// roots and verifies the certificate, which is the whole point of choosing
	// the scheme.
	if parsed.Scheme == "rediss" {
		options.TLSConfig = &tls.Config{ServerName: parsed.Hostname()}
	}

	client := redis.NewClient(options)

	pingCtx, cancel := context.WithTimeout(ctx, defaultRedisPingTimeout)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		// Close the client we are not returning, or its pool goroutines leak.
		_ = client.Close()
		return nil, fmt.Errorf("connect to redis at %s: %w", parsed.Host, err)
	}

	return client, nil
}

// errWithoutURL strips the URL from a *url.Error, whose Error() includes the
// original string and therefore the credentials in it.
func errWithoutURL(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return fmt.Errorf("%s: %w", urlErr.Op, urlErr.Err)
	}

	return err
}
