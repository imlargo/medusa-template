package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Connection pool defaults.
//
// The previous version passed an empty gorm.Config, which left Go's defaults in
// place: unlimited open connections and two idle ones. Unlimited is the harmful
// half — under load the pool opens as many connections as there are concurrent
// queries and hits Postgres' max_connections, where new connections are refused
// outright rather than queued.
const (
	// DefaultMaxOpenConns caps concurrent connections. Keep the total across all
	// replicas below the server's max_connections.
	DefaultMaxOpenConns = 25

	// DefaultMaxIdleConns keeps connections warm to avoid paying the handshake
	// on every request. Never above DefaultMaxOpenConns.
	DefaultMaxIdleConns = 10

	// DefaultConnMaxLifetime recycles connections so a load balancer or a
	// failover never leaves the pool holding a dead one indefinitely.
	DefaultConnMaxLifetime = time.Hour

	// DefaultConnMaxIdleTime closes connections nobody is using.
	DefaultConnMaxIdleTime = 10 * time.Minute

	// slowQueryThreshold is when gorm starts reporting a statement as slow.
	slowQueryThreshold = 200 * time.Millisecond
)

// PostgresConfig tunes the connection and the pool behind it.
//
// Record-not-found is never logged as an error: the service layer classifies it
// as a 404, so logging it would turn every miss into noise at error level.
type PostgresConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration

	// LogQueries turns on gorm's statement logging. Leave it off outside
	// development: it writes every statement, with its parameters, to stdout in
	// a format nothing downstream can parse, and those parameters are user data.
	LogQueries bool
}

// PostgresOption configures the connection.
type PostgresOption func(*PostgresConfig)

// WithMaxOpenConns caps concurrent connections.
func WithMaxOpenConns(n int) PostgresOption {
	return func(c *PostgresConfig) { c.MaxOpenConns = n }
}

// WithMaxIdleConns sets how many connections stay warm.
func WithMaxIdleConns(n int) PostgresOption {
	return func(c *PostgresConfig) { c.MaxIdleConns = n }
}

// WithConnMaxLifetime sets how long a connection may be reused.
func WithConnMaxLifetime(d time.Duration) PostgresOption {
	return func(c *PostgresConfig) { c.ConnMaxLifetime = d }
}

// WithConnMaxIdleTime sets how long an unused connection is kept.
func WithConnMaxIdleTime(d time.Duration) PostgresOption {
	return func(c *PostgresConfig) { c.ConnMaxIdleTime = d }
}

// WithQueryLogging enables gorm's statement logging. Development only.
func WithQueryLogging(enabled bool) PostgresOption {
	return func(c *PostgresConfig) { c.LogQueries = enabled }
}

// NewPostgresDatabase opens a connection pool and verifies it is usable.
//
// gorm.Open is lazy about some failures, so this pings the database before
// returning: a bad URL or an unreachable host should fail at startup, not on the
// first request that happens to need it.
func NewPostgresDatabase(url string, opts ...PostgresOption) (*gorm.DB, error) {
	cfg := PostgresConfig{
		MaxOpenConns:    DefaultMaxOpenConns,
		MaxIdleConns:    DefaultMaxIdleConns,
		ConnMaxLifetime: DefaultConnMaxLifetime,
		ConnMaxIdleTime: DefaultConnMaxIdleTime,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	logLevel := gormlogger.Warn
	if cfg.LogQueries {
		logLevel = gormlogger.Info
	}

	// gormlogger.Default is colourful, and ANSI escapes in a production log
	// stream are noise every aggregator has to strip. Colour follows the same
	// switch as statement logging: on for a terminal, off for a log pipeline.
	db, err := gorm.Open(postgres.Open(url), &gorm.Config{
		Logger: gormlogger.New(log.New(os.Stderr, "", log.LstdFlags), gormlogger.Config{
			SlowThreshold:             slowQueryThreshold,
			LogLevel:                  logLevel,
			Colorful:                  cfg.LogQueries,
			IgnoreRecordNotFoundError: true,
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	pool, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("access the connection pool: %w", err)
	}

	pool.SetMaxOpenConns(cfg.MaxOpenConns)
	pool.SetMaxIdleConns(min(cfg.MaxIdleConns, cfg.MaxOpenConns))
	pool.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	pool.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	if err := pool.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}
