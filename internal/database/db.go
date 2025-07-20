package database

import (
	"context"
	"fmt"
	"time"

	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Connect(ctx context.Context, dbURL string) (*gorm.DB, error) {
	log := gologger.WithComponent("database")

	// Disable detailed GORM logging
	newLogger := logger.New(
		nil, // Use nil to completely disable output
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

	// Run migrations silently
	modelsList := []interface{}{
		&models.Task{},
		&models.TaskResult{},
		&models.Runner{},
		&models.RunnerReputation{},
		&models.ReputationEvent{},
		&models.PromptRequest{},
		&models.ModelCapability{},
		&models.BillingMetric{},
		&models.FederatedLearningSession{},
		&models.FederatedLearningRound{},
		&models.FLRoundParticipant{},
		&models.ParticipantQualityMetrics{},
		&models.SessionQualityMetrics{},
		&models.NetworkQualityMetrics{},
		&models.QualityAlert{},
		&models.QualitySLA{},
		&models.QualityReport{},
	}

	for _, model := range modelsList {
		if err := db.AutoMigrate(model); err != nil {
			log.Error().Err(err).Msgf("error migrating %T", model)
			return nil, fmt.Errorf("error migrating %T: %w", model, err)
		}
	}

	// Verify tables exist silently
	for _, model := range modelsList {
		if !db.Migrator().HasTable(model) {
			log.Error().Msgf("table for model %T was not created", model)
			return nil, fmt.Errorf("table for model %T was not created", model)
		}
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

	log.Info().Msg("Database connected successfully")
	return db, nil
}
