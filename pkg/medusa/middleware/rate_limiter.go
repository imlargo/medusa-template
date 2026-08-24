package middleware

import (
	"math"

	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa/core/responses"
	"github.com/imlargo/ratelimit"
)

// NewRateLimiterMiddleware creates a middleware that enforces rate limiting per
// client IP, using the provided limiter. If the rate limit is exceeded, it
// returns a 429 Too Many Requests response with information about when to
// retry.
func NewRateLimiterMiddleware(rl *ratelimit.Limiter) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		d := rl.Check(ctx.Request.Context(), ratelimit.Subject{
			Identity: ctx.ClientIP(),
			Path:     ctx.Request.URL.Path,
			Method:   ctx.Request.Method,
			Host:     ctx.Request.Host,
		})
		d.WriteHeaders(ctx.Writer.Header())

		if !d.Allowed {
			// Round up: telling a client to retry in 0 seconds invites an
			// immediate retry that is guaranteed to be rejected again.
			seconds := int(math.Ceil(d.RetryAfter.Seconds()))
			responses.AbortWithError(ctx, responses.TooManyRequests(seconds))
			return
		}

		ctx.Next()
	}
}
