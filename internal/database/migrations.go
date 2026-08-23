package database

import (
	"fmt"

	"github.com/imlargo/medusa/internal/models"
	"gorm.io/gorm"
)

// Migrate brings the schema up to date with the models below.
//
// Add every new model here: AutoMigrate creates and alters tables, but it never
// drops a column or a table, so removals still need a hand-written migration.
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&models.User{},
	); err != nil {
		return fmt.Errorf("auto-migrate schema: %w", err)
	}

	return nil
}
