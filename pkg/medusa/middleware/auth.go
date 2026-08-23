// Package middleware provides HTTP middleware components for Medusa applications.
// It includes authentication, authorization, rate limiting, CORS, and metrics collection.
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa"
	"github.com/imlargo/medusa/pkg/medusa/core/jwt"
	"github.com/imlargo/medusa/pkg/medusa/core/responses"
)

// NewJWTAuthMiddleware creates a middleware that validates JWT tokens from the Authorization header.
// It expects the token in the format "Bearer <token>".
// On successful validation, it extracts the user ID from the token claims and stores it in the context.
// If validation fails, it aborts the request with an Unauthorized response.
func NewJWTAuthMiddleware(jwtAuth *jwt.JWT) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		medusaCtx := medusa.NewContext(ctx)

		token, err := medusaCtx.BearerToken()
		if err != nil {
			responses.AbortWithError(ctx, responses.Unauthorized(err.Error()))
			return
		}

		if token == "" {
			responses.AbortWithError(ctx, responses.Unauthorized("token is empty"))
			return
		}

		tokenData, err := jwtAuth.ParseToken(token)
		if err != nil {
			responses.AbortWithError(ctx, responses.Unauthorized("invalid or expired token"))
			return
		}

		medusaCtx.SetUserID(tokenData.UserID)

		ctx.Next()
	}
}
