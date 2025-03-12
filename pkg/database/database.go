package database

import (
	"context"
	"fmt"

	"github.com/theblitlabs/parity-server/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Connect establishes a connection to the database
func Connect(ctx context.Context, dbURL string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("error opening database: %w", err)
	}
	if err := db.AutoMigrate(&models.Task{}, &models.TaskResult{}, &models.Runner{}); err != nil {
		return nil, fmt.Errorf("error migrating database: %w", err)
	}

	return db, nil
}
