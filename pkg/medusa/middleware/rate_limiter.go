package middleware

import (
	"math"

	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa/core/responses"
	"github.com/imlargo/ratelimit"
)

// NewRateLimiterMiddleware enforces the limiter's rules on every request that
// reaches it. Requests over the limit get a 429 carrying the standard rate limit
// headers, so a client can back off instead of retrying blind.
//
// The request is handed to the limiter whole, rather than picking an address out
// of it here. That is deliberate: deriving a client address from a forwarding
// header is the step that decides whether the limiter can be bypassed at all,
// and it belongs in one audited place rather than at every call site. In
// particular, gin's ClientIP returns the *leftmost* X-Forwarded-For entry and
// trusts every proxy unless SetTrustedProxies says otherwise, so using it as the
// rate limit key lets any caller pick its own identity by setting a header and
// have no limit whatsoever.
func NewRateLimiterMiddleware(rl *ratelimit.Limiter) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		d := rl.CheckRequest(ctx.Request)
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
