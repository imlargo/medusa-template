package bootstrap

import (
	"time"

	"github.com/gin-gonic/gin"
	medusactx "github.com/imlargo/medusa/pkg/medusa/context"
	"github.com/imlargo/medusa/pkg/medusa/core/logger"
	"go.uber.org/zap"
)

// accessLogMiddleware logs one structured entry per request through the
// application logger, replacing gin.Default()'s plain-text writer so access logs
// end up in the same JSON stream as everything else.
//
// Server errors (5xx) are logged at error level, client errors (4xx) at warn and
// everything else at info, which keeps log-based alerting straightforward.
func accessLogMiddleware(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		status := c.Writer.Status()
		path := c.FullPath()
		if path == "" {
			// No route matched: log what the client actually asked for.
			path = c.Request.URL.Path
		}

		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Duration("latency", time.Since(start)),
			zap.String("ip", c.ClientIP()),
			zap.String("request_id", c.GetString(medusactx.RequestIDContextKey)),
		}

		if err := c.Errors.ByType(gin.ErrorTypePrivate).String(); err != "" {
			fields = append(fields, zap.String("error", err))
		}

		switch {
		case status >= 500:
			log.Error("request failed", fields...)
		case status >= 400:
			log.Warn("request rejected", fields...)
		default:
			log.Info("request completed", fields...)
		}
	}
}
