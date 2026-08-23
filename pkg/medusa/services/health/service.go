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
// Note: This should only be called during initialization before any concurrent access.
func (s *Service) RegisterChecker(checker Checker) {
	s.checkers = append(s.checkers, checker)
}

// Check runs every registered check and reports the combined status.
//
// Checks run concurrently: a readiness probe over N dependencies should cost the
// slowest one, not the sum of all of them. The configured timeout bounds the
// whole set, and results keep registration order regardless of completion order
// so the response is stable across calls.
func (s *Service) Check(ctx context.Context) HealthStatus {
	checkers := s.checkers

	checkCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	results := make([]CheckResult, len(checkers))

	var wg sync.WaitGroup
	for i, checker := range checkers {
		wg.Go(func() {
			result := CheckResult{
				Name:   checker.Name(),
				Status: StatusHealthy,
			}

			if err := checker.Check(checkCtx); err != nil {
				result.Status = StatusUnhealthy
				result.Message = err.Error()
			}

			// Each goroutine owns exactly one slot, so no lock is needed.
			results[i] = result
		})
	}
	wg.Wait()

	status := StatusHealthy
	for _, result := range results {
		if result.Status != StatusHealthy {
			status = StatusUnhealthy
			break
		}
	}

	return HealthStatus{
		Status: status,
		Checks: results,
	}
}

// CheckSimple reports whether every dependency is healthy, without the detail.
func (s *Service) CheckSimple(ctx context.Context) bool {
	return s.Check(ctx).Status == StatusHealthy
}
