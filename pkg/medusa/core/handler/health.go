package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa/services/health"
)

// HealthHandler handles health check endpoints.
type HealthHandler struct {
	*Handler
	service *health.Service
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(handler *Handler, service *health.Service) *HealthHandler {
	return &HealthHandler{
		Handler: handler,
		service: service,
	}
}

// Health returns a simple liveness check (always returns healthy if process is running).
// This is suitable for load balancers that only need to know if the process is alive.
// For detailed dependency health checks, use the Ready endpoint.
func (h *HealthHandler) Health(c *gin.Context) {
	// Extract request ID from context to include in response
	requestID := ""
	if id, exists := c.Get("request_id"); exists {
		if reqID, ok := id.(string); ok {
			requestID = reqID
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status":     "healthy",
		"request_id": requestID,
	})
}

// Ready checks all dependencies and returns detailed status.
// Returns 503 with Retry-After header if dependencies are unhealthy.
func (h *HealthHandler) Ready(c *gin.Context) {
	status := h.service.Check(c.Request.Context())

	// Extract request ID from context to include in response
	requestID := ""
	if id, exists := c.Get("request_id"); exists {
		if reqID, ok := id.(string); ok {
			requestID = reqID
		}
	}

	// Create response with request_id
	response := gin.H{
		"status":     status.Status,
		"checks":     status.Checks,
		"request_id": requestID,
	}

	if status.Status == "unhealthy" {
		// Suggest retry after 10 seconds for unhealthy dependencies
		c.Header("Retry-After", "10")
		c.JSON(http.StatusServiceUnavailable, response)
		return
	}

	c.JSON(http.StatusOK, response)
}
