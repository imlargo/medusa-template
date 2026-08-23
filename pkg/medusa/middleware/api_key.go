package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa/core/responses"
)

// ApiKeyMiddleware creates a middleware that validates API keys from the X-API-Key header.
// It compares the header value against the provided API key.
// If the key is missing or invalid, it aborts the request with an Unauthorized response.
func ApiKeyMiddleware(apiKey string) gin.HandlerFunc {

	return func(ctx *gin.Context) {
		apiKeyHeader := ctx.GetHeader("X-API-Key")

		if apiKeyHeader == "" {
			responses.AbortWithError(ctx, responses.Unauthorized("authorization header is missing"))
			return
		}

		if apiKeyHeader != apiKey {
			responses.AbortWithError(ctx, responses.Unauthorized("invalid API key"))
			return
		}

		ctx.Next()
	}
}
