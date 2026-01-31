// Package middleware provides HTTP middleware components for Medusa applications.
// It includes authentication, authorization, rate limiting, CORS, and metrics collection.
package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa"
	"github.com/imlargo/medusa/pkg/medusa/core/jwt"
	"github.com/imlargo/medusa/pkg/medusa/core/responses"
)

// NewAuthTokenMiddleware creates a middleware that validates JWT tokens from the Authorization header.
// It expects the token in the format "Bearer <token>".
// On successful validation, it extracts the user ID from the token claims and stores it in the context.
// If validation fails, it aborts the request with an Unauthorized response.
func NewAuthTokenMiddleware(jwtAuthenticator *jwt.JWT) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader("Authorization")

		if authHeader == "" {
			ctx.Abort()
			responses.ErrorUnauthorized(ctx, "authorization header is missing")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			ctx.Abort()
			responses.ErrorUnauthorized(ctx, "authorization header must be in format 'Bearer token'")
			return
		}

		token := parts[1]
		if token == "" {
			ctx.Abort()
			responses.ErrorUnauthorized(ctx, "token is empty")
			return
		}

		tokenData, err := jwtAuthenticator.ParseToken(token)
		if err != nil {
			ctx.Abort()
			responses.ErrorUnauthorized(ctx, err.Error())
			return
		}

		medusaCtx := medusa.NewContext(ctx)
		medusaCtx.SetUserID(tokenData.UserID)

		ctx.Next()
	}
}
