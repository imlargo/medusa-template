// pkg/medusa/errors.go
package medusa

import (
	"fmt"
	"net/http"
)

// Error represents an application error with HTTP context.
type Error struct {
	Code     string      `json:"code"`
	Message  string      `json:"message"`
	Status   int         `json:"-"`
	Details  interface{} `json:"details,omitempty"`
	Internal error       `json:"-"`
}

func (e *Error) Error() string {
	if e.Internal != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Internal)
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Internal
}

// Error constructors

func ErrBadRequest(message string) *Error {
	return &Error{Code: "BAD_REQUEST", Message: message, Status: http.StatusBadRequest}
}

func ErrValidation(message string, details interface{}) *Error {
	return &Error{Code: "BIND_JSON", Message: message, Status: http.StatusBadRequest, Details: details}
}

func ErrUnauthorized(message string) *Error {
	return &Error{Code: "UNAUTHORIZED", Message: message, Status: http.StatusUnauthorized}
}

func ErrForbidden(message string) *Error {
	return &Error{Code: "FORBIDDEN", Message: message, Status: http.StatusForbidden}
}

func ErrNotFound(resource string) *Error {
	return &Error{Code: "NOT_FOUND", Message: fmt.Sprintf("%s not found", resource), Status: http.StatusNotFound}
}

func ErrConflict(message string) *Error {
	return &Error{Code: "CONFLICT", Message: message, Status: http.StatusConflict}
}

func ErrInternal(err error) *Error {
	return &Error{Code: "INTERNAL_SERVER_ERROR", Message: "an internal error occurred", Status: http.StatusInternalServerError, Internal: err}
}

func ErrInternalWithMessage(message string, err error) *Error {
	return &Error{Code: "INTERNAL_SERVER_ERROR", Message: message, Status: http.StatusInternalServerError, Internal: err}
}

func ErrTooManyRequests(retryAfter int) *Error {
	return &Error{
		Code: "TOO_MANY_REQUESTS", Message: "rate limit exceeded", Status: http.StatusTooManyRequests,
		Details: map[string]int{"retry_after_seconds": retryAfter},
	}
}

func ErrServiceUnavailable(message string) *Error {
	return &Error{Code: "SERVICE_UNAVAILABLE", Message: message, Status: http.StatusServiceUnavailable}
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
