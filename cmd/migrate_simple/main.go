package main

import (
	"fmt"
	"log"

	"github.com/dbackup/backend-go/internal/config"
	"github.com/dbackup/backend-go/internal/database"
	"github.com/dbackup/backend-go/internal/models"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	// Initialize database
	err = database.Initialize(cfg)
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}

	// Get database instance
	db := database.GetDB()

	// Run AutoMigrate for essential models first (excluding BackupFile due to foreign key issue)
	err = db.AutoMigrate(
		&models.User{},
		&models.Team{},
		&models.TeamMember{},
		&models.DatabaseConnection{},
		&models.DatabaseTable{},
		&models.StorageConfiguration{},
		&models.BackupJob{},
		&models.TablePermission{},
		&models.AuditLog{},
	)
	if err != nil {
		log.Fatalf("Error running migrations: %v", err)
	}

	fmt.Println("✅ Database tables created successfully!")
}