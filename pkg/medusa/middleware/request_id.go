// pkg/medusa/middleware/request_id.go
package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/imlargo/medusa/pkg/medusa"
	"github.com/imlargo/medusa/pkg/medusa/core/logger"
)

// RequestIDMiddleware adds a unique request ID to each request.
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set(medusa.RequestIDContextKey, requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// ContextMiddleware sets up the Medusa context with logger and start time.
func ContextMiddleware(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(medusa.StartTimeContextKey, time.Now())
		c.Set(medusa.LoggerContextKey, log)
		c.Next()
	}
}

// RecoveryMiddleware recovers from panics and returns a proper error response.
func RecoveryMiddleware(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				ctx := medusa.NewContext(c)
				log.Sugar().Errorw("panic recovered",
					"panic", r,
					"request_id", ctx.RequestID(),
					"path", c.Request.URL.Path,
				)
				ctx.Error(medusa.ErrInternal(nil))
				c.Abort()
			}
		}()
		c.Next()
	}
}

// LoggingMiddleware logs requests with timing information.
func LoggingMiddleware(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		log.Sugar().Infow("request",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"duration", time.Since(start).String(),
			"ip", c.ClientIP(),
		)
	}
}
