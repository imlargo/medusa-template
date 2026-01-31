package medusa

import (
	"errors"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/imlargo/medusa/pkg/medusa/core/responses"
)

// BindJSON binds and validates JSON request body
func (c *Context) BindJSON(obj interface{}) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		c.handleBindError(err)
		return false
	}
	return true
}

// BindQuery binds and validates query parameters
func (c *Context) BindQuery(obj interface{}) bool {
	if err := c.ShouldBindQuery(obj); err != nil {
		c.handleBindError(err)
		return false
	}
	return true
}

func (c *Context) handleBindError(err error) {
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		responses.ErrorValidation(c.Context, err)
		return
	}
	
	responses.WriteErrorResponse(
		c.Context,
		http.StatusBadRequest,
		responses.ErrBindJson,
		"Invalid request format",
		map[string]string{"error": err.Error()},
	)
}
