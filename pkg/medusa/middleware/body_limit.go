package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// NewBodyLimitMiddleware rejects request bodies larger than maxBytes.
//
// Without a cap the server reads whatever a client sends into memory, so a
// single request can exhaust the process. This is the size-based counterpart to
// a read timeout — and the right tool here, because a time-based deadline on the
// connection would also kill the long-lived SSE streams this app serves.
//
// Reading past the limit surfaces as *http.MaxBytesError, which the binding
// helpers translate into a 413 rather than a generic 400.
func NewBodyLimitMiddleware(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}

		c.Next()
	}
}
