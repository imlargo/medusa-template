package bootstrap

import (
	"context"
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

// Container holds all application dependencies.
type Container struct {
	Config *config.Config
	Logger *logger.Logger

	// Infrastructure (required)
	DB    *gorm.DB
	Store *store.Store
	JWT   *jwt.JWT

	// Infrastructure (optional components are pointers)
	RedisClient *redis.Client       // nil if Redis not configured
	Cache       cache.Cache         // nil if Redis not configured
	Storage     storage.FileStorage // nil if Storage not configured
	Metrics     metrics.MetricsService
	RateLimiter ratelimiter.RateLimiter // nil if disabled

	// Services
	HealthService *health.Service

	// Application layers
	Services *Services
	Handlers *Handlers
}

// Services holds all application services.
type Services struct {
	User services.UserService
	Auth services.AuthService
	// Add more as needed:
	// Product services.ProductService
}

// Handlers holds all HTTP handlers.
type Handlers struct {
	Health *handler.HealthHandler
	Auth   *handlers.AuthHandler
	// Add more as needed:
	// Product *handlers.ProductHandler
}

// Options configures which components to initialize.
type Options struct {
	WithRedis   bool
	WithStorage bool
	WithMetrics bool
}

// DefaultOptions returns options for a full-featured app.
func DefaultOptions() Options {
	return Options{
		WithRedis:   true,
		WithStorage: true,
		WithMetrics: true,
	}
}

// MinimalOptions returns options for a lightweight app.
func MinimalOptions() Options {
	return Options{
		WithRedis:   false,
		WithStorage: false,
		WithMetrics: false,
	}
}

// NewContainer creates and wires all dependencies.
func NewContainer(cfg *config.Config, opts Options) (*Container, error) {
	log := logger.NewLogger()
	c := &Container{
		Config: cfg,
		Logger: log,
	}

	// === Infrastructure ===

	// Database (required)
	db, err := database.NewPostgresDatabase(cfg.Database.URL)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}
	c.DB = db
	medusaStore := repository.NewStore(db, log)
	c.Store = store.NewStore(medusaStore)

	// JWT (required)
	c.JWT = jwt.NewJwt(jwt.Config{
		Secret: cfg.Auth.JwtSecret,
	})

	// Redis (optional)
	if opts.WithRedis && cfg.Redis.Url != "" {
		redisClient, err := database.NewRedisClient(cfg.Redis.Url)
		if err != nil {
			return nil, fmt.Errorf("redis: %w", err)
		}
		c.RedisClient = redisClient
		c.Cache = cache.NewRedisCache(redisClient)
		log.Info("Redis initialized")
	}

	// Storage (optional)
	if opts.WithStorage && cfg.Storage.BucketName != "" {
		fileStorage, err := storage.NewFileStorage(storage.StorageProviderR2, cfg.Storage)
		if err != nil {
			return nil, fmt.Errorf("storage: %w", err)
		}
		c.Storage = fileStorage
		log.Info("Storage initialized")
	}

	// Metrics (optional)
	if opts.WithMetrics {
		c.Metrics = metrics.NewPrometheusMetrics()
	}

	// Rate Limiter (based on config)
	if cfg.RateLimiter.Enabled {
		c.RateLimiter = ratelimiter.NewTokenBucketLimiter(ratelimiter.Config{
			RequestsPerTimeFrame: cfg.RateLimiter.RequestsPerTimeFrame,
			TimeFrame:            cfg.RateLimiter.TimeFrame,
		})
		log.Info("Rate limiter enabled")
	}

	// === Health Service ===
	c.HealthService = c.buildHealthService()

	// === Services ===
	c.Services = c.buildServices()

	// === Handlers ===
	c.Handlers = c.buildHandlers()

	return c, nil
}

func (c *Container) buildHealthService() *health.Service {
	svc := health.NewService(5 * time.Second)

	// Register database checker
	svc.RegisterChecker(health.NewDatabaseChecker(c.DB))

	// Register Redis checker if available
	if c.RedisClient != nil {
		svc.RegisterChecker(health.NewRedisChecker(c.RedisClient))
	}

	// Register storage checker if available
	if c.Storage != nil {
		svc.RegisterChecker(health.NewGenericChecker("storage", func(ctx context.Context) error {
			// Check if storage is configured by attempting a basic operation
			// This respects the context cancellation
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				// Storage is considered healthy if it's initialized
				// Real implementations could add actual checks like listing buckets
				return nil
			}
		}))
	}

	return svc
}

func (c *Container) buildServices() *Services {
	baseService := service.NewService(c.Logger)
	serviceBase := services.NewService(baseService, c.Store, c.Config)

	return &Services{
		User: services.NewUserService(serviceBase),
		Auth: services.NewAuthService(serviceBase, nil, c.JWT), // User injected below
	}
}

func (c *Container) buildHandlers() *Handlers {
	baseHandler := handler.NewHandler(c.Logger)

	// Rebuild auth service with user service (circular dep resolution)
	baseService := service.NewService(c.Logger)
	serviceBase := services.NewService(baseService, c.Store, c.Config)
	c.Services.Auth = services.NewAuthService(serviceBase, c.Services.User, c.JWT)

	return &Handlers{
		Health: handler.NewHealthHandler(baseHandler, c.HealthService),
		Auth:   handlers.NewAuthHandler(baseHandler, c.Services.Auth),
	}
}

// Cleanup releases resources. Call with defer.
func (c *Container) Cleanup() {
	c.Logger.Sync()
}

// HasCache returns true if cache is available.
func (c *Container) HasCache() bool {
	return c.Cache != nil
}

// HasStorage returns true if storage is available.
func (c *Container) HasStorage() bool {
	return c.Storage != nil
}
