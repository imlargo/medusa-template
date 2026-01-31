package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa"
)

// RecoveryMiddleware recovers from panics and returns a proper error response.
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				ctx := medusa.NewContext(c)
				ctx.AbortWithError(medusa.ErrInternal(nil))
			}
		}()
		c.Next()
	}
}
