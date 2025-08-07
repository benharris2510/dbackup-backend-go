package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"

	"github.com/dbackup/backend-go/internal/config"
	"github.com/dbackup/backend-go/internal/database"
	"github.com/dbackup/backend-go/internal/models"
)

var (
	configFile    = flag.String("config", "config.yaml", "Configuration file path")
	migrationsDir = flag.String("migrations", "migrations", "Migrations directory path")
	targetVersion = flag.String("version", "", "Target migration version")
	migrationName = flag.String("name", "", "Migration name for create command")
)

func main() {
	flag.Parse()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[len(os.Args)-1]

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Printf("Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize database
	db, err := database.Initialize(cfg)
	if err != nil {
		fmt.Printf("Error initializing database: %v\n", err)
		os.Exit(1)
	}

	// Get absolute path for migrations directory
	absDir, err := filepath.Abs(*migrationsDir)
	if err != nil {
		fmt.Printf("Error getting absolute path for migrations directory: %v\n", err)
		os.Exit(1)
	}

	// Initialize migration system
	migrationSystem := database.NewMigrationSystem(db, absDir)
	if err := migrationSystem.Initialize(); err != nil {
		fmt.Printf("Error initializing migration system: %v\n", err)
		os.Exit(1)
	}

	// Load migrations from directory
	if err := migrationSystem.LoadMigrationsFromDir(); err != nil {
		fmt.Printf("Error loading migrations: %v\n", err)
		os.Exit(1)
	}

	// Register built-in model migrations
	if err := registerBuiltinMigrations(migrationSystem); err != nil {
		fmt.Printf("Error registering builtin migrations: %v\n", err)
		os.Exit(1)
	}

	// Execute command
	switch command {
	case "up":
		if err := migrationSystem.Up(*targetVersion); err != nil {
			fmt.Printf("Error running migrations up: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Migrations completed successfully")

	case "down":
		if err := migrationSystem.Down(*targetVersion); err != nil {
			fmt.Printf("Error running migrations down: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Rollback completed successfully")

	case "status":
		migrations, err := migrationSystem.Status()
		if err != nil {
			fmt.Printf("Error getting migration status: %v\n", err)
			os.Exit(1)
		}
		printMigrationStatus(migrations)

	case "create":
		if *migrationName == "" {
			fmt.Println("Migration name is required for create command")
			printUsage()
			os.Exit(1)
		}
		filepath, err := migrationSystem.CreateMigration(*migrationName)
		if err != nil {
			fmt.Printf("Error creating migration: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Created migration: %s\n", filepath)

	case "reset":
		if err := migrationSystem.Reset(); err != nil {
			fmt.Printf("Error resetting database: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Database reset completed successfully")

	case "refresh":
		if err := migrationSystem.Refresh(); err != nil {
			fmt.Printf("Error refreshing database: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Database refresh completed successfully")

	case "version":
		version, err := migrationSystem.GetVersion()
		if err != nil {
			fmt.Printf("Error getting current version: %v\n", err)
			os.Exit(1)
		}
		if version == "" {
			fmt.Println("No migrations applied")
		} else {
			fmt.Printf("Current version: %s\n", version)
		}

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: migrate [OPTIONS] COMMAND")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  up        Run pending migrations")
	fmt.Println("  down      Rollback migrations")
	fmt.Println("  status    Show migration status")
	fmt.Println("  create    Create a new migration")
	fmt.Println("  reset     Reset database (rollback all migrations)")
	fmt.Println("  refresh   Reset and rerun all migrations")
	fmt.Println("  version   Show current migration version")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -config string     Configuration file path (default: config.yaml)")
	fmt.Println("  -migrations string Migrations directory path (default: migrations)")
	fmt.Println("  -version string    Target migration version")
	fmt.Println("  -name string       Migration name for create command")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  migrate up")
	fmt.Println("  migrate down -version=20231201120000")
	fmt.Println("  migrate create -name=\"add user table\"")
	fmt.Println("  migrate status")
}

func printMigrationStatus(migrations []*models.Migration) {
	if len(migrations) == 0 {
		fmt.Println("No migrations found")
		return
	}

	fmt.Printf("%-14s %-10s %-30s %-20s %-15s\n", "VERSION", "STATUS", "NAME", "APPLIED AT", "EXECUTION TIME")
	fmt.Println(strings.Repeat("-", 90))

	for _, migration := range migrations {
		appliedAt := "-"
		executionTime := "-"

		if migration.AppliedAt != nil {
			appliedAt = migration.AppliedAt.Format("2006-01-02 15:04:05")
		}

		if migration.ExecutionTime > 0 {
			executionTime = fmt.Sprintf("%dms", migration.ExecutionTime)
		}

		name := migration.Name
		if len(name) > 30 {
			name = name[:27] + "..."
		}

		fmt.Printf("%-14s %-10s %-30s %-20s %-15s\n",
			migration.Version,
			string(migration.Status),
			name,
			appliedAt,
			executionTime,
		)

		if migration.ErrorMessage != "" {
			fmt.Printf("    Error: %s\n", migration.ErrorMessage)
		}
	}
}

func registerBuiltinMigrations(migrationSystem *database.MigrationSystem) error {
	// Register auto-migration for core models
	autoMigration := &database.MigrationDefinition{
		Version:     "00000000000000", // Earliest version to run first
		Name:        "Auto migrate core models",
		Description: "Automatically migrate core application models",
		Up: func(db *gorm.DB) error {
			return db.AutoMigrate(
				&models.User{},
				&models.Team{},
				&models.TeamMember{},
				&models.DatabaseConnection{},
				&models.DatabaseTable{},
				&models.BackupJob{},
				&models.BackupFile{},
				&models.StorageConfiguration{},
				&models.TablePermission{},
				&models.AuditLog{},
			)
		},
		Down: func(db *gorm.DB) error {
			// Down migration for auto-migrate is complex and risky
			// Better to handle this manually if needed
			return nil
		},
	}

	return migrationSystem.RegisterMigration(autoMigration)
}