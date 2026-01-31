package medusa

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa/core/logger"
)

// Context keys (kept for backward compatibility)
const (
	ContextUserIDKey    = "medusa_user_id"
	ContextRequestIDKey = "medusa_request_id"
	ContextStartTimeKey = "medusa_start_time"
)

// Helpers type-safe (kept for backward compatibility)
func GetUserID(c *gin.Context) (uint, bool) {
	id, exists := c.Get(ContextUserIDKey)
	if !exists {
		return 0, false
	}
	return id.(uint), true
}

func SetUserID(c *gin.Context, id uint) {
	c.Set(ContextUserIDKey, id)
}

var (
	ErrUserNotAuthenticated = errors.New("user not authenticated")
	ErrInvalidUserID        = errors.New("invalid user id in context")
)

// Context wraps gin.Context with additional helpers.
// It provides a cleaner API for common operations while
// maintaining full access to the underlying Gin context.
type Context struct {
	*gin.Context
	logger *logger.Logger
}

// NewContext creates a new Medusa context from a Gin context.
func NewContext(c *gin.Context, logger *logger.Logger) *Context {
	ctx := &Context{
		Context: c,
		logger:  logger,
	}
	return ctx
}

// GetContext retrieves the Medusa context from gin.Context
func GetContext(c *gin.Context) *Context {
	if mc, exists := c.Get(ContextKey); exists {
		if medusaCtx, ok := mc.(*Context); ok {
			return medusaCtx
		}
	}
	return nil
}

// Logger returns the logger instance
func (c *Context) Logger() *logger.Logger {
	return c.logger
}

// Ctx returns the standard library context.
// Use this when calling services that need context.Context.
func (c *Context) Ctx() context.Context {
	return c.Request.Context()
}

// GetRequestID returns the request ID if set.
func (c *Context) GetRequestID() string {
	if reqID, exists := c.Get(RequestIDContextKey); exists {
		if id, ok := reqID.(string); ok {
			return id
		}
	}
	return ""
}

// RequestID returns the request ID if set (backward compatibility).
func (c *Context) RequestID() string {
	return c.GetRequestID()
}

// SetRequestID sets the request ID (typically called by middleware).
func (c *Context) SetRequestID(id string) {
	c.Set(ContextRequestIDKey, id)
	c.Header("X-Request-ID", id)
}

// GetParamUint gets a URL parameter as uint
func (c *Context) GetParamUint(key string) (uint, error) {
	param := c.Param(key)
	if param == "" {
		return 0, fmt.Errorf("parameter '%s' not found", key)
	}
	
	value, err := strconv.ParseUint(param, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parameter '%s' must be a positive integer", key)
	}
	
	return uint(value), nil
}

// GetQueryInt gets a query parameter as int with default value
func (c *Context) GetQueryInt(key string, defaultValue int) int {
	if value := c.Query(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// LogError logs an error with context information
func (c *Context) LogError(err error, message string) {
	fields := []interface{}{
		"error", err.Error(),
		"request_id", c.GetRequestID(),
		"path", c.Request.URL.Path,
		"method", c.Request.Method,
	}
	c.logger.Sugar().Errorw(message, fields...)
}

// StartTime returns when the request started.
func (c *Context) StartTime() time.Time {
	if t, exists := c.Get(ContextStartTimeKey); exists {
		return t.(time.Time)
	}
	return time.Time{}
}

// Elapsed returns the time elapsed since request start.
func (c *Context) Elapsed() time.Duration {
	start := c.StartTime()
	if start.IsZero() {
		return 0
	}
	return time.Since(start)
}

// SetValue sets a value in the context.
func (c *Context) SetValue(key string, value interface{}) {
	c.Set(key, value)
}

// GetValue gets a value from the context.
func (c *Context) GetValue(key string) (interface{}, bool) {
	return c.Get(key)
}

// MustGetValue gets a value or panics if not found.
func (c *Context) MustGetValue(key string) interface{} {
	val, exists := c.Get(key)
	if !exists {
		panic("medusa: required context value not found: " + key)
	}
	return val
}

// GetUserID returns the authenticated user's ID with error handling.
func (c *Context) GetUserID() (uint, error) {
	userID, exists := c.Get(UserIDContextKey)
	if !exists {
		return 0, ErrUserNotAuthenticated
	}
	
	switch v := userID.(type) {
	case uint:
		return v, nil
	case uint64:
		return uint(v), nil
	case int:
		return uint(v), nil
	case int64:
		return uint(v), nil
	case float64:
		return uint(v), nil
	default:
		return 0, ErrInvalidUserID
	}
}

// UserID returns the authenticated user's ID (backward compatibility).
// Returns 0 and false if user is not authenticated.
func (c *Context) UserID() (uint, bool) {
	id, exists := c.Get(ContextUserIDKey)
	if !exists {
		return 0, false
	}

	switch v := id.(type) {
	case uint:
		return v, true
	case int:
		return uint(v), true
	case int64:
		return uint(v), true
	case float64:
		return uint(v), true
	default:
		return 0, false
	}
}

// SetUserID sets the authenticated user ID (called by auth middleware).
func (c *Context) SetUserID(id uint) {
	c.Set(ContextUserIDKey, id)
	c.Set(UserIDContextKey, id)
}

// IsAuthenticated returns true if user is authenticated.
func (c *Context) IsAuthenticated() bool {
	_, exists := c.UserID()
	return exists
}
