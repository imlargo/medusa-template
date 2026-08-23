package middleware

import (
	"math"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa/core/ratelimiter"
	"github.com/imlargo/medusa/pkg/medusa/core/responses"
)

// NewRateLimiterMiddleware creates a middleware that enforces rate limiting per client IP.
// It uses the provided rate limiter to check if a request should be allowed.
// If the rate limit is exceeded, it returns a 429 Too Many Requests response
// with information about when to retry.
func NewRateLimiterMiddleware(rl ratelimiter.RateLimiter) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		allow, retryAfter := rl.Allow(ctx.ClientIP())
		if !allow {
			// Round up: telling a client to retry in 0 seconds invites an
			// immediate retry that is guaranteed to be rejected again.
			seconds := int(math.Ceil(retryAfter))
			ctx.Header("Retry-After", strconv.Itoa(seconds))
			responses.AbortWithError(ctx, responses.TooManyRequests(seconds))
			return
		}

		ctx.Next()
	}
}
