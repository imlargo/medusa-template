package middleware

import (
	"log"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa"
)

// RecoveryMiddleware recovers from panics and returns a proper error response.
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				// Log panic details
				ctx := medusa.NewContext(c)
				
				// Get stack trace
				stack := debug.Stack()
				
				// Log the panic with details using standard logger
				// Note: Using standard log package since recovery middleware runs
				// before container initialization and doesn't have access to app logger
				log.Printf("[PANIC RECOVERED] Request ID: %s, Path: %s, Panic: %v\nStack:\n%s\n",
					ctx.RequestID(),
					c.Request.URL.Path,
					r,
					string(stack))
				
				// Return error response
				ctx.AbortWithError(medusa.ErrInternal(nil))
			}
		}()
		c.Next()
	}
}
