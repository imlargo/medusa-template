package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa"
	"github.com/imlargo/medusa/pkg/medusa/core/logger"
)

// NewContextMiddleware creates a middleware that injects Medusa context
func NewContextMiddleware(logger *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		mc := medusa.NewContext(c, logger)
		c.Set(medusa.ContextKey, mc)
		c.Next()
	}
}

// InjectContext retrieves or creates a Medusa context
func InjectContext(c *gin.Context) *medusa.Context {
	if mc := medusa.GetContext(c); mc != nil {
		return mc
	}
	return medusa.NewContext(c, logger.NewLogger())
}
