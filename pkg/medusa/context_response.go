// pkg/medusa/context_response.go
package medusa

import (
	"math"
	"net/http"
)

// Response represents a standard API response.
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// ErrorBody contains error details.
type ErrorBody struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// ErrorResponseBody is the error response format.
type ErrorResponseBody struct {
	Success bool      `json:"success"`
	Error   ErrorBody `json:"error"`
}

// PagedResponse is the paginated response format.
type PagedResponse[T any] struct {
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
	c.JSON(http.StatusOK, Response{Success: true, Data: data})
}

// OKWithMessage sends a 200 response with data and message.
func (c *Context) OKWithMessage(data interface{}, message string) {
	c.JSON(http.StatusOK, Response{Success: true, Data: data, Message: message})
}

// Created sends a 201 response with data.
func (c *Context) Created(data interface{}) {
	c.JSON(http.StatusCreated, Response{Success: true, Data: data, Message: "created"})
}

// NoContent sends a 204 response.
func (c *Context) NoContent() {
	c.Status(http.StatusNoContent)
}

// Deleted sends a 200 response for successful deletion.
func (c *Context) Deleted() {
	c.JSON(http.StatusOK, Response{Success: true, Message: "deleted"})
}

// Error sends an error response.
func (c *Context) Error(err error) {
	appErr := ToError(err)
	if appErr.Internal != nil && c.logger != nil {
		c.logger.Sugar().Error(appErr.Error())
	}
	c.JSON(appErr.Status, ErrorResponseBody{
		Success: false,
		Error: ErrorBody{
			Code:    appErr.Code,
			Message: appErr.Message,
			Details: appErr.Details,
		},
	})
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
