package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/imlargo/medusa/internal/config"
	"github.com/imlargo/medusa/internal/database"
	"github.com/imlargo/medusa/internal/handlers"
	"github.com/imlargo/medusa/internal/services"
	"github.com/imlargo/medusa/internal/store"
	"github.com/imlargo/medusa/pkg/medusa/core/handler"
	"github.com/imlargo/medusa/pkg/medusa/core/jwt"
	"github.com/imlargo/medusa/pkg/medusa/core/logger"
	"github.com/imlargo/medusa/pkg/medusa/core/metrics"
	"github.com/imlargo/medusa/pkg/medusa/core/repository"
	"github.com/imlargo/medusa/pkg/medusa/core/service"
	"github.com/imlargo/medusa/pkg/medusa/services/cache"
	"github.com/imlargo/medusa/pkg/medusa/services/health"
	"github.com/imlargo/medusa/pkg/medusa/services/storage"
	"github.com/imlargo/ratelimit"
	ratelimitprom "github.com/imlargo/ratelimit/metrics/prometheus"
	"github.com/imlargo/sse"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	// healthCheckTimeout bounds how long the readiness endpoint waits for all
	// dependency checks combined before reporting the app as unhealthy.
	healthCheckTimeout = 5 * time.Second

	// eventRetention is how long a published event stays replayable, which is
	// the window a reconnecting client can resume from without losing events.
	eventRetention = 5 * time.Minute
)

// Container holds every dependency of the application.
//
// Optional components are nil when the matching Options flag is off or the
// configuration for them is absent; always check before use, or go through the
// Has* helpers.
type Container struct {
	Config *config.Config
	Logger *logger.Logger

	// Infrastructure, always present.
	DB    *gorm.DB
	Store *store.Store
	JWT   *jwt.JWT

	// Infrastructure, optional.
	RedisClient *redis.Client          // nil unless Redis is configured
	Cache       cache.Cache            // nil unless Redis is configured
	Storage     storage.FileStorage    // nil unless object storage is configured
	Metrics     metrics.MetricsService // nil unless metrics are enabled
	RateLimiter *ratelimit.Limiter     // nil unless rate limiting is enabled

	// Events publishes server-sent events. Always present: it is in-memory and
	// costs nothing until something subscribes.
	Events *sse.Broker

	// EventsLifecycle tracks open SSE sessions so they can be drained together
	// on shutdown. Without it they would be drained one connection at a time by
	// the HTTP server's timeout, which is to say not at all.
	EventsLifecycle *sse.Lifecycle

	HealthService *health.Service

	Services *Services
	Handlers *Handlers

	// MetricsRegistry is what the /metrics endpoint scrapes. nil when metrics
	// are disabled.
	MetricsRegistry *prometheus.Registry

	// closers releases resources in reverse order of acquisition.
	closers []func() error
}

// Services holds the application services.
type Services struct {
	User services.UserService
	Auth services.AuthService
	// Add more as needed:
	// Product services.ProductService
}

// Handlers holds the HTTP handlers.
type Handlers struct {
	Health *handler.HealthHandler
	Auth   *handlers.AuthHandler
	Events *handlers.EventsHandler
	// Add more as needed:
	// Product *handlers.ProductHandler
}

// Options selects which optional components to initialize. A component is only
// built when its flag is on *and* the corresponding configuration is present.
type Options struct {
	WithRedis   bool
	WithStorage bool
	WithMetrics bool
}

// DefaultOptions enables every optional component.
func DefaultOptions() Options {
	return Options{
		WithRedis:   true,
		WithStorage: true,
		WithMetrics: true,
	}
}

// MinimalOptions keeps only the database and JWT.
func MinimalOptions() Options {
	return Options{}
}

// NewContainer wires all dependencies. On failure it closes whatever was already
// opened, so the caller never has to clean up after an error.
//
// ctx bounds the connectivity checks performed while dialing dependencies.
func NewContainer(ctx context.Context, cfg *config.Config, opts Options) (*Container, error) {
	c := &Container{
		Config: cfg,
		Logger: logger.NewLogger(),
	}
	c.onClose(func() error {
		// Registered first so it runs last: every other closer may still log.
		// Sync fails whenever stderr is not a syncable file, which is the norm on
		// macOS and in containers, so the error carries no signal worth surfacing.
		_ = c.Logger.Sync()
		return nil
	})

	if err := c.initInfrastructure(ctx, opts); err != nil {
		// Best effort: the caller only gets the initialization error, so surface
		// any cleanup failure through the logger instead of swallowing it.
		if closeErr := c.Close(); closeErr != nil {
			c.Logger.Sugar().Errorw("failed to release partially initialized resources", "error", closeErr)
		}
		return nil, err
	}

	c.HealthService = c.buildHealthService()
	c.Services = c.buildServices()
	c.Handlers = c.buildHandlers()

	return c, nil
}

// initInfrastructure opens the external resources the app depends on.
func (c *Container) initInfrastructure(ctx context.Context, opts Options) error {
	cfg := c.Config
	log := c.Logger.Sugar()

	db, err := database.NewPostgresDatabase(cfg.Database.URL,
		// Statement logging only outside production: it writes user data to a
		// stream nothing can parse.
		database.WithQueryLogging(!cfg.Environment.IsProduction()),
	)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	c.DB = db
	c.onClose(func() error {
		sqlDB, err := db.DB()
		if err != nil {
			return fmt.Errorf("get sql database handle: %w", err)
		}
		return sqlDB.Close()
	})

	c.Store = store.NewStore(repository.NewStore(db, c.Logger))

	jwtAuth, err := jwt.NewJWT(jwt.Config{Secret: cfg.Auth.JwtSecret})
	if err != nil {
		return fmt.Errorf("initialize jwt: %w", err)
	}
	c.JWT = jwtAuth

	c.Events = sse.NewBroker("events", sse.NewMemoryLog(sse.Retention{For: eventRetention}))
	c.EventsLifecycle = sse.NewLifecycle()

	if opts.WithRedis && cfg.Redis.Enabled() {
		redisClient, err := database.NewRedisClient(ctx, cfg.Redis.URL)
		if err != nil {
			return fmt.Errorf("connect to redis: %w", err)
		}
		c.RedisClient = redisClient
		c.Cache = cache.NewRedisCache(redisClient)
		c.onClose(redisClient.Close)
		log.Infow("redis initialized")
	}

	if opts.WithStorage && cfg.Storage.Enabled() {
		fileStorage, err := storage.NewFileStorage(cfg.Storage.Provider, cfg.Storage.StorageConfig)
		if err != nil {
			return fmt.Errorf("initialize storage: %w", err)
		}
		c.Storage = fileStorage
		log.Infow("storage initialized", "provider", cfg.Storage.Provider, "bucket", cfg.Storage.BucketName)
	}

	if opts.WithMetrics {
		// A dedicated registry rather than the global default: two containers in
		// one process (which is every test that builds one) would otherwise
		// panic on duplicate collector registration.
		registry := prometheus.NewRegistry()
		metricsService, err := metrics.NewPrometheusMetrics(registry)
		if err != nil {
			return fmt.Errorf("initialize metrics: %w", err)
		}
		c.Metrics = metricsService
		c.MetricsRegistry = registry
	}

	if cfg.RateLimiter.Enabled {
		rateLimiter, err := buildRateLimiter(cfg.RateLimiter, c.MetricsRegistry)
		if err != nil {
			return fmt.Errorf("initialize rate limiter: %w", err)
		}
		c.RateLimiter = rateLimiter
		c.onClose(rateLimiter.Close)
		log.Infow("rate limiter enabled",
			"requests", cfg.RateLimiter.RequestsPerTimeFrame,
			"per", cfg.RateLimiter.TimeFrame,
			"auth_requests_per_minute", cfg.RateLimiter.AuthRequestsPerMinute,
			"keyed_by", rateLimiterKeyDescription(cfg.RateLimiter),
			"rules", rateLimiter.Rules(),
		)
	}

	return nil
}

// buildRateLimiter assembles the rule table.
//
// Two rules, because they answer different questions. The general one caps how
// much of the API any one caller may use. The auth one caps guesses at
// credentials, which is a volume attack and needs a much tighter bound than a
// browser loading a page does. Both apply to a request that matches both, and
// the tighter one governs.
//
// Everything the limiter needs to reject a request is decided here, at startup:
// a bad selector, an incoherent quota or a malformed proxy range fails now,
// with an error naming the problem, rather than on the first request.
func buildRateLimiter(cfg config.RateLimiterConfig, registry *prometheus.Registry) (*ratelimit.Limiter, error) {
	key := ratelimit.ByPeer()
	if len(cfg.TrustedProxies) > 0 {
		key = ratelimit.ByIP(cfg.TrustedProxies...)
	}

	rlCfg := ratelimit.Config{
		Rules: []ratelimit.Rule{
			{
				// Credential endpoints, by address. Not by account: an attacker
				// guessing passwords supplies a different account every time,
				// and would get a fresh counter for each.
				Name:     "auth",
				Selector: "POST /v1/auth/",
				Quota:    ratelimit.PerMinute(cfg.AuthRequestsPerMinute),
				Key:      key,
			},
			{
				Name:  "general",
				Quota: ratelimit.Per(cfg.RequestsPerTimeFrame, cfg.TimeFrame),
				Key:   key,
			},
		},
	}

	// Metrics matter more here than anywhere else in the application: this is
	// the one component whose job is to refuse requests, so "is it refusing the
	// wrong ones" has to be answerable. Compare denied against shadow_denied
	// before tightening a limit, and watch store_saturated, which is the only
	// way this component can refuse a caller for a reason unrelated to its
	// quota.
	if registry != nil {
		exporter, err := ratelimitprom.New(registry, ratelimitprom.Options{Namespace: "medusa"})
		if err != nil {
			return nil, fmt.Errorf("rate limiter metrics: %w", err)
		}
		rlCfg.Metrics = exporter.Metrics()
	}

	return ratelimit.NewWith(rlCfg)
}

// rateLimiterKeyDescription is for the startup log. Which address a limiter
// counts by is the thing most worth seeing in a log line, because getting it
// wrong is silent in one direction and obvious in the other.
func rateLimiterKeyDescription(cfg config.RateLimiterConfig) string {
	if len(cfg.TrustedProxies) == 0 {
		return "connection address (no trusted proxies declared, forwarding headers ignored)"
	}
	return fmt.Sprintf("client address behind trusted proxies %v", cfg.TrustedProxies)
}

// buildHealthService registers a readiness check per external dependency.
//
// Object storage is deliberately left out: the client validates connectivity at
// initialization and handles errors per request, so a periodic check would only
// add latency to the readiness probe.
func (c *Container) buildHealthService() *health.Service {
	svc := health.NewService(healthCheckTimeout)
	svc.RegisterChecker(health.NewDatabaseChecker(c.DB))

	if c.RedisClient != nil {
		svc.RegisterChecker(health.NewRedisChecker(c.RedisClient))
	}

	return svc
}

func (c *Container) buildServices() *Services {
	base := services.NewService(service.NewService(c.Logger), c.Store, c.Config)

	// AuthService reads user records through UserService during login and
	// registration, so UserService has to exist first.
	userService := services.NewUserService(base)

	return &Services{
		User: userService,
		Auth: services.NewAuthService(base, userService, c.JWT),
	}
}

func (c *Container) buildHandlers() *Handlers {
	base := handler.NewHandler(c.Logger)

	return &Handlers{
		Health: handler.NewHealthHandler(base, c.HealthService),
		Auth:   handlers.NewAuthHandler(base, c.Services.Auth),
		Events: handlers.NewEventsHandler(base, c.Events, c.EventsLifecycle, c.JWT),
	}
}

// DrainEventStreams ends every open SSE session.
//
// It has to run before the HTTP server shuts down, which is why it is wired as
// an onStop hook rather than a Close closer: an SSE response never completes on
// its own, and http.Server.Shutdown waits for in-flight requests. Without this,
// every deploy with a connected client would stall for the server's full
// shutdown timeout and then cut the streams anyway.
func (c *Container) DrainEventStreams(ctx context.Context) error {
	if c.EventsLifecycle == nil {
		return nil
	}

	open := c.EventsLifecycle.NodeSessionCount()
	if open == 0 {
		return nil
	}

	timeout := c.Config.Runtime.DrainTimeout()
	c.Logger.Sugar().Infow("draining event streams", "open", open, "timeout", timeout)

	drainCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := c.EventsLifecycle.Shutdown(drainCtx); err != nil {
		return fmt.Errorf("drain event streams: %w", err)
	}

	return nil
}

// onClose registers a cleanup function to run on Close.
func (c *Container) onClose(fn func() error) {
	c.closers = append(c.closers, fn)
}

// Close releases every resource the container holds, in reverse order of
// acquisition, and returns all failures joined together. It is idempotent.
func (c *Container) Close() error {
	var errs []error

	for i := len(c.closers) - 1; i >= 0; i-- {
		errs = append(errs, c.closers[i]())
	}
	c.closers = nil

	return errors.Join(errs...)
}

// HasCache reports whether a cache is available.
func (c *Container) HasCache() bool { return c.Cache != nil }

// HasStorage reports whether object storage is available.
func (c *Container) HasStorage() bool { return c.Storage != nil }
