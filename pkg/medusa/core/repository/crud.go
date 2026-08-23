package repository

import (
	"context"
)

// WithCrud is an interface that defines standard CRUD operations for a repository.
// Implement this interface in your repositories to provide basic database operations
// with compile-time type safety.
type WithCrud[T any] interface {
	// Create inserts a new entity into the database.
	Create(ctx context.Context, entity *T) error

	// Get retrieves an entity by its primary key.
	Get(ctx context.Context, id any) (*T, error)

	// FindAll retrieves all entities of type T.
	FindAll(ctx context.Context) ([]T, error)

	// Update saves changes to an existing entity.
	Update(ctx context.Context, entity *T) error

	// Delete removes an entity by its primary key.
	Delete(ctx context.Context, id any) error
}

// CRUDRepository provides generic CRUD operations for any entity type.
// It embeds *Repository to inherit transaction-aware database access.
//
// This is a convenience wrapper that implements common database operations
// so you don't have to write them for every entity type.
//
// Example:
//
//	type User struct {
//	    ID    uint   `gorm:"primaryKey"`
//	    Name  string
//	    Email string
//	}
//
//	userRepo := repository.NewCRUDRepository[User](baseRepo)
//
//	// Create
//	user := &User{Name: "John", Email: "john@example.com"}
//	if err := userRepo.Create(ctx, user); err != nil {
//	    return err
//	}
//
//	// Get
//	user, err := userRepo.Get(ctx, 1)
//
//	// Update
//	user.Name = "Jane"
//	if err := userRepo.Update(ctx, user); err != nil {
//	    return err
//	}
//
//	// Delete
//	if err := userRepo.Delete(ctx, 1); err != nil {
//	    return err
//	}
type CRUDRepository[T any] struct {
	*Repository
}

// NewCRUDRepository creates a new CRUD repository for the specified entity type.
// The entity type T should be a GORM model with proper struct tags.
//
// Example:
//
//	repo := repository.NewCRUDRepository[User](baseRepo)
func NewCRUDRepository[T any](repository *Repository) *CRUDRepository[T] {
	return &CRUDRepository[T]{
		Repository: repository,
	}
}

// Create inserts a new entity into the database.
// The entity's ID will be populated after successful creation if using an auto-increment primary key.
//
// Returns an error if the insert fails (e.g., constraint violation, connection error).
func (r *CRUDRepository[T]) Create(ctx context.Context, entity *T) error {
	return r.DB(ctx).Create(entity).Error
}

// Get retrieves an entity by its primary key.
// Returns the entity if found, or an error if not found or if the query fails.
//
// The id parameter should match the type of the entity's primary key.
//
// Returns gorm.ErrRecordNotFound if the entity doesn't exist.
func (r *CRUDRepository[T]) Get(ctx context.Context, id any) (*T, error) {
	var entity T

	err := r.DB(ctx).First(&entity, id).Error
	if err != nil {
		return nil, err
	}

	return &entity, nil
}

// FindAll retrieves all entities of type T from the database.
// Returns an empty slice if no entities exist.
//
// Warning: This can be memory-intensive for large tables. Consider adding
// pagination for production use.
//
// Returns an error if the query fails.
func (r *CRUDRepository[T]) FindAll(ctx context.Context) ([]T, error) {
	var entities []T

	if err := r.DB(ctx).Find(&entities).Error; err != nil {
		return nil, err
	}

	return entities, nil
}

// Update saves changes to an existing entity.
// This uses GORM's Save method, which updates all fields.
//
// If you need to update specific fields only, use the repository's DB() method directly:
//
//	r.DB(ctx).Model(&entity).Updates(map[string]any{"name": "new name"})
//
// Returns an error if the update fails.
func (r *CRUDRepository[T]) Update(ctx context.Context, entity *T) error {
	if err := r.DB(ctx).Save(entity).Error; err != nil {
		return err
	}

	return nil
}

// Delete removes an entity by its primary key.
// This is a hard delete - the record is permanently removed from the database.
//
// If your entity uses soft deletes (has a DeletedAt field), the record will be
// marked as deleted but not removed. Use Unscoped().Delete() for permanent deletion.
//
// Returns an error if the deletion fails or if the entity doesn't exist.
func (r *CRUDRepository[T]) Delete(ctx context.Context, id any) error {
	var entity T

	result := r.DB(ctx).Delete(&entity, id)
	if result.Error != nil {
		return result.Error
	}

	return nil
}
