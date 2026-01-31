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

	results := make([]CheckResult, len(checkers))
	allHealthy := true

	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, checker := range checkers {
		wg.Add(1)
		go func(index int, chk Checker) {
			defer wg.Done()
			
			result := CheckResult{
				Name:   chk.Name(),
				Status: "healthy",
			}
			
			if err := chk.Check(checkCtx); err != nil {
				result.Status = "unhealthy"
				result.Message = err.Error()
				mu.Lock()
				allHealthy = false
				mu.Unlock()
			}
			
			mu.Lock()
			results[index] = result
			mu.Unlock()
		}(i, checker)
	}

	wg.Wait()

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
