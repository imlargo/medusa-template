package medusa

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa/core/logger"
)

// Context keys
const (
	UserIDContextKey    = "medusa_user_id"
	RequestIDContextKey = "medusa_request_id"
	LoggerContextKey    = "medusa_logger"
	StartTimeContextKey = "medusa_start_time"
)

// Helpers for backward compatibility
func GetUserID(c *gin.Context) (uint, bool) {
	id, exists := c.Get(UserIDContextKey)
	if !exists {
		return 0, false
	}
	return id.(uint), true
}

func SetUserID(c *gin.Context, id uint) {
	c.Set(UserIDContextKey, id)
}


// Context wraps gin.Context with additional helpers for common operations.
type Context struct {
	*gin.Context
	logger *logger.Logger
}

// NewContext creates a new Medusa context from a Gin context.
func NewContext(c *gin.Context) *Context {
	ctx := &Context{Context: c}
	if l, exists := c.Get(LoggerContextKey); exists {
		ctx.logger = l.(*logger.Logger)
	}
	return ctx
}

// Ctx returns the standard library context for passing to services.
func (c *Context) Ctx() context.Context {
	return c.Request.Context()
}

// Logger returns the request-scoped logger.
func (c *Context) Logger() *logger.Logger {
	return c.logger
}

// RequestID returns the request ID if set by middleware.
func (c *Context) RequestID() string {
	if id, exists := c.Get(RequestIDContextKey); exists {
		return id.(string)
	}
	return ""
}

// Elapsed returns time elapsed since request start.
func (c *Context) Elapsed() time.Duration {
	if t, exists := c.Get(StartTimeContextKey); exists {
		return time.Since(t.(time.Time))
	}
	return 0
}
