package handler

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthChecker is a function that checks a dependency's health.
type HealthChecker func(ctx context.Context) error

// HealthHandler handles health check endpoints.
type HealthHandler struct {
	*Handler
	checkers map[string]HealthChecker
	mu       sync.RWMutex
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(handler *Handler) *HealthHandler {
	return &HealthHandler{
		Handler:  handler,
		checkers: make(map[string]HealthChecker),
	}
}

// RegisterChecker adds a health checker for a dependency.
func (h *HealthHandler) RegisterChecker(name string, checker HealthChecker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkers[name] = checker
}

// HealthStatus represents the health check response.
type HealthStatus struct {
	Status string                 `json:"status"`
	Checks map[string]CheckResult `json:"checks,omitempty"`
}

// CheckResult represents a single health check result.
type CheckResult struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// Health returns a simple health status (for load balancers).
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

// Ready checks all dependencies and returns detailed status.
func (h *HealthHandler) Ready(c *gin.Context) {
	h.mu.RLock()
	checkers := make(map[string]HealthChecker, len(h.checkers))
	for k, v := range h.checkers {
		checkers[k] = v
	}
	h.mu.RUnlock()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	status := HealthStatus{
		Status: "healthy",
		Checks: make(map[string]CheckResult),
	}

	allHealthy := true
	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, checker := range checkers {
		wg.Add(1)
		go func(name string, checker HealthChecker) {
			defer wg.Done()
			result := CheckResult{Status: "healthy"}
			if err := checker(ctx); err != nil {
				result.Status = "unhealthy"
				result.Message = err.Error()
				mu.Lock()
				allHealthy = false
				mu.Unlock()
			}
			mu.Lock()
			status.Checks[name] = result
			mu.Unlock()
		}(name, checker)
	}

	wg.Wait()

	if !allHealthy {
		status.Status = "unhealthy"
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}

	c.JSON(http.StatusOK, status)
}
