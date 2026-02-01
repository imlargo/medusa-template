package medusa

import (
	"github.com/imlargo/medusa/pkg/medusa/context"
)

// Context re-exports from context package for backward compatibility.
type Context = context.Context

// NewContext re-exports from context package for backward compatibility.
var NewContext = context.NewContext

// Pagination re-exports from context package for backward compatibility.
type Pagination = context.Pagination

// Paged re-exports from context package for backward compatibility.
// Note: This is a generic function and must be used with type parameters, e.g., medusa.Paged[User](...)
func Paged[T any](c *Context, data []T, page Pagination, totalItems int64) {
	context.Paged(c, data, page, totalItems)
}

// Context key constants re-exported for compatibility.
const (
	UserIDContextKey    = context.UserIDContextKey
	RequestIDContextKey = context.RequestIDContextKey
)
