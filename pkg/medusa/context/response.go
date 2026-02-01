package context

import (
	"math"
	"net/http"

	"github.com/imlargo/medusa/pkg/medusa/core/responses"
)

// ErrorConverter is an interface for converting errors.
// This avoids circular dependencies with the medusa package.
type ErrorConverter interface {
	ToError(err error) ConvertedError
}

// ConvertedError represents a converted error with all necessary fields.
type ConvertedError struct {
	Code     responses.ErrorCode
	Message  string
	Details  interface{}
	Internal error
}

// defaultErrorConverter is set by the medusa package to avoid circular imports.
var defaultErrorConverter ErrorConverter

// SetErrorConverter sets the error converter for the context package.
func SetErrorConverter(ec ErrorConverter) {
	defaultErrorConverter = ec
}

// PagedResponse is the paginated response format.
type PagedResponse[T any] struct {
	Status     int            `json:"status"`
	Success    bool           `json:"success"`
	Data       []T            `json:"data"`
	Pagination PaginationMeta `json:"pagination"`
	RequestID  string         `json:"request_id,omitempty"` // Request ID for tracing
}

// PaginationMeta contains pagination metadata.
type PaginationMeta struct {
	Page       int   `json:"page"`        // Current page number
	PageSize   int   `json:"page_size"`   // Number of items per page
	TotalItems int64 `json:"total_items"` // Total number of items
	TotalPages int   `json:"total_pages"` // Total number of pages
	HasNext    bool  `json:"has_next"`    // Whether there is a next page
	HasPrev    bool  `json:"has_prev"`    // Whether there is a previous page
}

// OK sends a 200 response with data.
func (c *Context) OK(data interface{}) {
	responses.SuccessOK(c.Context, data)
}

// Created sends a 201 response with data.
func (c *Context) Created(data interface{}) {
	responses.SuccessCreated(c.Context, data)
}

// Updated sends a 200 response for successful update.
func (c *Context) Updated(data interface{}) {
	responses.SuccessUpdated(c.Context, data)
}

// Deleted sends a 200 response for successful deletion.
func (c *Context) Deleted() {
	responses.SuccessDeleted(c.Context)
}

// NoContent sends a 204 response.
func (c *Context) NoContent() {
	c.Status(http.StatusNoContent)
}

// Error sends an error response.
func (c *Context) Error(err error) {
	if defaultErrorConverter == nil {
		// Fallback if error converter not set
		responses.ErrorInternalServer(c.Context, nil)
		return
	}

	appErr := defaultErrorConverter.ToError(err)

	// Map to responses package error codes
	switch appErr.Code {
	case responses.ErrCodeBadRequest:
		responses.WriteErrorResponse(c.Context, http.StatusBadRequest, responses.ErrCodeBadRequest, appErr.Message, appErr.Details)
	case responses.ErrCodeBindJson:
		responses.WriteErrorResponse(c.Context, http.StatusBadRequest, responses.ErrCodeBindJson, appErr.Message, appErr.Details)
	case responses.ErrCodeUnauthorized:
		responses.ErrorUnauthorized(c.Context, appErr.Message)
	case responses.ErrCodeForbidden:
		responses.ErrorForbidden(c.Context, appErr.Message)
	case responses.ErrCodeNotFound:
		responses.WriteErrorResponse(c.Context, http.StatusNotFound, responses.ErrCodeNotFound, appErr.Message, appErr.Details)
	case responses.ErrCodeConflict:
		responses.ErrorConflict(c.Context, appErr.Message)
	case responses.ErrCodeTooManyRequests:
		responses.ErrorTooManyRequests(c.Context, appErr.Message)
	case responses.ErrCodeServiceUnavailable:
		responses.ErrorServiceUnavailable(c.Context, appErr.Message)
	case responses.ErrCodeInternalServer:
		// Always use the custom message for internal server errors
		// The Internal field is kept for logging but never exposed to clients
		responses.ErrorInternalServerWithMessage(c.Context, appErr.Message, appErr.Details)
	default:
		// Handle unknown error codes
		responses.WriteErrorResponse(c.Context, http.StatusBadRequest, responses.ErrCodeBadRequest, appErr.Message, appErr.Details)
	}
}

// AbortWithError sends an error and aborts the middleware chain.
func (c *Context) AbortWithError(err error) {
	c.Error(err)
	c.Abort()
}

// Paged sends a paginated response.
func Paged[T any](c *Context, data []T, page Pagination, totalItems int64) {
	pageSize := page.GetPageSize(DefaultPageSize)
	totalPages := int(math.Ceil(float64(totalItems) / float64(pageSize)))
	currentPage := page.GetPage()

	// Extract request ID if available
	requestID := c.RequestID()

	c.JSON(http.StatusOK, PagedResponse[T]{
		Status:  http.StatusOK,
		Success: true,
		Data:    data,
		Pagination: PaginationMeta{
			Page:       currentPage,
			PageSize:   pageSize,
			TotalItems: totalItems,
			TotalPages: totalPages,
			HasNext:    currentPage < totalPages,
			HasPrev:    currentPage > 1,
		},
		RequestID: requestID,
	})
}
