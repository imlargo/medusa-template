package context

import (
	"math"
	"net/http"

	"github.com/imlargo/medusa/pkg/medusa/core/responses"
)

// PagedResponse is the JSON body of a paginated list.
type PagedResponse[T any] struct {
	Status     int            `json:"status"`
	Success    bool           `json:"success"`
	Data       []T            `json:"data"`
	Pagination PaginationMeta `json:"pagination"`
	RequestID  string         `json:"request_id,omitempty"`
}

// PaginationMeta describes where the returned page sits in the full result set.
type PaginationMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

// OK replies 200 with data.
func (c *Context) OK(data any) {
	responses.WriteSuccess(c.Context, http.StatusOK, responses.MessageOK, data)
}

// Created replies 201 with the newly created resource.
func (c *Context) Created(data any) {
	responses.WriteSuccess(c.Context, http.StatusCreated, responses.MessageCreated, data)
}

// Updated replies 200 with the updated resource.
func (c *Context) Updated(data any) {
	responses.WriteSuccess(c.Context, http.StatusOK, responses.MessageUpdated, data)
}

// Deleted replies 200 confirming a deletion.
func (c *Context) Deleted() {
	responses.WriteSuccess(c.Context, http.StatusOK, responses.MessageDeleted, nil)
}

// NoContent replies 204 with an empty body.
func (c *Context) NoContent() {
	responses.WriteNoContent(c.Context)
}

// Fail renders err as the response, deriving the status from the error.
//
// It is named Fail rather than Error on purpose: gin.Context.Error means
// "record this error for later", and a method that shadowed it while doing
// something entirely different — writing the response and ending the request —
// would be a trap for anyone porting Gin code. Handlers wrapped by medusa.Handle
// return their error instead and never call this.
func (c *Context) Fail(err error) {
	responses.WriteError(c.Context, err)
}

// AbortWithFailure renders err and stops the middleware chain.
func (c *Context) AbortWithFailure(err error) {
	responses.AbortWithError(c.Context, err)
}

// Paged replies 200 with one page of results and the metadata to walk the rest.
func Paged[T any](c *Context, data []T, page Pagination, totalItems int64) {
	pageSize := page.GetPageSize(DefaultPageSize)
	currentPage := page.GetPage()
	totalPages := int(math.Ceil(float64(totalItems) / float64(pageSize)))

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
		RequestID: c.RequestID(),
	})
}
