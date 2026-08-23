<div align="center">
  <h1>🪼 Medusa</h1>
  <p><strong>A batteries-included Go framework for building modern, scalable backends</strong></p>
  
  [![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
  [![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
  
  <p>
    <a href="#features">Features</a> •
    <a href="#quick-start">Quick Start</a> •
    <a href="#architecture">Architecture</a> •
    <a href="#documentation">Documentation</a> •
    <a href="#roadmap">Roadmap</a>
  </p>
</div>

---

## What is Medusa?

Medusa is a **production-ready framework** for Go that eliminates the tedious setup of modern backend applications. It's not just another HTTP router it's a complete platform that integrates everything you need to build enterprise-grade systems.

**Stop wasting time on boilerplate.** Start building features on day one. 

### Built for

- ✅ REST APIs with authentication, validation, and rate limiting
- ✅ Real-time systems with SSE, WebSockets, and pub/sub
- ✅ Microservices architectures with clean boundaries
- ✅ SaaS platforms with storage, email, and notifications
- ✅ Event-driven applications with message queues
- ✅ Scalable backends with observability and metrics

### Design Philosophy

- **🔋 Batteries Included, Decisions Optional** Everything ready out-of-the-box, but never imposed
- **🧩 Modular & Composable** Use what you need, ignore what you don't
- **🏗️ Clean Architecture** SOLID principles without unnecessary ceremony
- **🚀 Fast to Ship, Built to Scale** Rapid iteration without technical debt
- **���� Pragmatic, Not Dogmatic** Sensible conventions, always flexible

---

## Features

### Core Framework

- **🎯 Application Lifecycle** Graceful shutdown, signal handling, context propagation
- **🏗️ Bootstrap Architecture** DI container with lifecycle hooks and optional components
- **🌐 HTTP Server** Built on Gin with extensible middleware and multiple server support
- **📝 Structured Logging** Production-ready logging with Zap
- **🗄️ Repository Pattern** Clean data layer abstractions with GORM
- **📊 Observability** Prometheus metrics, health checks, and monitoring
- **⚙️ Configuration** Environment-based config with validation
- **🔍 Request Tracking** Request ID propagation through all layers

### Enhanced Context & Error Handling

- **📦 Typed Context** Parameter extraction with automatic validation
- **✅ Validation Helpers** Built-in pagination, sorting, UUID validation
- **🎯 Authentication Helpers** `UserID()`, `IsAuthenticated()` for auth checks
- **📤 Response Helpers** `OK()`, `Created()`, `Error()`, `Paged()` for consistent responses
- **🚨 Structured Errors** HTTP-aware error system with request ID tracking
- **🔄 Error Wrapping** Support for error wrapping and unwrapping

### Authentication & Security

- **🔐 JWT Authentication** Token generation, validation, and refresh tokens
- **🔑 API Key Auth** Header and Bearer token strategies
- **🛡️ CORS** Configurable cross-origin policies
- **⏱️ Rate Limiting** Token bucket algorithm with per-IP limiting
- **🔒 Middleware Chain** Extensible security pipeline

### Services (The Batteries)

- **💾 Cache** Redis-backed distributed caching with clean interface
- **📦 Storage** Multi-provider file storage (S3, Cloudflare R2) with presigned URLs
- **🔔 Push Notifications** Web Push API integration
- **📡 Server-Sent Events** Real-time server-to-client streaming with client management
- **🗃️ Database** PostgreSQL with GORM and automatic migrations

---

## Quick Start

### Prerequisites

- Go 1.21 or higher
- PostgreSQL 14+
- Redis 7+

### Installation

```bash
# Clone the repository
git clone https://github.com/imlargo/medusa. git
cd medusa

# Install dependencies
go mod download

# Copy environment configuration
cp .env.example .env
# Edit .env with your database and Redis credentials

# Run the application
go run cmd/api/main.go
```

The server will start at `http://localhost:8080`

### Modern Bootstrap Architecture

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "os"

    "github.com/imlargo/medusa/cmd/api/bootstrap"
)

func main() {
    if err := run(context.Background()); err != nil {
        fmt.Fprintf(os.Stderr, "medusa-api: %v\n", err)
        os.Exit(1)
    }
}

// run owns the lifecycle so shutdown always happens: os.Exit — and therefore
// log.Fatal — would skip every deferred call.
func run(ctx context.Context) (err error) {
    // Bootstrap loads the config and wires every dependency.
    app, appErr := bootstrap.New("medusa-api")
    if appErr != nil {
        return appErr
    }
    defer func() {
        err = errors.Join(err, app.Close())
    }()

    // Run blocks until SIGINT/SIGTERM, then shuts down gracefully.
    return app.Run(ctx)
}
```

The bootstrap system automatically configures:
- Database connection with connection pooling
- Optional Redis for caching
- Optional object storage (S3/R2)
- JWT authentication
- Health checks (liveness at `/health`, readiness at `/ready`)
- Structured request logging
- Prometheus metrics at `/metrics` (restrict this at your ingress)
- Rate limiting over the whole `/v1` group, public endpoints included

`app.Close()` releases every resource in reverse order of acquisition and
returns the failures joined together.

### Enhanced Context Usage

```go
import "github.com/imlargo/medusa/pkg/medusa"

func GetUsers(c *gin.Context) {
    ctx := medusa.NewContext(c)
    
    // Type-safe parameter extraction with validation
    userID, err := ctx.ParamUUID("id")
    if err != nil {
        // Automatically returns 400 with validation error
        return
    }
    
    // Built-in pagination
    page := ctx.GetPage()      // Defaults to 1
    pageSize := ctx.GetPageSize() // Defaults to 20, max 100
    
    // Authentication helpers
    if !ctx.IsAuthenticated() {
        ctx.Error(medusa.ErrUnauthorized("Login required"))
        return
    }
    
    currentUserID, _ := ctx.UserID()
    
    // Query with context propagation
    users, err := userService.List(c.Request.Context(), page, pageSize)
    if err != nil {
        ctx.Error(medusa.Wrap(err, "Failed to fetch users"))
        return
    }
    
    // Paginated response with request ID tracking
    ctx.Paged(users, page, pageSize, totalCount)
}
```

```go
package main

import (
    "context"
    "github.com/gin-gonic/gin"
    "github.com/imlargo/medusa/pkg/medusa/core/app"
    "github.com/imlargo/medusa/pkg/medusa/core/logger"
    "github.com/imlargo/medusa/pkg/medusa/core/responses"
    "github.com/imlargo/medusa/pkg/medusa/core/server/http"
)

func main() {
    log := logger.NewLogger()
    defer log.Sync()

    router := gin.Default()
    srv := http.NewServer(router, log,
        http.WithServerHost("localhost"),
        http.WithServerPort("8080"),
    )

    app := app.NewApp(
        app. WithName("my-api"),
        app.WithServer(srv),
    )

    // Define your routes
    router.GET("/ping", func(c *gin.Context) {
        responses.WriteSuccess(c, http.StatusOK, responses.MessageOK, gin.H{"message": "pong"})
    })

    // Run with graceful shutdown
    app.Run(context.Background())
}
```

### Using Services

#### Cache with Redis

```go
import "github.com/imlargo/medusa/pkg/medusa/services/cache"

redisClient := database.NewRedisClient("redis://localhost:6379")
cache := cache.NewRedisCache(redisClient)

// Set value with expiration
cache.Set(ctx, "user:123", userData, 1*time.Hour)

// Get value
var user User
cache.Get(ctx, "user:123", &user)
```

#### File Storage (S3/R2)

```go
import "github.com/imlargo/medusa/pkg/medusa/services/storage"

storage, _ := storage.NewFileStorage(storage.StorageProviderR2, config)

// Upload file
file, _ := storage.Upload("avatars/user-123.jpg", fileReader, "image/jpeg", fileSize)
fmt. Println(file. Url) // Public URL

// Generate presigned URL for secure downloads
url, _ := storage.GetPresignedURL("documents/secret.pdf", 15*time.Minute)
```

#### Real-time with Server-Sent Events

SSE is provided by [github.com/imlargo/sse](https://github.com/imlargo/sse): a
broker publishes to topics, subscribers pick what they want, and the library owns
the write loop — headers, flushing, heartbeats, write deadlines, backpressure and
resumption after a reconnect.

```go
import "github.com/imlargo/sse"

// One broker for the whole app. Events stay replayable for five minutes, which
// is the window a reconnecting client can resume from.
broker := sse.NewBroker("events", sse.NewMemoryLog(sse.Retention{For: 5 * time.Minute}))

// The stream authenticates itself: the Authorizer runs on the raw request, so it
// both identifies the caller and confines them to their own topics.
router.GET("/v1/events", gin.WrapH(broker.Handler(sse.WithAuthorizer(authorize))))

func authorize(r *http.Request) (sse.Grant, error) {
    claims, err := jwtAuth.ParseToken(tokenFrom(r))
    if err != nil {
        return sse.Grant{}, sse.Unauthorized("invalid or expired token")
    }

    return sse.Grant{
        Identity: fmt.Sprintf("user:%d", claims.UserID),
        Filters:  []sse.Filter{sse.MustFilter(fmt.Sprintf("user.%d.>", claims.UserID))},
        // The session ends when the token does. The client reconnects with a
        // fresh one and resumes from its cursor, so nothing is missed.
        Deadline: claims.ExpiresAt.Time,
    }, nil
}

// Publish from wherever the event happens. This never touches a subscriber, so a
// slow client cannot slow down the publisher or anybody else.
broker.Publish(ctx, sse.MustTopic("user.123.notifications"),
    gin.H{"title": "New message"}, sse.Name("notification"))
```

See `internal/handlers/events.go` for the wired-up version, and the library's
README for backpressure policies, topic routing and running across many nodes.

#### Handlers return their result

Handlers do not touch `*gin.Context` or write responses. They return a value and
an error; the `medusa.Handle*` adapters bind and validate the body, pick the
status and render the result, so that logic lives in one place instead of in
every handler.

```go
func (h *UserHandler) Create(ctx *medusa.Context, in *dto.NewUser) (*models.User, error) {
    user, err := h.users.Create(ctx.Ctx(), in)
    if errors.Is(err, ErrEmailTaken) {
        return nil, responses.Conflict("email already registered")
    }
    return user, err
}

users.POST("", medusa.HandleCreate(h.Create))
```

The status comes from the error itself, so returning `responses.NotFound("user")`
produces a 404 without the handler naming a status code. An error nobody
classified becomes a 500 whose cause reaches the log and never the client.

| Adapter | Reads body | Success |
|---|---|---|
| `medusa.Handle` | yes | 200 |
| `medusa.HandleCreate` | yes | 201 |
| `medusa.HandleUpdate` | yes | 200 |
| `medusa.HandleGet` | no | 200 |
| `medusa.HandleDelete` | no | 200 |
| `medusa.Handler` | no | writes its own response |

#### Health Checks

The framework includes built-in health check endpoints with request ID tracking:

```go
// Liveness check - Returns 200 if process is running
GET /health
Response: {
    "status": "healthy",
    "request_id": "550e8400-e29b-41d4-a716-446655440000"
}

// Readiness check - Returns detailed dependency status
GET /ready
Response: {
    "status": "healthy",
    "checks": {
        "database": "healthy",
        "redis": "healthy"
    },
    "request_id": "550e8400-e29b-41d4-a716-446655440000"
}

// Returns 503 if dependencies are unhealthy with Retry-After header
```

The health service automatically checks:
- Database connectivity with configurable timeout
- Redis connectivity (if configured)
- Returns appropriate HTTP status codes for monitoring
- Includes Retry-After header for unhealthy responses

---

## Architecture

Medusa follows **Clean Architecture** principles with a pragmatic twist structure without bureaucracy. 

```
.
├── cmd/                       # Application entry points
│   ├── api/                  # Main HTTP server
│   ├── sse/                  # Dedicated SSE server (optional)
│   └── worker/               # Background workers (future)
│
├── internal/                  # Private application code
│   ├── config/               # Configuration management
│   ├── database/             # Database connections
│   ├── handlers/             # HTTP handlers (controllers)
│   ├── models/               # Domain entities
│   ├── repository/           # Data access layer
│   ├── service/              # Business logic
│   └── store/                # Repository composition
│
└── pkg/medusa/               # 🪼 THE FRAMEWORK (reusable)
    ├── core/                 # Core components
    │   ├── app/             # Application lifecycle
    │   ├── env/             # Environment utilities
    │   ├── handler/         # Base handler
    │   ├── jwt/             # JWT auth
    │   ├── logger/          # Structured logging
    │   ├── metrics/         # Observability
    │   ├── ratelimiter/     # Rate limiting
    │   ├── repository/      # Repository pattern
    │   ├── responses/       # HTTP response helpers
    │   └── server/          # Server abstractions
    │
    ├── middleware/           # HTTP middleware
    │   ├── auth. go          # JWT authentication
    │   ├── api_key.go       # API key auth
    │   ├── cors.go          # CORS policies
    │   ├── metrics.go       # Metrics collection
    │   └── rate_limiter.go  # Rate limiting
    │
    ├── services/             # Infrastructure services
    │   ├── cache/           # Redis cache
    │   ├── email/           # Email service
    │   ├── notification/    # Push notifications
    │   ├── sse/             # Server-Sent Events
    │   └── storage/         # File storage
    │
    └── tools/                # Utilities
        ├── bind. go          # Data binding helpers
        └── url.go           # URL utilities
```

### Layer Principles

- **`cmd/`** Minimal entry point, bootstrap only
- **`internal/`** Your application-specific code
- **`pkg/medusa/`** Pure framework, no app dependencies
- **Golden Rule**:  `internal/` imports `pkg/medusa/`, never the reverse

### Modularity

Every component in `pkg/medusa` is independently usable:

```go
// Use only what you need
import "github.com/imlargo/medusa/pkg/medusa/services/cache"
import "github.com/imlargo/medusa/pkg/medusa/core/logger"

// No need to import the entire framework
```

---

## Examples

### Complete REST API with Auth

```go
func main() {
    log := logger.NewLogger()
    router := gin.Default()
    
    // Database
    db, _ := database.NewPostgresDatabase(os.Getenv("DATABASE_URL"))
    
    // JWT Auth
    jwtAuth, err := jwt.NewJWT(jwt.Config{Secret: os.Getenv("JWT_SECRET")})
    
    // Public routes
    router.POST("/auth/register", RegisterHandler)
    router.POST("/auth/login", LoginHandler)
    
    // Protected routes
    protected := router.Group("/api")
    protected.Use(middleware.AuthTokenMiddleware(jwtAuth))
    {
        protected.GET("/profile", GetProfileHandler)
        protected.PUT("/profile", UpdateProfileHandler)
    }
    
    // Rate limited routes
    limited := router.Group("/api/actions")
    limited.Use(middleware.NewRateLimiterMiddleware(rateLimiter))
    {
        limited.POST("/upload", UploadHandler)
    }
    
    srv := http.NewServer(router, log)
    app := app.NewApp(app.WithServer(srv))
    app.Run(context.Background())
}
```

### Multi-Server Application

```go
// Run HTTP and SSE servers concurrently
func main() {
    log := logger.NewLogger()
    
    // HTTP Server
    httpRouter := gin.Default()
    httpServer := http.NewServer(httpRouter, log,
        http.WithServerPort("8080"),
    )
    
    // SSE Server
    sseRouter := gin.Default()
    sseServer := http.NewServer(sseRouter, log,
        http.WithServerPort("8081"),
    )
    
    // Application with multiple servers
    app := app. NewApp(
        app.WithName("multi-server"),
        app.WithServer(httpServer, sseServer),
    )
    
    // Both servers managed with single graceful shutdown
    app.Run(context.Background())
}
```

### Repository Pattern

```go
// Define repository interface
type UserRepository interface {
    Create(ctx context.Context, user *User) error
    FindByID(ctx context.Context, id uint) (*User, error)
    Update(ctx context.Context, user *User) error
}

// Implementation
type userRepository struct {
    *medusarepo.Repository
}

func NewUserRepository(repo *medusarepo.Repository) UserRepository {
    return &userRepository{Repository: repo}
}

func (r *userRepository) Create(ctx context.Context, user *User) error {
    return r. DB.WithContext(ctx).Create(user).Error
}

func (r *userRepository) FindByID(ctx context.Context, id uint) (*User, error) {
    var user User
    err := r. DB.WithContext(ctx).First(&user, id).Error
    return &user, err
}
```

---

## Configuration

Medusa uses environment variables for configuration.  Create a `.env` file:

`internal/config` reads every variable once at startup, applies the defaults
below and reports **all** problems in a single error, so a bad deployment fails
immediately with the complete list.

```bash
# Environment: development | staging | production
# production enables Gin release mode and requires a JWT_SECRET of 32+ chars.
APP_ENV=development

# Server
HOST=localhost
PORT=8000

# Database (required)
DATABASE_URL=postgres://user:password@localhost:5432/dbname?sslmode=disable

# Redis - optional, leave empty to run without cache
REDIS_URL=redis://localhost:6379

# JWT (JWT_SECRET is required). Expirations are Go durations: 15m, 24h, 168h...
JWT_SECRET=your-secret-key-change-this
JWT_ISSUER=medusa
JWT_TOKEN_EXPIRATION=15m
JWT_REFRESH_EXPIRATION=168h

# CORS - comma separated. The server host is always allowed.
CORS_ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com

# Rate limiting (applies to the whole /v1 group, public endpoints included)
RATE_LIMITER_ENABLED=true
RATE_LIMITER_REQUESTS_PER_TIME_FRAME=100
RATE_LIMITER_TIME_FRAME=1m

# Storage (S3/R2) - optional. Set the bucket plus all three credentials, or none.
STORAGE_PROVIDER=r2
STORAGE_BUCKET_NAME=your_bucket
STORAGE_ACCOUNT_ID=your_account_id
STORAGE_ACCESS_KEY_ID=your_access_key
STORAGE_SECRET_ACCESS_KEY=your_secret_key
STORAGE_PUBLIC_DOMAIN=cdn.example.com
STORAGE_USE_PUBLIC_URL=true
```

### Defaults

| Variable | Default | Notes |
|---|---|---|
| `APP_ENV` | `development` | |
| `HOST` / `PORT` | `localhost` / `8000` | |
| `DATABASE_URL` | — | Required |
| `JWT_SECRET` | — | Required; 32+ chars in production |
| `JWT_ISSUER` | `medusa` | |
| `JWT_TOKEN_EXPIRATION` | `15m` | Must be shorter than the refresh expiration |
| `JWT_REFRESH_EXPIRATION` | `168h` | |
| `REDIS_URL` | empty | Empty disables the cache |
| `CORS_ALLOWED_ORIGINS` | empty | |
| `RATE_LIMITER_ENABLED` | `true` | |
| `RATE_LIMITER_REQUESTS_PER_TIME_FRAME` | `100` | |
| `RATE_LIMITER_TIME_FRAME` | `1m` | |
| `STORAGE_PROVIDER` | `r2` | |
| `STORAGE_BUCKET_NAME` | empty | Empty disables storage |

---

## Middleware

### Available Middleware

```go
import "github.com/imlargo/medusa/pkg/medusa/middleware"

// JWT Authentication
router.Use(middleware.AuthTokenMiddleware(jwtAuth))

// API Key Authentication (Header)
router.Use(middleware.ApiKeyMiddleware("your-api-key"))

// API Key Authentication (Bearer)
router.Use(middleware.BearerApiKeyMiddleware("your-api-key"))

// CORS
router.Use(middleware.NewCorsMiddleware("https://myapp.com", []string{
    "https://app.myapp.com",
}))

// Rate Limiting
router.Use(middleware.NewRateLimiterMiddleware(rateLimiter))

// Metrics Collection
router.Use(middleware.NewMetricsMiddleware(metricsService))
```

### Creating Custom Middleware

```go
func LogRequestMiddleware(log *logger.Logger) gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        
        c.Next()
        
        duration := time.Since(start)
        log.Info("Request completed",
            zap.String("method", c.Request.Method),
            zap.String("path", c.Request.URL. Path),
            zap.Int("status", c.Writer.Status()),
            zap.Duration("duration", duration),
        )
    }
}
```

---

## Development

### Hot Reload

Medusa includes Air for hot reload during development:

```bash
# Install Air (if not already installed)
go install github.com/cosmtrek/air@latest

# Run with hot reload
air
```

Configuration is in `.air.toml`.

### Available Commands

```bash
# Run the application
make run

# Run with hot reload
make dev

# Format code
make format

# Run tests
make test

# Build binary
make build

# Generate Swagger docs
make swag

# Clean build artifacts
make clean
```

---

## Testing

```go
package handlers_test

import (
    "testing"
    "net/http/httptest"
    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
)

func TestPingHandler(t *testing.T) {
    gin.SetMode(gin.TestMode)
    
    router := gin.Default()
    router.GET("/ping", PingHandler)
    
    req := httptest.NewRequest("GET", "/ping", nil)
    w := httptest.NewRecorder()
    
    router.ServeHTTP(w, req)
    
    assert.Equal(t, 200, w.Code)
    assert.Contains(t, w.Body.String(), "pong")
}
```

---

## Roadmap

### ✅ v0.1 - Foundation (Current)

- [x] Core framework (app, server, logger)
- [x] JWT & API key authentication
- [x] Repository pattern with GORM
- [x] Redis cache
- [x] File storage (S3/R2)
- [x] Server-Sent Events
- [x] Email & push notifications
- [x] Rate limiting & CORS
- [x] Prometheus metrics

### 🚧 v0.2 - Developer Experience (In Progress)

- [x] Health checks & readiness probes
- [x] Request ID middleware
- [x] Enhanced error handling with request tracking
- [x] Bootstrap architecture with DI container
- [x] Typed context with validation helpers
- [ ] Comprehensive documentation
- [ ] Example applications
- [ ] Testing utilities
- [ ] Docker Compose setup
- [ ] CI/CD examples

### 🎯 v0.3 - Validation & Documentation

- [x] Automatic request validation
- [x] Declarative validation tags
- [x] OpenAPI 3.0 generation
- [x] Swagger UI at `/docs`
- [ ] ReDoc integration
- [ ] Auto-generated examples

### 🎯 v0.4 - Type Safety & Ergonomics

- [x] Enhanced `medusa.Context` with helpers
- [x] Type-safe handlers with generics
- [x] Dependency injection system
- [x] Automatic pagination
- [x] Query filter builders
- [ ] File upload helpers

### 🎯 v0.5 - CLI & Generators

- [ ] `medusa` CLI tool
- [ ] Project scaffolding
- [ ] Code generators (handlers, models, repos)
- [ ] Migration management
- [ ] Custom templates

### 🎯 v0.6 - Testing Framework

- [ ] Built-in testing utilities
- [ ] Test database helpers
- [ ] HTTP testing tools
- [ ] Mock generators
- [ ] Fixture factories

### 🎯 v0.7 - Background Processing

- [ ] Job queue system
- [ ] Scheduled tasks (cron)
- [ ] Worker pools
- [ ] Retry policies
- [ ] Job monitoring

### 🎯 v0.8 - Advanced Observability

- [ ] OpenTelemetry integration
- [ ] Distributed tracing
- [ ] APM metrics
- [ ] Error tracking
- [ ] Performance profiling

### 🎯 v0.9 - WebSockets

- [ ] Native WebSocket support
- [ ] Rooms & namespaces
- [ ] Broadcasting
- [ ] Presence detection

### 🎯 v1.0 - Production Ready

- [ ] Security audit
- [ ] Performance benchmarks
- [ ] Complete documentation
- [ ] Video tutorials
- [ ] Production case studies

---

## Why Medusa?

### vs. Minimalist Frameworks (Gin, Echo, Fiber)

**Gin/Echo/Fiber** are excellent routers, but you still need to: 
- Set up database connections
- Configure cache
- Implement auth
- Add file storage
- Set up observability
- Wire everything together

**Medusa** gives you all of this out-of-the-box, with clean interfaces and best practices baked in.

### vs. Full-Stack Frameworks (Buffalo, Beego)

**Buffalo/Beego** are comprehensive but: 
- Include frontend tooling you might not need
- More opinionated and harder to customize
- Heavier and less modular

**Medusa** focuses on backends only, stays modular, and never forces decisions. 

### vs. Starting from Scratch

**Building your own** means:
- Weeks of setup before writing features
- Reinventing patterns (repository, cache, storage)
- Maintenance burden for infrastructure code
- No community or shared knowledge

**Medusa** lets you ship on day one with production-ready patterns.

---

## Contributing

Contributions are welcome! Whether it's: 

- 🐛 Bug reports
- 💡 Feature requests  
- 📖 Documentation improvements
- 🔧 Code contributions

Please open an issue first to discuss what you'd like to change.

### Development Setup

```bash
# Fork and clone
git clone https://github.com/yourusername/medusa.git
cd medusa

# Install dependencies
go mod download

# Create a branch
git checkout -b feature/amazing-feature

# Make your changes and test
make test

# Commit and push
git commit -m "Add amazing feature"
git push origin feature/amazing-feature
```

Then open a Pull Request! 

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## Community

- **GitHub Issues** Bug reports and feature requests
- **GitHub Discussions** Questions and community chat
- **Twitter** [@yourusername](https://twitter.com/yourusername) for updates

---

## Acknowledgments

Medusa stands on the shoulders of giants:

- [Gin](https://github.com/gin-gonic/gin) HTTP framework
- [GORM](https://gorm.io) ORM
- [Zap](https://github.com/uber-go/zap) Logging
- [Go Redis](https://github.com/redis/go-redis) Redis client
- [AWS SDK](https://github.com/aws/aws-sdk-go-v2) Cloud storage

---

<div align="center">
  <p>Built with ❤️ by <a href="https://github.com/imlargo">imlargo</a></p>
  <p>
    <a href="#quick-start">Get Started</a> •
    <a href="https://github.com/imlargo/medusa/issues">Report Bug</a> •
    <a href="https://github.com/imlargo/medusa/issues">Request Feature</a>
  </p>
</div>
