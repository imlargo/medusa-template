package handler

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa/core/health"
	"github.com/imlargo/medusa/pkg/medusa/core/responses"
)

type HealthHandler struct {
	*Handler
	healthService *health.Service
}

func NewHealthHandler(handler *Handler, healthService *health.Service) *HealthHandler {
	return &HealthHandler{
		Handler:       handler,
		healthService: healthService,
	}
}

func (h *HealthHandler) Health(c *gin.Context) {
	// If no health service, return simple OK
	if h.healthService == nil {
		responses.SuccessOK(c, gin.H{"status": "ok"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	checks := h.healthService.CheckAll(ctx)

	allHealthy := true
	for _, check := range checks {
		if check.Status == health.StatusUnhealthy {
			allHealthy = false
			break
		}
	}

	status := 200
	if !allHealthy {
		status = 503
	}

	c.JSON(status, gin.H{
		"status": "healthy",
		"checks": checks,
	})
}

func (h *HealthHandler) Liveness(c *gin.Context) {
	responses.SuccessOK(c, gin.H{"status": "alive"})
}

func (h *HealthHandler) Readiness(c *gin.Context) {
	// If no health service, always ready
	if h.healthService == nil {
		responses.SuccessOK(c, gin.H{"status": "ready"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if h.healthService.IsHealthy(ctx) {
		responses.SuccessOK(c, gin.H{"status": "ready"})
	} else {
		c.JSON(503, gin.H{"status": "not ready"})
	}
}
