package health

import (
	"context"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// DatabaseChecker checks database connectivity.
type DatabaseChecker struct {
	db *gorm.DB
}

// NewDatabaseChecker creates a new database health checker.
func NewDatabaseChecker(db *gorm.DB) *DatabaseChecker {
	if db == nil {
		panic("health: NewDatabaseChecker called with nil *gorm.DB")
	}
	return &DatabaseChecker{db: db}
}

// Name returns the checker name.
func (c *DatabaseChecker) Name() string {
	return "database"
}

// Check performs the database health check.
func (c *DatabaseChecker) Check(ctx context.Context) error {
	sqlDB, err := c.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// RedisChecker checks Redis connectivity.
type RedisChecker struct {
	client *redis.Client
}

// NewRedisChecker creates a new Redis health checker.
func NewRedisChecker(client *redis.Client) *RedisChecker {
	if client == nil {
		panic("health: NewRedisChecker called with nil *redis.Client")
	}
	return &RedisChecker{client: client}
}

// Name returns the checker name.
func (c *RedisChecker) Name() string {
	return "redis"
}

// Check performs the Redis health check.
func (c *RedisChecker) Check(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// GenericChecker is a generic health checker.
type GenericChecker struct {
	name      string
	checkFunc func(ctx context.Context) error
}

// NewGenericChecker creates a new generic health checker.
func NewGenericChecker(name string, checkFunc func(ctx context.Context) error) *GenericChecker {
	if name == "" {
		panic("health: NewGenericChecker called with empty name")
	}
	if checkFunc == nil {
		panic("health: NewGenericChecker called with nil checkFunc")
	}
	return &GenericChecker{
		name:      name,
		checkFunc: checkFunc,
	}
}

// Name returns the checker name.
func (c *GenericChecker) Name() string {
	return c.name
}

// Check performs the health check.
func (c *GenericChecker) Check(ctx context.Context) error {
	return c.checkFunc(ctx)
}
