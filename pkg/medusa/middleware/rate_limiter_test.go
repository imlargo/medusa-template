package middleware_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa/middleware"
	"github.com/imlargo/ratelimit"
)

func init() { gin.SetMode(gin.TestMode) }

// serveN drives n requests through the middleware and reports how many were
// served. remote is the connecting address; xff, when it returns a value, is
// sent as X-Forwarded-For.
func serveN(t *testing.T, key ratelimit.Key, quota ratelimit.Quota, n int, remote string, xff func(i int) string) int {
	t.Helper()

	lim, err := ratelimit.NewWith(ratelimit.Config{
		Rules: []ratelimit.Rule{{Name: "test", Quota: quota, Key: key}},
	})
	if err != nil {
		t.Fatalf("build limiter: %v", err)
	}
	t.Cleanup(func() { _ = lim.Close() })

	r := gin.New()
	// Mirrors the real router: gin is told which proxies to trust, and the
	// limiter is mounted on the versioned group.
	if err := r.SetTrustedProxies(nil); err != nil {
		t.Fatal(err)
	}
	v1 := r.Group("/v1")
	v1.Use(middleware.NewRateLimiterMiddleware(lim))
	v1.POST("/auth/login", func(c *gin.Context) { c.Status(http.StatusOK) })

	served := 0
	for i := 0; i < n; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
		req.RemoteAddr = remote
		if v := xff(i); v != "" {
			req.Header.Set("X-Forwarded-For", v)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code == http.StatusOK {
			served++
		}
	}
	return served
}

func noHeader(int) string { return "" }

// TestForwardedHeaderCannotBuyQuota is the regression test for a bypass that
// made the rate limiter useless.
//
// The middleware used to resolve the client address itself, with gin.ClientIP,
// and hand the result to the limiter as an identity. Gin trusts every proxy
// unless told otherwise and returns the *leftmost* X-Forwarded-For entry, which
// is whatever the caller wrote. A single caller rotating that header got a fresh
// counter on every request: 3000 of 3000 requests served against a quota of 100.
//
// The middleware now hands the request to the limiter whole and lets it derive
// the address from declared trusted proxies, so a forwarding header buys
// nothing.
func TestForwardedHeaderCannotBuyQuota(t *testing.T) {
	const (
		quota    = 100
		attempts = 3000
	)
	rotating := func(i int) string { return fmt.Sprintf("198.51.100.%d", i%254+1) }

	cases := []struct {
		name   string
		key    ratelimit.Key
		remote string
		xff    func(int) string
	}{
		{
			name:   "no proxies declared, forged header",
			key:    ratelimit.ByPeer(),
			remote: "203.0.113.9:1234",
			xff:    rotating,
		},
		{
			name:   "no proxies declared, no header",
			key:    ratelimit.ByPeer(),
			remote: "203.0.113.9:1234",
			xff:    noHeader,
		},
		{
			// A proxy appends the address that connected to it, on the right.
			// Everything to the left is the caller's and is ignored.
			name:   "trusted proxy appends the real address",
			key:    ratelimit.ByIP("10.0.0.0/8"),
			remote: "10.0.0.1:1234",
			xff:    func(i int) string { return rotating(i) + ", 203.0.113.9" },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			served := serveN(t, tc.key, ratelimit.PerMinute(quota), attempts, tc.remote, tc.xff)
			if served != quota {
				t.Errorf("served %d of %d attempts against a quota of %d; a forwarding header must not buy quota",
					served, attempts, quota)
			}
		})
	}
}

// TestDeniedResponseTellsTheClientWhatToDo. A 429 with no headers leaves a
// client retrying blind, which is how a rate limit turns into a retry storm.
func TestDeniedResponseTellsTheClientWhatToDo(t *testing.T) {
	lim, err := ratelimit.NewWith(ratelimit.Config{
		Rules: []ratelimit.Rule{{Name: "tight", Quota: ratelimit.PerMinute(1), Key: ratelimit.ByPeer()}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lim.Close() }()

	r := gin.New()
	v1 := r.Group("/v1")
	v1.Use(middleware.NewRateLimiterMiddleware(lim))
	v1.POST("/auth/login", func(c *gin.Context) { c.Status(http.StatusOK) })

	call := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
		req.RemoteAddr = "203.0.113.9:1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	if w := call(); w.Code != http.StatusOK {
		t.Fatalf("first request: status %d, want 200", w.Code)
	}

	w := call()
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: status %d, want 429", w.Code)
	}
	for _, h := range []string{"RateLimit", "RateLimit-Policy", "Retry-After"} {
		if w.Header().Get(h) == "" {
			t.Errorf("a denied response is missing the %s header", h)
		}
	}
	if got := w.Header().Get("Retry-After"); got == "0" {
		t.Error("Retry-After is 0, which invites an immediate retry that is certain to be rejected")
	}
}

// TestAllowedResponsesCarryHeaders too, so a client can slow down before it is
// refused rather than after.
func TestAllowedResponsesCarryHeaders(t *testing.T) {
	lim, err := ratelimit.NewWith(ratelimit.Config{
		Rules: []ratelimit.Rule{{Name: "general", Quota: ratelimit.PerMinute(100), Key: ratelimit.ByPeer()}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lim.Close() }()

	r := gin.New()
	v1 := r.Group("/v1")
	v1.Use(middleware.NewRateLimiterMiddleware(lim))
	v1.GET("/things", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/v1/things", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", w.Code)
	}
	if got := w.Header().Get("RateLimit"); got == "" {
		t.Error("an allowed response carries no RateLimit header, so a client cannot see it approaching the limit")
	}
}
