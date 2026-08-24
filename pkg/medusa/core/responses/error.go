package responses

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorCode is a machine-readable error code clients can branch on.
// Codes are part of the API contract and must stay stable across versions.
type ErrorCode string

const (
	// ErrCodeBadRequest: the request was malformed or carried invalid parameters (400).
	ErrCodeBadRequest ErrorCode = "BAD_REQUEST"

	// ErrCodeValidation: the body parsed but failed validation (400).
	// Details carries a field-to-message map.
	ErrCodeValidation ErrorCode = "VALIDATION_FAILED"

	// ErrCodeUnauthorized: authentication is required, missing or invalid (401).
	ErrCodeUnauthorized ErrorCode = "UNAUTHORIZED"

	// ErrCodeForbidden: authenticated but not allowed to touch this resource (403).
	ErrCodeForbidden ErrorCode = "FORBIDDEN"

	// ErrCodeNotFound: the requested resource does not exist (404).
	ErrCodeNotFound ErrorCode = "NOT_FOUND"

	// ErrCodeConflict: the request conflicts with the resource's current state (409).
	ErrCodeConflict ErrorCode = "CONFLICT"

	// ErrCodePayloadTooLarge: the request body exceeded the configured cap (413).
	ErrCodePayloadTooLarge ErrorCode = "PAYLOAD_TOO_LARGE"

	// ErrCodeTooManyRequests: the client exceeded its rate limit (429).
	ErrCodeTooManyRequests ErrorCode = "TOO_MANY_REQUESTS"

	// ErrCodeInternalServer: an unexpected server-side failure (500).
	// The underlying cause is logged, never sent to the client.
	ErrCodeInternalServer ErrorCode = "INTERNAL_SERVER_ERROR"

	// ErrCodeServiceUnavailable: a dependency is down or the app is draining (503).
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
)

// ErrorResponse is the JSON body of every failed request.
type ErrorResponse struct {
	Status    int       `json:"status"`               // HTTP status code
	Code      ErrorCode `json:"code"`                 // Machine-readable code
	Error     string    `json:"error"`                // Human-readable message
	Details   any       `json:"details,omitempty"`    // Field errors or extra context
	RequestID string    `json:"request_id,omitempty"` // Correlation ID, when available
}

// Error is an application error carrying everything needed to render an HTTP
// response: the status, a stable code, a client-safe message and optional
// details. It is the only error type handlers need to construct.
//
// Internal holds the underlying cause. It is never serialized — it reaches the
// access log instead, so a 500 can be diagnosed without leaking internals to the
// caller. Use the constructors below rather than building one by hand.
type Error struct {
	Code     ErrorCode `json:"code"`
	Message  string    `json:"message"`
	Status   int       `json:"-"`
	Details  any       `json:"details,omitempty"`
	Internal error     `json:"-"`
}

// Error implements the error interface. It includes the internal cause, so this
// string is safe for logs but not for clients — clients get Message.
func (e *Error) Error() string {
	if e.Internal != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Internal)
	}
	return e.Message
}

// Unwrap exposes the internal cause to errors.Is and errors.As.
func (e *Error) Unwrap() error { return e.Internal }

// WithDetails attaches details and returns the same error for chaining.
func (e *Error) WithDetails(details any) *Error {
	e.Details = details
	return e
}

// BadRequest reports a malformed request (400).
func BadRequest(message string) *Error {
	return &Error{Code: ErrCodeBadRequest, Message: message, Status: http.StatusBadRequest}
}

// Validation reports a body that parsed but failed validation (400).
// details is typically a field-to-message map.
func Validation(message string, details any) *Error {
	return &Error{Code: ErrCodeValidation, Message: message, Status: http.StatusBadRequest, Details: details}
}

// Unauthorized reports missing or invalid authentication (401).
func Unauthorized(message string) *Error {
	return &Error{Code: ErrCodeUnauthorized, Message: message, Status: http.StatusUnauthorized}
}

// Forbidden reports an authenticated caller without permission (403).
func Forbidden(message string) *Error {
	return &Error{Code: ErrCodeForbidden, Message: message, Status: http.StatusForbidden}
}

// NotFound reports a missing resource (404). resource names what was looked up,
// for example "user".
func NotFound(resource string) *Error {
	return &Error{Code: ErrCodeNotFound, Message: resource + " not found", Status: http.StatusNotFound}
}

// Conflict reports a clash with the resource's current state (409).
func Conflict(message string) *Error {
	return &Error{Code: ErrCodeConflict, Message: message, Status: http.StatusConflict}
}

// PayloadTooLarge reports a request body over the limit (413).
func PayloadTooLarge(maxBytes int64) *Error {
	return &Error{
		Code:    ErrCodePayloadTooLarge,
		Message: "request body is too large",
		Status:  http.StatusRequestEntityTooLarge,
		Details: map[string]int64{"max_bytes": maxBytes},
	}
}

// TooManyRequests reports an exhausted rate limit (429), telling the client how
// many seconds to wait.
func TooManyRequests(retryAfterSeconds int) *Error {
	return &Error{
		Code:    ErrCodeTooManyRequests,
		Message: "rate limit exceeded",
		Status:  http.StatusTooManyRequests,
		Details: map[string]int{"retry_after_seconds": retryAfterSeconds},
	}
}

// Internal reports an unexpected failure (500). The cause is logged; the client
// only ever sees a generic message.
func Internal(cause error) *Error {
	return &Error{
		Code:     ErrCodeInternalServer,
		Message:  "an internal error occurred",
		Status:   http.StatusInternalServerError,
		Internal: cause,
	}
}

// InternalWithMessage is Internal with a client-safe message of your own.
// Never pass raw error text: it is the most common way internals leak.
func InternalWithMessage(message string, cause error) *Error {
	return &Error{
		Code:     ErrCodeInternalServer,
		Message:  message,
		Status:   http.StatusInternalServerError,
		Internal: cause,
	}
}

// ServiceUnavailable reports a dependency being down or the app draining (503).
func ServiceUnavailable(message string) *Error {
	return &Error{Code: ErrCodeServiceUnavailable, Message: message, Status: http.StatusServiceUnavailable}
}

// From converts any error into an *Error.
//
// An *Error anywhere in the chain is returned as-is, so a wrapped domain error
// keeps its status and code. Anything else becomes a 500 with the original error
// as the internal cause, which is the safe default: an error nobody classified
// is not something to describe to a client.
func From(err error) *Error {
	if err == nil {
		return nil
	}

	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}

	return Internal(err)
}

// Wrap adds context to an error while preserving its classification. A wrapped
// *Error keeps its status, code and details; anything else becomes a 500.
func Wrap(err error, message string) *Error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return &Error{
			Code:     appErr.Code,
			Message:  message,
			Status:   appErr.Status,
			Details:  appErr.Details,
			Internal: err,
		}
	}

	return InternalWithMessage(message, err)
}

// WriteError renders err as the response, deriving the status from the error
// itself. Any error is accepted; unclassified ones become a 500.
//
// When the error carries an internal cause, it is also handed to Gin's error
// list so the access-log middleware records why the request failed. That is the
// only place the cause appears: it is never serialized to the client.
func WriteError(c *gin.Context, err error) {
	appErr := From(err)
	if appErr == nil {
		return
	}

	if appErr.Internal != nil {
		// Recorded for the access log, not for the response body.
		_ = c.Error(appErr.Internal)
	}

	c.JSON(appErr.Status, ErrorResponse{
		Status:    appErr.Status,
		Code:      appErr.Code,
		Error:     appErr.Message,
		Details:   appErr.Details,
		RequestID: requestID(c),
	})
}

// AbortWithError renders err and stops the middleware chain. Middleware should
// use this; a handler that simply returns its error does not need it.
func AbortWithError(c *gin.Context, err error) {
	WriteError(c, err)
	c.Abort()
}
