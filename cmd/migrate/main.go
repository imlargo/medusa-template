// Command migrate applies the database schema migrations and exits.
package main

import (
	"fmt"
	"os"

	"github.com/imlargo/medusa/internal/config"
	"github.com/imlargo/medusa/internal/database"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
}

func run() (err error) {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := database.NewPostgresDatabase(cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	defer func() {
		sqlDB, dbErr := db.DB()
		if dbErr != nil {
			return
		}
		if closeErr := sqlDB.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close database: %w", closeErr)
		}
	}()

	if err := database.Migrate(db); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	fmt.Println("migrations applied successfully")

	return nil
}
