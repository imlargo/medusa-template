package medusa

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa/context"
	"github.com/imlargo/medusa/pkg/medusa/core/responses"
)

// The adapters below turn a handler that *returns* its result into a
// gin.HandlerFunc. Binding, status selection and error rendering happen once,
// here, instead of in every handler:
//
//	// Business logic only. No *gin.Context, no response writing.
//	func (h *UserHandler) Create(ctx *medusa.Context, in *dto.NewUser) (*dto.User, error) {
//	    user, err := h.users.Create(ctx.Ctx(), in)
//	    if err != nil {
//	        return nil, err
//	    }
//	    return user, nil
//	}
//
//	// Wired once, at the route.
//	users.POST("", medusa.HandleCreate(h.Create))
//
// Any error is rendered by responses.WriteError, which derives the status from
// the error: return responses.NotFound("user") for a 404, and an unclassified
// error becomes a 500 with its cause sent to the log and never to the client.

// HandlerFunc is a handler that writes its own response and only reports errors.
// Use it for endpoints that do not fit the typed adapters — streaming, file
// downloads, redirects.
type HandlerFunc func(*context.Context) error

// Handler adapts a HandlerFunc. Nothing is written on success; the handler is
// expected to have produced the response itself.
func Handler(fn HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.NewContext(c)
		if err := fn(ctx); err != nil {
			responses.WriteError(c, err)
		}
	}
}

// TypedHandler takes a decoded, validated request body and returns the payload
// to send back.
type TypedHandler[Req any, Res any] func(*context.Context, *Req) (Res, error)

// NoBodyHandler returns a payload without reading a request body.
type NoBodyHandler[Res any] func(*context.Context) (Res, error)

// Handle binds and validates the JSON body, then replies 200 with the result.
func Handle[Req any, Res any](fn TypedHandler[Req, Res]) gin.HandlerFunc {
	return handleWithBody(fn, http.StatusOK, responses.MessageOK)
}

// HandleCreate binds and validates the JSON body, then replies 201 with the
// created resource.
func HandleCreate[Req any, Res any](fn TypedHandler[Req, Res]) gin.HandlerFunc {
	return handleWithBody(fn, http.StatusCreated, responses.MessageCreated)
}

// HandleUpdate binds and validates the JSON body, then replies 200 with the
// updated resource.
func HandleUpdate[Req any, Res any](fn TypedHandler[Req, Res]) gin.HandlerFunc {
	return handleWithBody(fn, http.StatusOK, responses.MessageUpdated)
}

// HandleGet replies 200 with the result, without reading a body.
func HandleGet[Res any](fn NoBodyHandler[Res]) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.NewContext(c)

		result, err := fn(ctx)
		if err != nil {
			responses.WriteError(c, err)
			return
		}

		responses.WriteSuccess(c, http.StatusOK, responses.MessageOK, result)
	}
}

// HandleDelete replies 200 confirming the deletion, or renders the error.
func HandleDelete(fn HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.NewContext(c)

		if err := fn(ctx); err != nil {
			responses.WriteError(c, err)
			return
		}

		responses.WriteSuccess(c, http.StatusOK, responses.MessageDeleted, nil)
	}
}

// handleWithBody is the shared body of the adapters that read a request body.
func handleWithBody[Req any, Res any](fn TypedHandler[Req, Res], status int, message string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.NewContext(c)

		var request Req
		if err := ctx.Bind(&request); err != nil {
			responses.WriteError(c, err)
			return
		}

		result, err := fn(ctx, &request)
		if err != nil {
			responses.WriteError(c, err)
			return
		}

		responses.WriteSuccess(c, status, message, result)
	}
}
