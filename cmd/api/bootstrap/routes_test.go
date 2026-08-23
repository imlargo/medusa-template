package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/internal/config"
	"github.com/imlargo/medusa/internal/handlers"
	"github.com/imlargo/medusa/pkg/medusa/core/handler"
	"github.com/imlargo/medusa/pkg/medusa/core/jwt"
	"github.com/imlargo/medusa/pkg/medusa/core/logger"
	"github.com/imlargo/medusa/pkg/medusa/core/metrics"
	"github.com/imlargo/medusa/pkg/medusa/core/ratelimiter"
	"github.com/imlargo/medusa/pkg/medusa/services/health"
	"github.com/imlargo/sse"
	"github.com/prometheus/client_golang/prometheus"
)

// newTestContainer builds a container with only the pieces the router touches,
// so route wiring can be tested without any external dependency.
func newTestContainer(t *testing.T, mutate func(*Container)) *Container {
	t.Helper()

	log := logger.NewLogger()
	c := &Container{
		Config: &config.Config{
			Environment: config.EnvDevelopment,
		},
		Logger:          log,
		JWT:             jwt.MustNewJWT(jwt.Config{Secret: strings.Repeat("s", 32)}),
		HealthService:   health.NewService(time.Second),
		Events:          sse.NewBroker("events", sse.NewMemoryLog(sse.Retention{For: time.Minute})),
		EventsLifecycle: sse.NewLifecycle(),
	}
	c.Config.Server.Host = "localhost"
	c.Config.Server.Port = 8000

	c.Handlers = &Handlers{
		Health: handler.NewHealthHandler(handler.NewHandler(log), c.HealthService),
		Events: handlers.NewEventsHandler(handler.NewHandler(log), c.Events, c.EventsLifecycle, c.JWT),
	}

	if mutate != nil {
		mutate(c)
	}

	return c
}

func get(t *testing.T, router *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestProbesAreUnauthenticated(t *testing.T) {
	router := newRouter(newTestContainer(t, nil))

	for _, path := range []string{"/health", "/ready"} {
		if got := get(t, router, path).Code; got != http.StatusOK {
			t.Errorf("GET %s = %d, want %d", path, got, http.StatusOK)
		}
	}
}

func TestRequestIDHeaderIsAlwaysSet(t *testing.T) {
	router := newRouter(newTestContainer(t, nil))

	if got := get(t, router, "/health").Header().Get("X-Request-ID"); got == "" {
		t.Error("X-Request-ID header is empty, want a generated request ID")
	}
}

func TestMetricsEndpointFollowsTheMetricsService(t *testing.T) {
	t.Run("exposed when enabled", func(t *testing.T) {
		router := newRouter(newTestContainer(t, func(c *Container) {
			registry := prometheus.NewRegistry()
			metricsService, err := metrics.NewPrometheusMetrics(registry)
			if err != nil {
				t.Fatalf("NewPrometheusMetrics() = %v, want nil error", err)
			}
			c.Metrics = metricsService
			c.MetricsRegistry = registry
		}))

		if got := get(t, router, "/metrics").Code; got != http.StatusOK {
			t.Errorf("GET /metrics = %d, want %d", got, http.StatusOK)
		}
	})

	t.Run("absent when disabled", func(t *testing.T) {
		router := newRouter(newTestContainer(t, nil))

		if got := get(t, router, "/metrics").Code; got != http.StatusNotFound {
			t.Errorf("GET /metrics = %d, want %d", got, http.StatusNotFound)
		}
	})
}

func TestProtectedRoutesRequireAToken(t *testing.T) {
	router := newRouter(newTestContainer(t, nil))

	if got := get(t, router, "/v1/auth/user").Code; got != http.StatusUnauthorized {
		t.Errorf("GET /v1/auth/user = %d, want %d", got, http.StatusUnauthorized)
	}
}

// The rate limiter guards the whole /v1 group, so it must reject a client before
// authentication even runs: unauthenticated endpoints are the ones most in need
// of throttling.
func TestRateLimiterRunsBeforeAuthentication(t *testing.T) {
	router := newRouter(newTestContainer(t, func(c *Container) {
		c.RateLimiter = ratelimiter.NewTokenBucketLimiter(ratelimiter.Config{
			RequestsPerTimeFrame: 1,
			TimeFrame:            time.Minute,
		})
	}))

	if got := get(t, router, "/v1/auth/user").Code; got != http.StatusUnauthorized {
		t.Fatalf("first request = %d, want %d", got, http.StatusUnauthorized)
	}
	if got := get(t, router, "/v1/auth/user").Code; got != http.StatusTooManyRequests {
		t.Errorf("second request = %d, want %d", got, http.StatusTooManyRequests)
	}
}

func TestGinModeFollowsEnvironment(t *testing.T) {
	tests := map[config.Environment]string{
		config.EnvDevelopment: gin.DebugMode,
		config.EnvStaging:     gin.DebugMode,
		config.EnvProduction:  gin.ReleaseMode,
	}

	for environment, want := range tests {
		c := newTestContainer(t, func(c *Container) { c.Config.Environment = environment })
		if got := ginMode(c); got != want {
			t.Errorf("ginMode(%q) = %q, want %q", environment, got, want)
		}
	}
}

// A metrics service can be built more than once in a process now that the
// registry is injected. With the global default registry this panicked.
func TestMetricsCanBeBuiltTwice(t *testing.T) {
	for i := range 2 {
		if _, err := metrics.NewPrometheusMetrics(prometheus.NewRegistry()); err != nil {
			t.Fatalf("build %d = %v, want nil error", i, err)
		}
	}
}

func TestEventStreamRequiresAToken(t *testing.T) {
	router := newRouter(newTestContainer(t, nil))

	if got := get(t, router, "/v1/events").Code; got != http.StatusUnauthorized {
		t.Errorf("GET /v1/events = %d, want %d", got, http.StatusUnauthorized)
	}
}

// Shutdown drains SSE streams before the HTTP server stops. With nothing open it
// has to return immediately: an unnecessary wait here is added to every deploy.
func TestDrainEventStreamsIsImmediateWhenIdle(t *testing.T) {
	c := newTestContainer(t, nil)

	start := time.Now()
	if err := c.DrainEventStreams(context.Background()); err != nil {
		t.Fatalf("DrainEventStreams() = %v, want nil", err)
	}

	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("drain took %s with no open streams, want it to return immediately", elapsed)
	}
}

func TestDrainEventStreamsWithoutALifecycle(t *testing.T) {
	c := newTestContainer(t, func(c *Container) { c.EventsLifecycle = nil })

	if err := c.DrainEventStreams(context.Background()); err != nil {
		t.Errorf("DrainEventStreams() = %v, want nil when no lifecycle is configured", err)
	}
}
