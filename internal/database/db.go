package database

import (
	"context"
	"fmt"

	"github.com/theblitlabs/parity-server/internal/core/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(ctx context.Context, dbURL string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("error opening database: %w", err)
	}
	if err := db.AutoMigrate(&models.Task{}, &models.TaskResult{}, &models.Runner{}); err != nil {
		return nil, fmt.Errorf("error migrating database: %w", err)
	}

	var count int64
	if err := db.Model(&models.Runner{}).Where("last_heartbeat IS NULL").Count(&count).Error; err != nil {
		return nil, fmt.Errorf("error checking for null LastHeartbeat: %w", err)
	}

	if count > 0 {
		if err := db.Model(&models.Runner{}).Where("last_heartbeat IS NULL").Update("last_heartbeat", gorm.Expr("NOW()")).Error; err != nil {
			return nil, fmt.Errorf("error updating null LastHeartbeat values: %w", err)
		}
	}

	return db, nil
}
