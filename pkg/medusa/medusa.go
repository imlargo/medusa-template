// Package medusa is the entry point application code imports.
//
// It is a thin facade: the Context wrapper and pagination types come from
// medusa/context, the error model and response format from
// medusa/core/responses, and this package adds the Handle* adapters that connect
// a handler returning (Res, error) to Gin.
//
// A typical handler and its route:
//
//	func (h *UserHandler) Get(ctx *medusa.Context) (*dto.User, error) {
//	    id, err := ctx.ParamID("id")
//	    if err != nil {
//	        return nil, err
//	    }
//	    user, err := h.users.ByID(ctx.Ctx(), id)
//	    if errors.Is(err, gorm.ErrRecordNotFound) {
//	        return nil, responses.NotFound("user")
//	    }
//	    return user, err
//	}
//
//	users.GET("/:id", medusa.HandleGet(h.Get))
package medusa

import "github.com/imlargo/medusa/pkg/medusa/context"

// Context is the request context handed to every Medusa handler. It embeds
// *gin.Context, so every Gin method remains available.
type Context = context.Context

// NewContext wraps a *gin.Context. The Handle* adapters call it for you; it is
// exported for middleware and for handlers registered directly with Gin.
var NewContext = context.NewContext

// Pagination holds the page and page_size query parameters.
type Pagination = context.Pagination

// Context keys, re-exported so application middleware can read them without
// importing the context package.
const (
	UserIDContextKey    = context.UserIDContextKey
	RequestIDContextKey = context.RequestIDContextKey
)

// Paged replies 200 with one page of results plus the metadata to walk the rest.
// It is a function rather than a method because Go does not allow type
// parameters on methods.
func Paged[T any](c *Context, data []T, page Pagination, totalItems int64) {
	context.Paged(c, data, page, totalItems)
}
