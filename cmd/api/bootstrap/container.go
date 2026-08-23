package bootstrap

import (
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
	"github.com/imlargo/medusa/pkg/medusa/core/ratelimiter"
	"github.com/imlargo/medusa/pkg/medusa/core/repository"
	"github.com/imlargo/medusa/pkg/medusa/core/service"
	"github.com/imlargo/medusa/pkg/medusa/services/cache"
	"github.com/imlargo/medusa/pkg/medusa/services/health"
	"github.com/imlargo/medusa/pkg/medusa/services/storage"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// healthCheckTimeout bounds how long the readiness endpoint waits for all
// dependency checks combined before reporting the app as unhealthy.
const healthCheckTimeout = 5 * time.Second

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
	RedisClient *redis.Client           // nil unless Redis is configured
	Cache       cache.Cache             // nil unless Redis is configured
	Storage     storage.FileStorage     // nil unless object storage is configured
	Metrics     metrics.MetricsService  // nil unless metrics are enabled
	RateLimiter ratelimiter.RateLimiter // nil unless rate limiting is enabled

	HealthService *health.Service

	Services *Services
	Handlers *Handlers

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
func NewContainer(cfg *config.Config, opts Options) (*Container, error) {
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

	if err := c.initInfrastructure(opts); err != nil {
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
func (c *Container) initInfrastructure(opts Options) error {
	cfg := c.Config
	log := c.Logger.Sugar()

	db, err := database.NewPostgresDatabase(cfg.Database.URL)
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
	c.JWT = jwt.NewJwt(jwt.Config{Secret: cfg.Auth.JwtSecret})

	if opts.WithRedis && cfg.Redis.Enabled() {
		redisClient, err := database.NewRedisClient(cfg.Redis.URL)
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
		c.Metrics = metrics.NewPrometheusMetrics()
	}

	if cfg.RateLimiter.Enabled {
		c.RateLimiter = ratelimiter.NewTokenBucketLimiter(ratelimiter.Config{
			RequestsPerTimeFrame: cfg.RateLimiter.RequestsPerTimeFrame,
			TimeFrame:            cfg.RateLimiter.TimeFrame,
		})
		log.Infow("rate limiter enabled",
			"requests", cfg.RateLimiter.RequestsPerTimeFrame,
			"per", cfg.RateLimiter.TimeFrame,
		)
	}

	return nil
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
	}
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
