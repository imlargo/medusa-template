package middleware

import (
	"fmt"
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
				
				// Log the panic with details (using fmt for now since logger was removed)
				fmt.Printf("[PANIC RECOVERED] Request ID: %s, Path: %s, Panic: %v\nStack: %s\n",
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
