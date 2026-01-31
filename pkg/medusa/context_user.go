package medusa

import (
	"net/http"

	"github.com/imlargo/medusa/pkg/medusa/core/responses"
)

// MustGetUserID returns the user ID or aborts with unauthorized error
func (c *Context) MustGetUserID() uint {
	userID, err := c.GetUserID()
	if err != nil {
		c.AbortWithError(
			http.StatusUnauthorized,
			responses.ErrUnauthorized,
			"Authentication required",
			nil,
		)
		panic(err)
	}
	return userID
}

// AbortWithError aborts the request and writes an error response
func (c *Context) AbortWithError(statusCode int, errCode responses.ErrorCode, message string, details interface{}) {
	c.Abort()
	responses.WriteErrorResponse(c.Context, statusCode, errCode, message, details)
}
