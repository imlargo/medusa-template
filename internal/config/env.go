package config

import "time"

// Names of the environment variables read by Load.
const (
	envAppEnv = "APP_ENV"

	envHost = "HOST"
	envPort = "PORT"

	envDatabaseURL = "DATABASE_URL"
	envRedisURL    = "REDIS_URL"

	envJWTSecret            = "JWT_SECRET"
	envJWTIssuer            = "JWT_ISSUER"
	envJWTTokenExpiration   = "JWT_TOKEN_EXPIRATION"
	envJWTRefreshExpiration = "JWT_REFRESH_EXPIRATION"

	envCORSAllowedOrigins = "CORS_ALLOWED_ORIGINS"

	envRateLimiterEnabled   = "RATE_LIMITER_ENABLED"
	envRateLimiterRequests  = "RATE_LIMITER_REQUESTS_PER_TIME_FRAME"
	envRateLimiterTimeFrame = "RATE_LIMITER_TIME_FRAME"

	envStorageProvider        = "STORAGE_PROVIDER"
	envStorageBucketName      = "STORAGE_BUCKET_NAME"
	envStorageAccountID       = "STORAGE_ACCOUNT_ID"
	envStorageAccessKeyID     = "STORAGE_ACCESS_KEY_ID"
	envStorageSecretAccessKey = "STORAGE_SECRET_ACCESS_KEY"
	envStoragePublicDomain    = "STORAGE_PUBLIC_DOMAIN"
	envStorageUsePublicURL    = "STORAGE_USE_PUBLIC_URL"
)

// Values applied when the corresponding variable is unset or empty.
const (
	defaultHost = "localhost"
	defaultPort = 8000

	defaultJWTIssuer         = "medusa"
	defaultTokenExpiration   = 15 * time.Minute
	defaultRefreshExpiration = 7 * 24 * time.Hour

	defaultRateLimiterEnabled   = true
	defaultRateLimiterRequests  = 100
	defaultRateLimiterTimeFrame = time.Minute
)

// minJWTSecretLength is the shortest secret accepted in production. HS256 keys
// shorter than the 256-bit digest weaken the signature, so anything under 32
// bytes is rejected instead of silently deployed.
const minJWTSecretLength = 32
