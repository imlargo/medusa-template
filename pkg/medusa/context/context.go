package context

import (
	stdcontext "context"

	"github.com/gin-gonic/gin"
)

// Context keys
const (
	UserIDContextKey    = "medusa_user_id"
	RequestIDContextKey = "medusa_request_id"
)

// Context wraps gin.Context with additional helpers for common operations.
type Context struct {
	*gin.Context
}

// NewContext creates a new Medusa context from a Gin context.
func NewContext(c *gin.Context) *Context {
	return &Context{Context: c}
}

// Ctx returns the standard library context for passing to services.
func (c *Context) Ctx() stdcontext.Context {
	return c.Request.Context()
}

// RequestID returns the request ID if set by middleware.
func (c *Context) RequestID() string {
	if id, exists := c.Get(RequestIDContextKey); exists {
		if strID, ok := id.(string); ok {
			return strID
		}
	}
	return ""
}
