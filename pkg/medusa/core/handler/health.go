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

// Health returns a simple health status (for load balancers).
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

// Ready checks all dependencies and returns detailed status.
func (h *HealthHandler) Ready(c *gin.Context) {
	status := h.service.Check(c.Request.Context())

	if status.Status == "unhealthy" {
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}

	c.JSON(http.StatusOK, status)
}
