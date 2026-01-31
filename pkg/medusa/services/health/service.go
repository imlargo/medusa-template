package health

import (
	"context"
	"sync"
	"time"
)

// Service manages health checks for system dependencies.
type Service struct {
	checkers []Checker
	timeout  time.Duration
	mu       sync.RWMutex
}

// NewService creates a new health check service.
func NewService(timeout time.Duration) *Service {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &Service{
		checkers: make([]Checker, 0),
		timeout:  timeout,
	}
}

// RegisterChecker adds a health checker to the service.
func (s *Service) RegisterChecker(checker Checker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkers = append(s.checkers, checker)
}

// Check performs all health checks and returns the overall status.
func (s *Service) Check(ctx context.Context) HealthStatus {
	s.mu.RLock()
	checkers := make([]Checker, len(s.checkers))
	copy(checkers, s.checkers)
	s.mu.RUnlock()

	// Create context with timeout
	checkCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	results := make([]CheckResult, 0, len(checkers))
	allHealthy := true

	// Execute checks synchronously for easier debugging
	for _, checker := range checkers {
		result := CheckResult{
			Name:   checker.Name(),
			Status: "healthy",
		}

		if err := checker.Check(checkCtx); err != nil {
			result.Status = "unhealthy"
			result.Message = err.Error()
			allHealthy = false
		}

		results = append(results, result)
	}

	status := "healthy"
	if !allHealthy {
		status = "unhealthy"
	}

	return HealthStatus{
		Status: status,
		Checks: results,
	}
}

// CheckSimple performs a simple health check without detailed results.
func (s *Service) CheckSimple(ctx context.Context) bool {
	status := s.Check(ctx)
	return status.Status == "healthy"
}
