package context

import (
	stdcontext "context"

	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa/core/responses"
)

// Context keys.
const (
	// UserIDContextKey holds the authenticated user's ID, set by the auth middleware.
	UserIDContextKey = "medusa_user_id"

	// RequestIDContextKey holds the current request ID. It aliases
	// responses.RequestIDKey, which is the single definition, so the key used by
	// the middleware, the responses and this Context can never drift apart.
	RequestIDContextKey = responses.RequestIDKey
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

// UserAgent returns the User-Agent header from the request.
func (c *Context) UserAgent() string {
	return c.GetHeader("User-Agent")
}

// Referer returns the Referer header from the request.
func (c *Context) Referer() string {
	return c.GetHeader("Referer")
}

// AcceptLanguage returns the Accept-Language header from the request.
func (c *Context) AcceptLanguage() string {
	return c.GetHeader("Accept-Language")
}
