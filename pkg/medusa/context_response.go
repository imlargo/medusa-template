package medusa

import (
	"net/http"

	"github.com/imlargo/medusa/pkg/medusa/core/responses"
)

// OK writes a 200 OK response
func (c *Context) OK(data interface{}) {
	responses.SuccessOK(c.Context, data)
}

// Created writes a 201 Created response
func (c *Context) Created(data interface{}) {
	responses.WriteSuccessResponse(c.Context, http.StatusCreated, "Resource created successfully", data)
}

// NoContent writes a 204 No Content response
func (c *Context) NoContent() {
	c.Status(http.StatusNoContent)
}

// BadRequest writes a 400 Bad Request response
func (c *Context) BadRequest(message string) {
	responses.ErrorBadRequest(c.Context, message)
}

// Unauthorized writes a 401 Unauthorized response
func (c *Context) Unauthorized(message string) {
	responses.ErrorUnauthorized(c.Context, message)
}

// NotFound writes a 404 Not Found response
func (c *Context) NotFound(message string) {
	responses.ErrorNotFound(c.Context, message)
}

// InternalError writes a 500 Internal Server Error response
func (c *Context) InternalError(message string, err error) {
	if err != nil {
		c.LogError(err, message)
	}
	responses.ErrorInternalServer(c.Context, nil)
}
