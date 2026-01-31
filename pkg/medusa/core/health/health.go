package health

import (
"context"
"time"
)

type Status string

const (
StatusHealthy   Status = "healthy"
StatusDegraded  Status = "degraded"
StatusUnhealthy Status = "unhealthy"
)

type Check struct {
Name      string        `json:"name"`
Status    Status        `json:"status"`
Timestamp time.Time     `json:"timestamp"`
Duration  time.Duration `json:"duration"`
Error     string        `json:"error,omitempty"`
}

type Checker interface {
Name() string
Check(ctx context.Context) Check
}

type Service struct {
checkers []Checker
}

func NewService(checkers ...Checker) *Service {
return &Service{checkers: checkers}
}

func (s *Service) CheckAll(ctx context.Context) map[string]Check {
results := make(map[string]Check)

for _, checker := range s.checkers {
check := checker.Check(ctx)
results[check.Name] = check
}

return results
}

func (s *Service) IsHealthy(ctx context.Context) bool {
checks := s.CheckAll(ctx)

for _, check := range checks {
if check.Status == StatusUnhealthy {
return false
}
}

return true
}
