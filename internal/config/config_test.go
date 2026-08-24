package config

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/imlargo/medusa/pkg/medusa/services/storage"
)

// validEnv is the smallest environment that loads successfully.
func validEnv(t *testing.T) {
	t.Helper()
	t.Setenv(envDatabaseURL, "postgres://user:pass@localhost:5432/medusa")
	t.Setenv(envJWTSecret, strings.Repeat("s", minJWTSecretLength))
}

func TestLoadAppliesDefaults(t *testing.T) {
	validEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() = %v, want nil error", err)
	}

	if cfg.Environment != EnvDevelopment {
		t.Errorf("Environment = %q, want %q", cfg.Environment, EnvDevelopment)
	}
	if cfg.Server.Host != defaultHost || cfg.Server.Port != defaultPort {
		t.Errorf("Server = %s:%d, want %s:%d", cfg.Server.Host, cfg.Server.Port, defaultHost, defaultPort)
	}
	if cfg.Auth.TokenExpiration != defaultTokenExpiration {
		t.Errorf("TokenExpiration = %s, want %s", cfg.Auth.TokenExpiration, defaultTokenExpiration)
	}
	if cfg.Redis.Enabled() {
		t.Error("Redis.Enabled() = true, want false when REDIS_URL is unset")
	}
	if cfg.Storage.Enabled() {
		t.Error("Storage.Enabled() = true, want false when no bucket is configured")
	}
	if !cfg.RateLimiter.Enabled {
		t.Error("RateLimiter.Enabled = false, want the default true")
	}
}

func TestLoadParsesDurationsAndLists(t *testing.T) {
	validEnv(t)
	t.Setenv(envJWTTokenExpiration, "30m")
	t.Setenv(envJWTRefreshExpiration, "168h")
	t.Setenv(envRateLimiterTimeFrame, "1h30m")
	t.Setenv(envCORSAllowedOrigins, " https://a.dev , ,https://b.dev ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() = %v, want nil error", err)
	}

	if got, want := cfg.Auth.TokenExpiration, 30*time.Minute; got != want {
		t.Errorf("TokenExpiration = %s, want %s", got, want)
	}
	if got, want := cfg.Auth.RefreshExpiration, 168*time.Hour; got != want {
		t.Errorf("RefreshExpiration = %s, want %s", got, want)
	}
	if got, want := cfg.RateLimiter.TimeFrame, 90*time.Minute; got != want {
		t.Errorf("RateLimiter.TimeFrame = %s, want %s", got, want)
	}

	want := []string{"https://a.dev", "https://b.dev"}
	if got := cfg.CORS.AllowedOrigins; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("CORS.AllowedOrigins = %q, want %q", got, want)
	}
}

func TestLoadReadsStorage(t *testing.T) {
	validEnv(t)
	t.Setenv(envStorageBucketName, "assets")
	t.Setenv(envStorageAccountID, "account")
	t.Setenv(envStorageAccessKeyID, "key")
	t.Setenv(envStorageSecretAccessKey, "secret")
	t.Setenv(envStorageUsePublicURL, "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() = %v, want nil error", err)
	}

	if !cfg.Storage.Enabled() {
		t.Fatal("Storage.Enabled() = false, want true")
	}
	if cfg.Storage.Provider != storage.ProviderR2 {
		t.Errorf("Storage.Provider = %q, want %q", cfg.Storage.Provider, storage.ProviderR2)
	}
	if cfg.Storage.BucketName != "assets" || !cfg.Storage.UsePublicURL {
		t.Errorf("Storage = %+v, want bucket %q with public URLs", cfg.Storage.Config, "assets")
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{
			name:    "missing required variables",
			env:     map[string]string{},
			wantErr: envDatabaseURL + " is required",
		},
		{
			name:    "malformed port",
			env:     map[string]string{envPort: "eight thousand"},
			wantErr: "not a valid integer",
		},
		{
			name:    "port out of range",
			env:     map[string]string{envPort: "70000"},
			wantErr: "out of the valid port range",
		},
		{
			name:    "malformed duration",
			env:     map[string]string{envJWTTokenExpiration: "15"},
			wantErr: "not a valid duration",
		},
		{
			name:    "refresh shorter than access token",
			env:     map[string]string{envJWTTokenExpiration: "24h", envJWTRefreshExpiration: "1h"},
			wantErr: "must be longer than",
		},
		{
			name:    "unknown environment",
			env:     map[string]string{envAppEnv: "prod"},
			wantErr: "unknown environment",
		},
		{
			name:    "weak secret in production",
			env:     map[string]string{envAppEnv: "production", envJWTSecret: "short"},
			wantErr: envJWTSecret + ": must be at least",
		},
		{
			name:    "rate limiter without requests",
			env:     map[string]string{envRateLimiterRequests: "0"},
			wantErr: envRateLimiterRequests + ": must be positive",
		},
		{
			name:    "storage credentials without bucket",
			env:     map[string]string{envStorageAccountID: "account"},
			wantErr: envStorageBucketName + " is required",
		},
		{
			name:    "storage bucket without credentials",
			env:     map[string]string{envStorageBucketName: "assets"},
			wantErr: "storage is configured but",
		},
		{
			name:    "unsupported storage provider",
			env:     map[string]string{envStorageProvider: "gcs", envStorageBucketName: "assets"},
			wantErr: "unsupported provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name != "missing required variables" {
				validEnv(t)
			}
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			_, err := Load()
			if err == nil {
				t.Fatal("Load() = nil error, want a validation failure")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Load() error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadReportsEveryProblemAtOnce(t *testing.T) {
	validEnv(t)
	t.Setenv(envPort, "not-a-port")
	t.Setenv(envRateLimiterEnabled, "maybe")
	t.Setenv(envAppEnv, "prod")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() = nil error, want a validation failure")
	}

	for _, want := range []string{envPort, envRateLimiterEnabled, envAppEnv} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Load() error = %q, want it to mention %s", err, want)
		}
	}
}

func TestEnvironmentPredicates(t *testing.T) {
	tests := []struct {
		env                            Environment
		valid, production, development bool
	}{
		{EnvDevelopment, true, false, true},
		{EnvStaging, true, false, false},
		{EnvProduction, true, true, false},
		{Environment("prod"), false, false, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.env), func(t *testing.T) {
			if got := tt.env.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
			if got := tt.env.IsProduction(); got != tt.production {
				t.Errorf("IsProduction() = %v, want %v", got, tt.production)
			}
			if got := tt.env.IsDevelopment(); got != tt.development {
				t.Errorf("IsDevelopment() = %v, want %v", got, tt.development)
			}
		})
	}
}

// Docs default to off in production: publishing the whole API surface should be
// a decision, not something an environment variable forgets to turn off.
func TestDocsDefaultOffInProductionOnly(t *testing.T) {
	tests := map[string]bool{
		"development": true,
		"staging":     true,
		"production":  false,
	}

	for environment, want := range tests {
		t.Run(environment, func(t *testing.T) {
			validEnv(t)
			t.Setenv(envAppEnv, environment)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() = %v, want nil error", err)
			}
			if got := cfg.Runtime.DocsEnabled; got != want {
				t.Errorf("DocsEnabled = %v, want %v in %s", got, want, environment)
			}
		})
	}
}

func TestDocsCanBeForcedOnInProduction(t *testing.T) {
	validEnv(t)
	t.Setenv(envAppEnv, "production")
	t.Setenv(envDocsEnabled, "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() = %v, want nil error", err)
	}
	if !cfg.Runtime.DocsEnabled {
		t.Error("DocsEnabled = false, want an explicit DOCS_ENABLED=true to win")
	}
}

// The two shutdown phases are sequential, so their budgets have to add up to the
// total rather than each being it. Exceeding the total means the orchestrator
// SIGKILLs the process mid-drain.
func TestShutdownBudgetIsSplitNotDuplicated(t *testing.T) {
	validEnv(t)
	t.Setenv(envShutdownTimeout, "30s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() = %v, want nil error", err)
	}

	runtime := cfg.Runtime
	drain := runtime.DrainTimeout()
	server := runtime.ServerShutdownTimeout()

	if drain <= 0 {
		t.Errorf("DrainTimeout() = %s, want a positive slice", drain)
	}
	if server <= 0 {
		t.Errorf("ServerShutdownTimeout() = %s, want a positive slice", server)
	}
	if total := drain + server; total != runtime.ShutdownTimeout {
		t.Errorf("drain (%s) + server (%s) = %s, want exactly %s", drain, server, total, runtime.ShutdownTimeout)
	}
}

func TestRuntimeDefaults(t *testing.T) {
	validEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() = %v, want nil error", err)
	}

	if cfg.Runtime.ShutdownTimeout != defaultShutdownTimeout {
		t.Errorf("ShutdownTimeout = %s, want %s", cfg.Runtime.ShutdownTimeout, defaultShutdownTimeout)
	}
	if cfg.Runtime.MaxRequestBody != defaultMaxRequestBody {
		t.Errorf("MaxRequestBody = %d, want %d", cfg.Runtime.MaxRequestBody, defaultMaxRequestBody)
	}
}

func TestRuntimeValidation(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{"non-positive shutdown", map[string]string{envShutdownTimeout: "0s"}, envShutdownTimeout + ": must be positive"},
		{"non-positive body cap", map[string]string{envMaxRequestBody: "0"}, envMaxRequestBody + ": must be positive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validEnv(t)
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			_, err := Load()
			if err == nil {
				t.Fatal("Load() = nil error, want a validation failure")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Load() error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

// TestRateLimiterTrustedProxiesAreValidated. A malformed CIDR would be dropped
// silently and the proxy it described would stop being trusted, which changes
// which address gets counted. That has to fail at startup.
func TestRateLimiterTrustedProxiesAreValidated(t *testing.T) {
	cases := []struct {
		value   string
		wantErr bool
	}{
		{"", false},
		{"10.0.0.0/8", false},
		{"10.0.0.0/8,172.16.0.0/12, 192.168.0.0/16", false},
		{"::1/128", false},
		{"10.0.0.1", true},    // an address, not a block
		{"10.0.0.0/33", true}, // out of range
		{"not-a-cidr", true},  // nonsense
		{"10.0.0.0/8,garbage", true},
	}

	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			validEnv(t)
			t.Setenv(envRateLimiterEnabled, "true")
			t.Setenv(envRateLimiterTrustedProxies, tc.value)

			cfg, err := Load()
			switch {
			case tc.wantErr && err == nil:
				t.Fatalf("%q was accepted as a trusted proxy list", tc.value)
			case !tc.wantErr && err != nil:
				t.Fatalf("%q was rejected: %v", tc.value, err)
			case err != nil:
				if !strings.Contains(err.Error(), "RATE_LIMITER_TRUSTED_PROXIES") {
					t.Errorf("the error does not name the offending variable: %v", err)
				}
				return
			}
			for _, cidr := range cfg.RateLimiter.TrustedProxies {
				if _, perr := netip.ParsePrefix(cidr); perr != nil {
					t.Errorf("Load returned an unparseable prefix %q", cidr)
				}
			}
		})
	}
}

// TestRateLimiterQuotasAreValidated for the same reason: a quota of zero is a
// limiter that refuses everything, and finding that out from production traffic
// is the expensive way.
func TestRateLimiterQuotasAreValidated(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		wantErr bool
	}{
		{"defaults", map[string]string{envRateLimiterEnabled: "true"}, false},
		{"zero requests", map[string]string{envRateLimiterEnabled: "true", envRateLimiterRequests: "0"}, true},
		{"negative requests", map[string]string{envRateLimiterEnabled: "true", envRateLimiterRequests: "-5"}, true},
		{"zero auth requests", map[string]string{envRateLimiterEnabled: "true", envRateLimiterAuthRequests: "0"}, true},
		{"disabled, nothing checked", map[string]string{envRateLimiterEnabled: "false", envRateLimiterRequests: "0"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			_, err := Load()
			if tc.wantErr != (err != nil) {
				t.Fatalf("wantErr=%v, got %v", tc.wantErr, err)
			}
		})
	}
}
