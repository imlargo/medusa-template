// Package config resolves the application configuration from the process
// environment.
//
// Load reads every value once at startup, applies defaults for the optional
// ones and reports *all* problems it finds in a single error, so a misconfigured
// deployment fails immediately with a complete list instead of one variable at a
// time. The returned Config is read-only and safe to share across goroutines.
package config

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/imlargo/medusa/pkg/medusa/core/app"
	"github.com/imlargo/medusa/pkg/medusa/core/env"
	"github.com/imlargo/medusa/pkg/medusa/services/storage"
)

// Environment identifies the deployment the process is running in.
type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvStaging     Environment = "staging"
	EnvProduction  Environment = "production"
)

// IsProduction reports whether the app runs with production settings: release
// mode router, structured access logs and the stricter secret validation.
func (e Environment) IsProduction() bool { return e == EnvProduction }

// IsDevelopment reports whether the app runs with developer conveniences.
func (e Environment) IsDevelopment() bool { return e == EnvDevelopment }

// IsValid reports whether e is one of the known environments.
func (e Environment) IsValid() bool {
	switch e {
	case EnvDevelopment, EnvStaging, EnvProduction:
		return true
	default:
		return false
	}
}

// Config is the fully resolved application configuration.
type Config struct {
	app.Config

	Environment Environment
	CORS        CORSConfig
	RateLimiter RateLimiterConfig
	Redis       RedisConfig
	Storage     StorageConfig
}

// CORSConfig lists the origins allowed to call the API from a browser. The
// server's own host is always allowed and does not need to be repeated here.
type CORSConfig struct {
	AllowedOrigins []string
}

// RateLimiterConfig caps how many requests a single client IP may issue.
type RateLimiterConfig struct {
	Enabled              bool
	RequestsPerTimeFrame int
	TimeFrame            time.Duration
}

// RedisConfig points at the Redis instance backing the cache. Redis is optional:
// an empty URL disables it.
type RedisConfig struct {
	URL string
}

// Enabled reports whether Redis is configured.
func (c RedisConfig) Enabled() bool { return c.URL != "" }

// StorageConfig holds the object storage credentials and the provider to talk
// to. Storage is optional: an empty bucket name disables it.
type StorageConfig struct {
	storage.StorageConfig

	Provider storage.StorageProvider
}

// Enabled reports whether object storage is configured.
func (c StorageConfig) Enabled() bool { return c.BucketName != "" }

// Load reads the configuration from the environment. A .env file in the working
// directory is loaded first when present; it never overrides variables already
// exported in the environment.
func Load() (*Config, error) {
	env.LoadEnv()

	var l loader

	cfg := &Config{
		Config: app.Config{
			Server: app.ServerConfig{
				Host: l.text(envHost, defaultHost),
				Port: l.integer(envPort, defaultPort),
			},
			Database: app.DbConfig{
				URL: l.required(envDatabaseURL),
			},
			Auth: app.AuthConfig{
				JwtSecret:         l.required(envJWTSecret),
				JwtIssuer:         l.text(envJWTIssuer, defaultJWTIssuer),
				TokenExpiration:   l.duration(envJWTTokenExpiration, defaultTokenExpiration),
				RefreshExpiration: l.duration(envJWTRefreshExpiration, defaultRefreshExpiration),
			},
		},
		Environment: Environment(strings.ToLower(l.text(envAppEnv, string(EnvDevelopment)))),
		CORS: CORSConfig{
			AllowedOrigins: l.list(envCORSAllowedOrigins),
		},
		RateLimiter: RateLimiterConfig{
			Enabled:              l.boolean(envRateLimiterEnabled, defaultRateLimiterEnabled),
			RequestsPerTimeFrame: l.integer(envRateLimiterRequests, defaultRateLimiterRequests),
			TimeFrame:            l.duration(envRateLimiterTimeFrame, defaultRateLimiterTimeFrame),
		},
		Redis: RedisConfig{
			URL: l.text(envRedisURL, ""),
		},
		Storage: StorageConfig{
			Provider: storage.StorageProvider(strings.ToLower(l.text(envStorageProvider, string(storage.StorageProviderR2)))),
			StorageConfig: storage.StorageConfig{
				BucketName:      l.text(envStorageBucketName, ""),
				AccountID:       l.text(envStorageAccountID, ""),
				AccessKeyID:     l.text(envStorageAccessKeyID, ""),
				SecretAccessKey: l.text(envStorageSecretAccessKey, ""),
				PublicDomain:    l.text(envStoragePublicDomain, ""),
				UsePublicURL:    l.boolean(envStorageUsePublicURL, false),
			},
		},
	}

	if err := errors.Join(append(l.errs, cfg.Validate())...); err != nil {
		return nil, fmt.Errorf("invalid configuration:\n%w", err)
	}

	return cfg, nil
}

// Validate checks the invariants that survive parsing and returns every
// violation joined together.
func (c *Config) Validate() error {
	var errs []error

	if !c.Environment.IsValid() {
		errs = append(errs, fmt.Errorf("%s: unknown environment %q, want one of %q, %q, %q",
			envAppEnv, c.Environment, EnvDevelopment, EnvStaging, EnvProduction))
	}

	if c.Server.Port < 1 || c.Server.Port > 65535 {
		errs = append(errs, fmt.Errorf("%s: %d is out of the valid port range 1-65535", envPort, c.Server.Port))
	}

	if c.Auth.TokenExpiration <= 0 {
		errs = append(errs, fmt.Errorf("%s: must be positive, got %s", envJWTTokenExpiration, c.Auth.TokenExpiration))
	}

	// A refresh token that expires before the access token it renews can never
	// be used, which surfaces as users being logged out at random.
	if c.Auth.RefreshExpiration <= c.Auth.TokenExpiration {
		errs = append(errs, fmt.Errorf("%s (%s) must be longer than %s (%s)",
			envJWTRefreshExpiration, c.Auth.RefreshExpiration, envJWTTokenExpiration, c.Auth.TokenExpiration))
	}

	if c.Environment.IsProduction() && len(c.Auth.JwtSecret) < minJWTSecretLength {
		errs = append(errs, fmt.Errorf("%s: must be at least %d characters in %s", envJWTSecret, minJWTSecretLength, EnvProduction))
	}

	if c.RateLimiter.Enabled {
		if c.RateLimiter.RequestsPerTimeFrame <= 0 {
			errs = append(errs, fmt.Errorf("%s: must be positive when the rate limiter is enabled, got %d",
				envRateLimiterRequests, c.RateLimiter.RequestsPerTimeFrame))
		}
		if c.RateLimiter.TimeFrame <= 0 {
			errs = append(errs, fmt.Errorf("%s: must be positive when the rate limiter is enabled, got %s",
				envRateLimiterTimeFrame, c.RateLimiter.TimeFrame))
		}
	}

	errs = append(errs, c.Storage.validate())

	return errors.Join(errs...)
}

// validate reports partially configured storage. Storage is opt-in, but half a
// set of credentials is always a mistake and would otherwise fail on first use.
func (c StorageConfig) validate() error {
	credentials := map[string]string{
		envStorageAccountID:       c.AccountID,
		envStorageAccessKeyID:     c.AccessKeyID,
		envStorageSecretAccessKey: c.SecretAccessKey,
	}

	var missing []string
	for key, value := range credentials {
		if value == "" {
			missing = append(missing, key)
		}
	}

	// Nothing configured at all: storage stays disabled.
	if !c.Enabled() && len(missing) == len(credentials) {
		return nil
	}

	var errs []error

	if !c.Enabled() {
		errs = append(errs, fmt.Errorf("%s is required when storage credentials are set", envStorageBucketName))
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		errs = append(errs, fmt.Errorf("storage is configured but %s are missing", strings.Join(missing, ", ")))
	}
	if !c.Provider.IsValid() {
		errs = append(errs, fmt.Errorf("%s: unsupported provider %q", envStorageProvider, c.Provider))
	}

	return errors.Join(errs...)
}

// loader reads typed values from the environment and collects the parse errors
// so a single Load reports every malformed variable at once.
type loader struct {
	errs []error
}

// text returns the trimmed value of key, or fallback when it is unset or empty.
func (l *loader) text(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

// required returns the trimmed value of key and records an error when unset.
func (l *loader) required(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		l.errs = append(l.errs, fmt.Errorf("%s is required", key))
	}
	return value
}

// integer parses key as a base-10 integer, falling back when it is unset and
// recording an error when it is set but malformed.
func (l *loader) integer(key string, fallback int) int {
	raw := l.text(key, "")
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		l.errs = append(l.errs, fmt.Errorf("%s: %q is not a valid integer", key, raw))
		return fallback
	}
	return value
}

// boolean parses key as a boolean ("true", "1", "false", "0", ...).
func (l *loader) boolean(key string, fallback bool) bool {
	raw := l.text(key, "")
	if raw == "" {
		return fallback
	}

	value, err := strconv.ParseBool(raw)
	if err != nil {
		l.errs = append(l.errs, fmt.Errorf("%s: %q is not a valid boolean", key, raw))
		return fallback
	}
	return value
}

// duration parses key as a Go duration such as "15m", "168h" or "1h30m".
func (l *loader) duration(key string, fallback time.Duration) time.Duration {
	raw := l.text(key, "")
	if raw == "" {
		return fallback
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		l.errs = append(l.errs, fmt.Errorf("%s: %q is not a valid duration, want a value like %q or %q", key, raw, "15m", "168h"))
		return fallback
	}
	return value
}

// list splits key on commas, trimming each entry and dropping empty ones.
// Returns nil when the variable is unset.
func (l *loader) list(key string) []string {
	raw := l.text(key, "")
	if raw == "" {
		return nil
	}

	var values []string
	for entry := range strings.SplitSeq(raw, ",") {
		if entry = strings.TrimSpace(entry); entry != "" {
			values = append(values, entry)
		}
	}
	return values
}
