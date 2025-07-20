package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	log := gologger.WithComponent("migrate")

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Warn().Err(err).Msg("Error loading .env file")
	}

	// Build database URL from environment variables
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("DATABASE_USERNAME"),
		os.Getenv("DATABASE_PASSWORD"),
		os.Getenv("DATABASE_HOST"),
		os.Getenv("DATABASE_PORT"),
		os.Getenv("DATABASE_DATABASE_NAME"),
	)

	log.Info().Str("db_url", dbURL).Msg("Connecting to database")

	// Connect to database
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}

	log.Info().Msg("Connected to database successfully")

	// Run migrations silently
	modelsList := []interface{}{
		&models.Task{},
		&models.TaskResult{},
		&models.Runner{},
		&models.PromptRequest{},
		&models.ModelCapability{},
		&models.BillingMetric{},
	}

	for _, model := range modelsList {
		if err := db.AutoMigrate(model); err != nil {
			log.Fatal().Err(err).Msgf("Error migrating %T", model)
		}
	}

	log.Info().Msg("All database migrations completed successfully")

	// Verify tables were created
	var tables []string
	db.Raw("SELECT tablename FROM pg_tables WHERE schemaname = 'public'").Scan(&tables)

	log.Info().Strs("tables", tables).Msg("Migration verification completed")
}
