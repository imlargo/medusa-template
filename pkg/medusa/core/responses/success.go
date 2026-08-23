package responses

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SuccessResponse is the JSON body of every successful request.
type SuccessResponse struct {
	Status    int    `json:"status"`               // HTTP status code
	Success   bool   `json:"success"`              // Always true
	Message   string `json:"message,omitempty"`    // Human-readable message
	Data      any    `json:"data,omitempty"`       // Payload
	RequestID string `json:"request_id,omitempty"` // Correlation ID, when available
}

// Default messages for the common success cases.
const (
	MessageOK      = "ok"
	MessageCreated = "created"
	MessageUpdated = "updated"
	MessageDeleted = "deleted"
)

// WriteSuccess renders a successful response. It is the only success writer;
// the status and message spell out the intent at the call site, and the medusa
// Context helpers (OK, Created, Updated, Deleted) wrap the usual combinations.
func WriteSuccess(c *gin.Context, status int, message string, data any) {
	c.JSON(status, SuccessResponse{
		Status:    status,
		Success:   true,
		Message:   message,
		Data:      data,
		RequestID: requestID(c),
	})
}

// WriteNoContent replies 204 with an empty body, for operations with nothing to
// report back.
func WriteNoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
