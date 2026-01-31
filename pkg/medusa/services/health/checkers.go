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

// StorageChecker checks storage connectivity.
type StorageChecker struct {
	checkFunc func(ctx context.Context) error
}

// NewStorageChecker creates a new storage health checker.
func NewStorageChecker(checkFunc func(ctx context.Context) error) *StorageChecker {
	return &StorageChecker{checkFunc: checkFunc}
}

// Name returns the checker name.
func (c *StorageChecker) Name() string {
	return "storage"
}

// Check performs the storage health check.
func (c *StorageChecker) Check(ctx context.Context) error {
	return c.checkFunc(ctx)
}

// GenericChecker is a generic health checker.
type GenericChecker struct {
	name      string
	checkFunc func(ctx context.Context) error
}

// NewGenericChecker creates a new generic health checker.
func NewGenericChecker(name string, checkFunc func(ctx context.Context) error) *GenericChecker {
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
