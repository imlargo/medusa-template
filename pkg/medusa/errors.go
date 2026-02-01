// pkg/medusa/errors.go
package medusa

import (
	"fmt"
	"net/http"

	"github.com/imlargo/medusa/pkg/medusa/core/responses"
)

// Error represents an application error with HTTP context.
type Error struct {
	Code     responses.ErrorCode `json:"code"`
	Message  string              `json:"message"`
	Status   int                 `json:"-"`
	Details  interface{}         `json:"details,omitempty"`
	Internal error               `json:"-"`
}

// Error returns the error message, implementing the error interface.
func (e *Error) Error() string {
	if e.Internal != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Internal)
	}
	return e.Message
}

// Unwrap returns the internal error, implementing errors.Unwrap.
func (e *Error) Unwrap() error {
	return e.Internal
}

// WithDetails adds or replaces details on the error.
func (e *Error) WithDetails(details interface{}) *Error {
	e.Details = details
	return e
}

// Error constructors

func ErrBadRequest(message string) *Error {
	return &Error{Code: responses.ErrCodeBadRequest, Message: message, Status: http.StatusBadRequest}
}

func ErrValidation(message string, details interface{}) *Error {
	return &Error{Code: responses.ErrCodeBindJson, Message: message, Status: http.StatusBadRequest, Details: details}
}

func ErrUnauthorized(message string) *Error {
	return &Error{Code: responses.ErrCodeUnauthorized, Message: message, Status: http.StatusUnauthorized}
}

func ErrForbidden(message string) *Error {
	return &Error{Code: responses.ErrCodeForbidden, Message: message, Status: http.StatusForbidden}
}

func ErrNotFound(resource string) *Error {
	return &Error{Code: responses.ErrCodeNotFound, Message: fmt.Sprintf("%s not found", resource), Status: http.StatusNotFound}
}

func ErrConflict(message string) *Error {
	return &Error{Code: responses.ErrCodeConflict, Message: message, Status: http.StatusConflict}
}

func ErrInternal(err error) *Error {
	return &Error{Code: responses.ErrCodeInternalServer, Message: "an internal error occurred", Status: http.StatusInternalServerError, Internal: err}
}

func ErrInternalWithMessage(message string, err error) *Error {
	return &Error{Code: responses.ErrCodeInternalServer, Message: message, Status: http.StatusInternalServerError, Internal: err}
}

func ErrTooManyRequests(retryAfter int) *Error {
	return &Error{
		Code: responses.ErrCodeTooManyRequests, Message: "rate limit exceeded", Status: http.StatusTooManyRequests,
		Details: map[string]int{"retry_after_seconds": retryAfter},
	}
}

func ErrServiceUnavailable(message string) *Error {
	return &Error{Code: responses.ErrCodeServiceUnavailable, Message: message, Status: http.StatusServiceUnavailable}
}

// ToError converts any error to *Error.
func ToError(err error) *Error {
	if e, ok := err.(*Error); ok {
		return e
	}
	return ErrInternal(err)
}

// Wrap wraps an error with a message.
func Wrap(err error, message string) *Error {
	if appErr, ok := err.(*Error); ok {
		return &Error{
			Code: appErr.Code, Message: message, Status: appErr.Status,
			Details: appErr.Details, Internal: err,
		}
	}
	return ErrInternalWithMessage(message, err)
}
