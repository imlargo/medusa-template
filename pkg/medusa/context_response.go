package medusa

import (
	"math"
	"net/http"

	"github.com/imlargo/medusa/pkg/medusa/core/responses"
)

// PagedResponse is the paginated response format.
type PagedResponse[T any] struct {
	Status     int            `json:"status"`
	Success    bool           `json:"success"`
	Data       []T            `json:"data"`
	Pagination PaginationMeta `json:"pagination"`
}

// PaginationMeta contains pagination metadata.
type PaginationMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
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
	appErr := ToError(err)
	
	// Map to responses package error codes
	switch appErr.Code {
	case responses.ErrCodeBadRequest:
		responses.ErrorBadRequest(c.Context, appErr.Message)
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
		if appErr.Internal != nil {
			responses.ErrorInternalServer(c.Context, appErr.Details)
		} else {
			responses.ErrorInternalServerWithMessage(c.Context, appErr.Message, appErr.Details)
		}
	default:
		// Handle unknown error codes
		responses.ErrorBadRequest(c.Context, appErr.Message)
	}
}

// AbortWithError sends an error and aborts the middleware chain.
func (c *Context) AbortWithError(err error) {
	c.Error(err)
	c.Abort()
}

// Paged sends a paginated response.
func Paged[T any](c *Context, data []T, page Pagination, totalItems int64) {
	pageSize := page.GetPageSize(20)
	totalPages := int(math.Ceil(float64(totalItems) / float64(pageSize)))
	currentPage := page.GetPage()

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
	})
}
