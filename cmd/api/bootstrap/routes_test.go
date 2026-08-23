package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/internal/config"
	"github.com/imlargo/medusa/pkg/medusa/core/handler"
	"github.com/imlargo/medusa/pkg/medusa/core/jwt"
	"github.com/imlargo/medusa/pkg/medusa/core/logger"
	"github.com/imlargo/medusa/pkg/medusa/core/metrics"
	"github.com/imlargo/medusa/pkg/medusa/core/ratelimiter"
	"github.com/imlargo/medusa/pkg/medusa/services/health"
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
		Logger:        log,
		JWT:           jwt.NewJwt(jwt.Config{Secret: strings.Repeat("s", 32)}),
		HealthService: health.NewService(time.Second),
	}
	c.Config.Server.Host = "localhost"
	c.Config.Server.Port = 8000

	c.Handlers = &Handlers{
		Health: handler.NewHealthHandler(handler.NewHandler(log), c.HealthService),
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
			c.Metrics = metrics.NewPrometheusMetrics()
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
