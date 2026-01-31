package health

import (
"context"
"time"

"github.com/redis/go-redis/v9"
"gorm.io/gorm"
)

type DatabaseChecker struct {
db *gorm.DB
}

func NewDatabaseChecker(db *gorm.DB) *DatabaseChecker {
return &DatabaseChecker{db: db}
}

func (d *DatabaseChecker) Name() string {
return "database"
}

func (d *DatabaseChecker) Check(ctx context.Context) Check {
start := time.Now()
check := Check{
Name:      d.Name(),
Timestamp: start,
}

sqlDB, err := d.db.DB()
if err != nil {
check.Status = StatusUnhealthy
check.Error = err.Error()
check.Duration = time.Since(start)
return check
}

if err := sqlDB.PingContext(ctx); err != nil {
check.Status = StatusUnhealthy
check.Error = err.Error()
check.Duration = time.Since(start)
return check
}

check.Status = StatusHealthy
check.Duration = time.Since(start)
return check
}

type RedisChecker struct {
client *redis.Client
}

func NewRedisChecker(client *redis.Client) *RedisChecker {
return &RedisChecker{client: client}
}

func (r *RedisChecker) Name() string {
return "redis"
}

func (r *RedisChecker) Check(ctx context.Context) Check {
start := time.Now()
check := Check{
Name:      r.Name(),
Timestamp: start,
}

if err := r.client.Ping(ctx).Err(); err != nil {
check.Status = StatusUnhealthy
check.Error = err.Error()
check.Duration = time.Since(start)
return check
}

check.Status = StatusHealthy
check.Duration = time.Since(start)
return check
}
