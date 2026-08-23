// Package responses defines the wire format of every HTTP response the API
// emits, the error model behind it, and the two functions that write them.
//
// There is exactly one way to send a success and one way to send a failure:
//
//	responses.WriteSuccess(c, http.StatusOK, "ok", user)
//	responses.WriteError(c, responses.NotFound("user"))
//
// Handlers rarely call either directly. They return an *Error and let the
// medusa.Handle adapters render it, which keeps status codes and the response
// shape in one place instead of spread across every handler.
//
// Success format:
//
//	{"status": 200, "success": true, "message": "ok", "data": {...}}
//
// Error format:
//
//	{"status": 404, "code": "NOT_FOUND", "error": "user not found", "request_id": "..."}
package responses

import "github.com/gin-gonic/gin"

// RequestIDKey is the Gin context key holding the current request ID.
//
// This is the single definition in the module: the request-ID middleware writes
// it, this package reads it into every response, and medusa's Context exposes it
// through Context.RequestID. Anything that needs the key must reference this
// constant rather than repeating the literal.
const RequestIDKey = "medusa_request_id"

// requestID returns the request ID attached by the request-ID middleware, or an
// empty string when the middleware is not installed.
func requestID(c *gin.Context) string {
	id, ok := c.Get(RequestIDKey)
	if !ok {
		return ""
	}

	value, _ := id.(string)
	return value
}
