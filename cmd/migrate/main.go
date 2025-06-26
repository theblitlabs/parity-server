package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	// Build database URL from environment variables
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("DATABASE_USERNAME"),
		os.Getenv("DATABASE_PASSWORD"),
		os.Getenv("DATABASE_HOST"),
		os.Getenv("DATABASE_PORT"),
		os.Getenv("DATABASE_DATABASE_NAME"),
	)

	log.Printf("Connecting to database: %s", dbURL)

	// Connect to database
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Connected to database successfully")

	// Run migrations
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
			log.Fatalf("Error migrating %T: %v", model, err)
		}
		log.Printf("Successfully migrated table for model: %T", model)
	}

	log.Println("All database migrations completed successfully!")

	// Verify tables were created
	log.Println("Verifying tables...")

	var tables []string
	db.Raw("SELECT tablename FROM pg_tables WHERE schemaname = 'public'").Scan(&tables)

	log.Printf("Created tables: %v", tables)
}
