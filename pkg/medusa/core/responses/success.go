// Package responses provides standardized HTTP response helpers for Gin applications.
//
// This package defines consistent response structures for both success and error cases,
// making API responses predictable and easy to consume. All responses follow a common format
// with status codes, messages, and optional data or error details.
//
// Success Response Format:
//
//	{
//	    "status": 200,
//	    "success": true,
//	    "message": "ok",
//	    "data": {...}
//	}
//
// Example usage:
//
//	func GetUserHandler(c *gin.Context) {
//	    user := User{ID: 1, Name: "John"}
//	    responses.SuccessOK(c, user)
//	}
//
//	func CreateUserHandler(c *gin.Context) {
//	    user := User{ID: 2, Name: "Jane"}
//	    responses.SuccessCreated(c, user)
//	}
package responses

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SuccessResponse represents a standardized successful HTTP response.
// It includes the HTTP status code, a success flag, an optional message,
// optional data payload, and request ID for tracing.
type SuccessResponse struct {
	Status    int         `json:"status"`               // HTTP status code (e.g., 200, 201)
	Success   bool        `json:"success"`              // Always true for success responses
	Message   string      `json:"message,omitempty"`    // Human-readable success message
	Data      interface{} `json:"data,omitempty"`       // Response payload, can be any JSON-serializable data
	RequestID string      `json:"request_id,omitempty"` // Request ID for tracing (if available)
}

// WriteSuccessResponse writes a successful JSON response with the given status, message, and data.
// This is a low-level function; prefer using the helper functions like SuccessOK or SuccessCreated
// for common use cases.
//
// Parameters:
//   - c: The Gin context
//   - status: HTTP status code (e.g., http.StatusOK)
//   - message: Human-readable success message
//   - data: Response payload (can be nil)
func WriteSuccessResponse(c *gin.Context, status int, message string, data interface{}) {
	// Extract request ID if available
	requestID := ""
	if id, exists := c.Get("medusa_request_id"); exists {
		if strID, ok := id.(string); ok {
			requestID = strID
		}
	}

	c.JSON(status, SuccessResponse{
		Status:    status,
		Success:   true,
		Message:   message,
		Data:      data,
		RequestID: requestID,
	})
}

// SuccessOK writes a 200 OK response with the provided data.
// Use this for successful GET requests or operations that completed successfully.
//
// Example:
//
//	responses.SuccessOK(c, gin.H{"users": users})
func SuccessOK(c *gin.Context, data interface{}) {
	WriteSuccessResponse(c, http.StatusOK, "ok", data)
}

// SuccessCreated writes a 201 Created response with the provided data.
// Use this for successful POST requests that created a new resource.
//
// Example:
//
//	responses.SuccessCreated(c, gin.H{"user": newUser})
func SuccessCreated(c *gin.Context, data interface{}) {
	WriteSuccessResponse(c, http.StatusCreated, "created", data)
}

// SuccessUpdated writes a 200 OK response for successful update operations.
// Use this for successful PUT or PATCH requests.
//
// Example:
//
//	responses.SuccessUpdated(c, gin.H{"user": updatedUser})
func SuccessUpdated(c *gin.Context, data interface{}) {
	WriteSuccessResponse(c, http.StatusOK, "updated", data)
}

// SuccessDeleted writes a 200 OK response for successful delete operations.
// No data payload is included. Use this for successful DELETE requests.
//
// Example:
//
//	responses.SuccessDeleted(c)
func SuccessDeleted(c *gin.Context) {
	WriteSuccessResponse(c, http.StatusOK, "deleted", nil)
}
