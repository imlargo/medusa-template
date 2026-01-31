package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/imlargo/medusa/pkg/medusa"
)

const RequestIDHeader = "X-Request-ID"

// NewRequestIDMiddleware creates a middleware that generates or propagates request IDs
func NewRequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(RequestIDHeader)

		if requestID == "" {
			requestID = uuid.New().String()
		}

		c.Set(medusa.RequestIDContextKey, requestID)
		c.Header(RequestIDHeader, requestID)

		c.Next()
	}
}
