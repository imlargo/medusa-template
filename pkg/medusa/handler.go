// pkg/medusa/handler.go
package medusa

import (
	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa/context"
)

// HandlerFunc is the signature for Medusa handlers.
type HandlerFunc func(*context.Context) error

// Handler wraps a Medusa handler to work with Gin.
func Handler(fn HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.NewContext(c)
		if err := fn(ctx); err != nil {
			ctx.Error(err)
		}
	}
}

// TypedHandler is a handler with typed request and response.
type TypedHandler[Req any, Res any] func(*context.Context, *Req) (Res, error)

// Handle creates a Gin handler from a typed Medusa handler.
func Handle[Req any, Res any](fn TypedHandler[Req, Res]) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.NewContext(c)
		var req Req
		if err := ctx.Bind(&req); err != nil {
			ctx.Error(err)
			return
		}
		res, err := fn(ctx, &req)
		if err != nil {
			ctx.Error(err)
			return
		}
		ctx.OK(res)
	}
}

// HandleCreate is like Handle but returns 201 Created.
func HandleCreate[Req any, Res any](fn TypedHandler[Req, Res]) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.NewContext(c)
		var req Req
		if err := ctx.Bind(&req); err != nil {
			ctx.Error(err)
			return
		}
		res, err := fn(ctx, &req)
		if err != nil {
			ctx.Error(err)
			return
		}
		ctx.Created(res)
	}
}

// NoBodyHandler is a handler without request body.
type NoBodyHandler[Res any] func(*context.Context) (Res, error)

// HandleGet creates a handler for GET requests (no body).
func HandleGet[Res any](fn NoBodyHandler[Res]) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.NewContext(c)
		res, err := fn(ctx)
		if err != nil {
			ctx.Error(err)
			return
		}
		ctx.OK(res)
	}
}

// HandleDelete creates a handler for DELETE requests.
func HandleDelete(fn func(*context.Context) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.NewContext(c)
		if err := fn(ctx); err != nil {
			ctx.Error(err)
			return
		}
		ctx.Deleted()
	}
}
