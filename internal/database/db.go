package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/theblitlabs/parity-server/internal/core/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Connect(ctx context.Context, dbURL string) (*gorm.DB, error) {
	// Disable detailed GORM logging
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second,   // Slow SQL threshold
			LogLevel:                  logger.Silent, // Disable SQL logging
			IgnoreRecordNotFoundError: true,          // Ignore not found errors
			Colorful:                  false,         // Disable color
		},
	)

	// Configure GORM with minimal logging
	config := &gorm.Config{
		Logger:      newLogger,
		DryRun:      false,
		PrepareStmt: true,
	}

	db, err := gorm.Open(postgres.Open(dbURL), config)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %w", err)
	}

	// Run migrations with minimal logging
	log.Println("Starting database migrations...")

	modelsList := []interface{}{
		&models.Task{},
		&models.TaskResult{},
		&models.Runner{},
		&models.PromptRequest{},
		&models.ModelCapability{},
		&models.BillingMetric{},
	}

	for _, model := range modelsList {
		log.Printf("Migrating table for model: %T", model)
		if err := db.AutoMigrate(model); err != nil {
			return nil, fmt.Errorf("error migrating %T: %w", model, err)
		}
		log.Printf("Successfully migrated table for model: %T", model)
	}

	log.Println("All database migrations completed successfully")

	// Verify tables exist and have correct schema
	for _, model := range modelsList {
		if !db.Migrator().HasTable(model) {
			return nil, fmt.Errorf("table for model %T was not created", model)
		}
		log.Printf("Verified table exists for model: %T", model)
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
