// Package health provides health check functionality for system dependencies.
package health

import "context"

// Checker defines the interface for health checks.
type Checker interface {
	// Check performs the health check and returns an error if unhealthy.
	Check(ctx context.Context) error
	// Name returns the name of the checker.
	Name() string
}

// Status values reported for a dependency and for the app as a whole. They are
// part of the /ready response contract, so they live here as constants rather
// than as literals repeated at each use.
const (
	StatusHealthy   = "healthy"
	StatusUnhealthy = "unhealthy"
)

// CheckResult represents the result of a health check.
type CheckResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// HealthStatus represents the overall health status.
type HealthStatus struct {
	Status string        `json:"status"`
	Checks []CheckResult `json:"checks,omitempty"`
}
