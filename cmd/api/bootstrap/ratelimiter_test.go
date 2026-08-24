package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/imlargo/medusa/internal/config"
	"github.com/imlargo/ratelimit"
	"github.com/prometheus/client_golang/prometheus"
)

func rateLimiterConfig() config.RateLimiterConfig {
	return config.RateLimiterConfig{
		Enabled:               true,
		RequestsPerTimeFrame:  100,
		TimeFrame:             time.Minute,
		AuthRequestsPerMinute: 10,
	}
}

// TestRateLimiterRuleTable checks the two rules do what they are there for: the
// auth rule is much tighter, and the general rule still applies to everything
// else. Both cover a request that matches both, and the tighter one governs.
func TestRateLimiterRuleTable(t *testing.T) {
	lim, err := buildRateLimiter(rateLimiterConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lim.Close() }()

	if got := lim.Rules(); len(got) != 2 {
		t.Fatalf("rules = %v, want two", got)
	}

	drain := func(method, path, remote string) int {
		served := 0
		for i := 0; i < 200; i++ {
			r := httptest.NewRequest(method, path, nil)
			r.RemoteAddr = remote
			if !lim.CheckRequest(r).Allowed {
				return served
			}
			served++
		}
		return served
	}

	// Credential endpoints stop at the auth limit, not the general one.
	if got := drain(http.MethodPost, "/v1/auth/login", "203.0.113.1:1"); got != 10 {
		t.Errorf("POST /v1/auth/login served %d requests, want the auth limit of 10", got)
	}
	if got := drain(http.MethodPost, "/v1/auth/register", "203.0.113.2:1"); got != 10 {
		t.Errorf("POST /v1/auth/register served %d requests, want the auth limit of 10", got)
	}

	// Everything else gets the general limit.
	if got := drain(http.MethodGet, "/v1/events", "203.0.113.3:1"); got != 100 {
		t.Errorf("GET /v1/events served %d requests, want the general limit of 100", got)
	}
	// Reading a user is a GET, so the auth rule's method does not match it.
	if got := drain(http.MethodGet, "/v1/auth/user", "203.0.113.4:1"); got != 100 {
		t.Errorf("GET /v1/auth/user served %d requests, want the general limit of 100", got)
	}
}

// TestRateLimiterAuthDenialNamesTheAuthRule, so an operator reading a metric or
// a log can tell brute force apart from a caller merely being busy.
func TestRateLimiterAuthDenialNamesTheAuthRule(t *testing.T) {
	lim, err := buildRateLimiter(rateLimiterConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lim.Close() }()

	var d ratelimit.Decision
	for i := 0; i < 20; i++ {
		r := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
		r.RemoteAddr = "203.0.113.9:1"
		d = lim.CheckRequest(r)
	}
	if d.Allowed {
		t.Fatal("expected the auth limit to deny")
	}
	if d.Rule != "auth" {
		t.Errorf("denial attributed to rule %q, want \"auth\"", d.Rule)
	}
	if d.Reason != ratelimit.ReasonDeniedQuota {
		t.Errorf("reason %v, want ReasonDeniedQuota", d.Reason)
	}
}

// TestRateLimiterKeysByConnectionWithoutTrustedProxies. With no proxies
// declared, a forwarding header must change nothing.
func TestRateLimiterKeysByConnectionWithoutTrustedProxies(t *testing.T) {
	lim, err := buildRateLimiter(rateLimiterConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lim.Close() }()

	served := 0
	for i := 0; i < 200; i++ {
		r := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
		r.RemoteAddr = "203.0.113.9:1"
		r.Header.Set("X-Forwarded-For", "198.51.100.1")
		if lim.CheckRequest(r).Allowed {
			served++
		}
	}
	if served != 100 {
		t.Errorf("a caller rotating no header at all served %d of 200; want the general limit of 100", served)
	}
}

// TestRateLimiterRejectsProxiesItCannotParse at build time. A malformed range
// would silently stop trusting the proxy it described.
func TestRateLimiterRejectsProxiesItCannotParse(t *testing.T) {
	cfg := rateLimiterConfig()
	cfg.TrustedProxies = []string{"not-a-cidr"}
	if _, err := buildRateLimiter(cfg, nil); err == nil {
		t.Error("a malformed trusted proxy range was accepted")
	}
}

// TestRateLimiterMetricsAreRegistered. This is the one component whose job is to
// refuse requests, so whether it is refusing the wrong ones has to be
// answerable.
func TestRateLimiterMetricsAreRegistered(t *testing.T) {
	registry := prometheus.NewRegistry()
	lim, err := buildRateLimiter(rateLimiterConfig(), registry)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lim.Close() }()

	for i := 0; i < 20; i++ {
		r := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
		r.RemoteAddr = "203.0.113.9:1"
		lim.CheckRequest(r)
	}

	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"medusa_ratelimit_decisions_total": false,
		"medusa_ratelimit_denied_total":    false,
	}
	for _, f := range families {
		if _, ok := want[f.GetName()]; ok {
			want[f.GetName()] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("%s was never recorded", name)
		}
	}

	// The rule name is a label; the caller's address must never be.
	for _, f := range families {
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				if v := l.GetValue(); v == "203.0.113.9" {
					t.Errorf("%s carries the caller address as the label %q", f.GetName(), l.GetName())
				}
			}
		}
	}
}
