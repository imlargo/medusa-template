// pkg/medusa/context_request.go
package context

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
)

// ErrorConstructor is an interface for creating errors.
// This avoids circular dependencies with the medusa package.
type ErrorConstructor interface {
	Validation(message string, details interface{}) error
	BadRequest(message string) error
}

// defaultErrorConstructor is set by the medusa package to avoid circular imports.
var defaultErrorConstructor ErrorConstructor

// SetErrorConstructor sets the error constructor for the context package.
// This is called by the medusa package during initialization.
func SetErrorConstructor(ec ErrorConstructor) {
	defaultErrorConstructor = ec
}

// Bind binds and validates the request body to the given struct.
func (c *Context) Bind(obj interface{}) error {
	if err := c.ShouldBindJSON(obj); err != nil {
		return c.formatValidationError(err)
	}
	return nil
}

// BindQuery binds and validates query parameters.
func (c *Context) BindQuery(obj interface{}) error {
	if err := c.ShouldBindQuery(obj); err != nil {
		return c.formatValidationError(err)
	}
	return nil
}

// BindURI binds and validates URI parameters.
func (c *Context) BindURI(obj interface{}) error {
	if err := c.ShouldBindUri(obj); err != nil {
		return c.formatValidationError(err)
	}
	return nil
}

func (c *Context) formatValidationError(err error) error {
	if defaultErrorConstructor == nil {
		return err
	}
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		details := make(map[string]string)
		for _, e := range validationErrors {
			field := strings.ToLower(e.Field())
			details[field] = formatValidationMessage(e)
		}
		return defaultErrorConstructor.Validation("validation failed", details)
	}
	return defaultErrorConstructor.BadRequest("invalid request body")
}

func formatValidationMessage(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return "this field is required"
	case "email":
		return "invalid email format"
	case "min":
		// Check if it's a string/slice field or numeric field
		if e.Type().Kind() == reflect.String || e.Type().Kind() == reflect.Slice {
			return "must be at least " + e.Param() + " characters"
		}
		return "must be at least " + e.Param()
	case "max":
		// Check if it's a string/slice field or numeric field
		if e.Type().Kind() == reflect.String || e.Type().Kind() == reflect.Slice {
			return "must be at most " + e.Param() + " characters"
		}
		return "must be at most " + e.Param()
	case "gte":
		return "must be greater than or equal to " + e.Param()
	case "lte":
		return "must be less than or equal to " + e.Param()
	case "oneof":
		return "must be one of: " + e.Param()
	case "url":
		return "invalid URL format"
	case "uuid":
		return "invalid UUID format"
	default:
		return "invalid value"
	}
}

// ParamID gets a URL parameter as uint.
func (c *Context) ParamID(name string) (uint, error) {
	param := c.Param(name)
	if param == "" {
		if defaultErrorConstructor != nil {
			return 0, defaultErrorConstructor.BadRequest(name + " is required")
		}
		return 0, nil
	}
	id, err := strconv.ParseUint(param, 10, 64)
	if err != nil {
		if defaultErrorConstructor != nil {
			return 0, defaultErrorConstructor.BadRequest("invalid " + name)
		}
		return 0, err
	}
	return uint(id), nil
}

// QueryInt gets a query parameter as int with default value.
func (c *Context) QueryInt(name string, defaultValue int) int {
	val := c.Query(name)
	if val == "" {
		return defaultValue
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}
	return i
}

// QueryBool gets a query parameter as bool with default value.
func (c *Context) QueryBool(name string, defaultValue bool) bool {
	val := c.Query(name)
	if val == "" {
		return defaultValue
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return defaultValue
	}
	return b
}

// Pagination represents pagination parameters.
type Pagination struct {
	Page     int `form:"page" binding:"omitempty,min=1"`
	PageSize int `form:"page_size" binding:"omitempty,min=1"`
}

// GetPage returns validated page number (minimum 1).
func (p Pagination) GetPage() int {
	if p.Page < 1 {
		return 1
	}
	return p.Page
}

// GetPageSize returns validated page size with default.
func (p Pagination) GetPageSize(defaultSize int) int {
	if p.PageSize < 1 {
		return defaultSize
	}
	if p.PageSize > 100 {
		return 100
	}
	return p.PageSize
}

// Offset calculates SQL offset for pagination.
func (p Pagination) Offset(defaultSize int) int {
	return (p.GetPage() - 1) * p.GetPageSize(defaultSize)
}

// Pagination gets pagination params from query string.
func (c *Context) Pagination() Pagination {
	return Pagination{
		Page:     c.QueryInt("page", 1),
		PageSize: c.QueryInt("page_size", 20),
	}
}

// ClientIP returns the real client IP.
func (c *Context) ClientIP() string {
	return c.Context.ClientIP()
}

// BearerToken extracts the token from Authorization header.
func (c *Context) BearerToken() string {
	auth := c.GetHeader("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}
