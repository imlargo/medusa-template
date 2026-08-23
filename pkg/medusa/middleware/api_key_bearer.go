package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa/core/responses"
)

// BearerApiKeyMiddleware creates a middleware that validates API keys from the Authorization header.
// It expects the key in the format "Bearer <api-key>".
// If the key is missing or invalid, it aborts the request with an Unauthorized response.
func BearerApiKeyMiddleware(apiKey string) gin.HandlerFunc {

	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader("Authorization")

		if authHeader == "" {
			responses.AbortWithError(ctx, responses.Unauthorized("authorization header is missing"))
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			responses.AbortWithError(ctx, responses.Unauthorized("authorization header must be in format 'Bearer token'"))
			return
		}

		apiKeyHeader := parts[1]
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
