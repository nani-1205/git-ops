package repository

import (
	"log"

	"gitlab-code-scan/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB(dsn string) {
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Migrate the schema
	err = DB.AutoMigrate(&models.User{}, &models.ScanHistory{}, &models.ScanJob{})
	if err != nil {
		log.Fatalf("Failed to migrate database schema: %v", err)
	}
	
	log.Println("Database connection established and schema migrated.")
}
