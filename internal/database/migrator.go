package database

import (
	"fmt"
	"log"
	"reflect"

	"gorm.io/gorm"
)

// MigratorInterface defines the interface for database migration operations
type MigratorInterface interface {
	AutoMigrate(models ...interface{}) error
	CreateTable(models ...interface{}) error
	DropTable(models ...interface{}) error
	HasTable(model interface{}) bool
	CreateIndex(model interface{}, indexName string) error
	DropIndex(model interface{}, indexName string) error
	HasIndex(model interface{}, indexName string) bool
	CreateConstraint(model interface{}, constraintName string) error
	DropConstraint(model interface{}, constraintName string) error
	HasConstraint(model interface{}, constraintName string) bool
}

// Migrator wraps GORM's migrator with additional functionality
type Migrator struct {
	db *gorm.DB
}

// NewMigrator creates a new migrator instance
func NewMigrator(db *gorm.DB) *Migrator {
	return &Migrator{db: db}
}

// AutoMigrate automatically migrates the given models
func (m *Migrator) AutoMigrate(models ...interface{}) error {
	if len(models) == 0 {
		return fmt.Errorf("no models provided for migration")
	}

	log.Printf("Starting auto-migration for %d models", len(models))

	for _, model := range models {
		modelName := getModelName(model)
		log.Printf("Migrating model: %s", modelName)

		if err := m.db.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate model %s: %w", modelName, err)
		}

		log.Printf("Successfully migrated model: %s", modelName)
	}

	log.Printf("Auto-migration completed successfully")
	return nil
}

// CreateTable creates tables for the given models
func (m *Migrator) CreateTable(models ...interface{}) error {
	migrator := m.db.Migrator()

	for _, model := range models {
		modelName := getModelName(model)
		
		if migrator.HasTable(model) {
			log.Printf("Table for model %s already exists, skipping", modelName)
			continue
		}

		if err := migrator.CreateTable(model); err != nil {
			return fmt.Errorf("failed to create table for model %s: %w", modelName, err)
		}

		log.Printf("Created table for model: %s", modelName)
	}

	return nil
}

// DropTable drops tables for the given models
func (m *Migrator) DropTable(models ...interface{}) error {
	migrator := m.db.Migrator()

	for _, model := range models {
		modelName := getModelName(model)
		
		if !migrator.HasTable(model) {
			log.Printf("Table for model %s does not exist, skipping", modelName)
			continue
		}

		if err := migrator.DropTable(model); err != nil {
			return fmt.Errorf("failed to drop table for model %s: %w", modelName, err)
		}

		log.Printf("Dropped table for model: %s", modelName)
	}

	return nil
}

// HasTable checks if a table exists for the given model
func (m *Migrator) HasTable(model interface{}) bool {
	return m.db.Migrator().HasTable(model)
}

// CreateIndex creates an index for the given model
func (m *Migrator) CreateIndex(model interface{}, indexName string) error {
	migrator := m.db.Migrator()
	
	if migrator.HasIndex(model, indexName) {
		log.Printf("Index %s already exists for model %s, skipping", indexName, getModelName(model))
		return nil
	}

	if err := migrator.CreateIndex(model, indexName); err != nil {
		return fmt.Errorf("failed to create index %s for model %s: %w", indexName, getModelName(model), err)
	}

	log.Printf("Created index %s for model %s", indexName, getModelName(model))
	return nil
}

// DropIndex drops an index for the given model
func (m *Migrator) DropIndex(model interface{}, indexName string) error {
	migrator := m.db.Migrator()
	
	if !migrator.HasIndex(model, indexName) {
		log.Printf("Index %s does not exist for model %s, skipping", indexName, getModelName(model))
		return nil
	}

	if err := migrator.DropIndex(model, indexName); err != nil {
		return fmt.Errorf("failed to drop index %s for model %s: %w", indexName, getModelName(model), err)
	}

	log.Printf("Dropped index %s for model %s", indexName, getModelName(model))
	return nil
}

// HasIndex checks if an index exists for the given model
func (m *Migrator) HasIndex(model interface{}, indexName string) bool {
	return m.db.Migrator().HasIndex(model, indexName)
}

// CreateConstraint creates a constraint for the given model
func (m *Migrator) CreateConstraint(model interface{}, constraintName string) error {
	migrator := m.db.Migrator()
	
	if migrator.HasConstraint(model, constraintName) {
		log.Printf("Constraint %s already exists for model %s, skipping", constraintName, getModelName(model))
		return nil
	}

	if err := migrator.CreateConstraint(model, constraintName); err != nil {
		return fmt.Errorf("failed to create constraint %s for model %s: %w", constraintName, getModelName(model), err)
	}

	log.Printf("Created constraint %s for model %s", constraintName, getModelName(model))
	return nil
}

// DropConstraint drops a constraint for the given model
func (m *Migrator) DropConstraint(model interface{}, constraintName string) error {
	migrator := m.db.Migrator()
	
	if !migrator.HasConstraint(model, constraintName) {
		log.Printf("Constraint %s does not exist for model %s, skipping", constraintName, getModelName(model))
		return nil
	}

	if err := migrator.DropConstraint(model, constraintName); err != nil {
		return fmt.Errorf("failed to drop constraint %s for model %s: %w", constraintName, getModelName(model), err)
	}

	log.Printf("Dropped constraint %s for model %s", constraintName, getModelName(model))
	return nil
}

// HasConstraint checks if a constraint exists for the given model
func (m *Migrator) HasConstraint(model interface{}, constraintName string) bool {
	return m.db.Migrator().HasConstraint(model, constraintName)
}

// RunMigrations runs a series of migration operations in a transaction
func (m *Migrator) RunMigrations(migrations []func(*Migrator) error) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		txMigrator := NewMigrator(tx)
		
		for i, migration := range migrations {
			log.Printf("Running migration step %d/%d", i+1, len(migrations))
			
			if err := migration(txMigrator); err != nil {
				return fmt.Errorf("migration step %d failed: %w", i+1, err)
			}
		}
		
		log.Printf("All migration steps completed successfully")
		return nil
	})
}

// GetMigrator returns a migrator for the current database instance
func GetMigrator() *Migrator {
	return NewMigrator(GetDB())
}

// RunFullMigration runs the complete migration process including versioned migrations
func RunFullMigration() error {
	db := GetDB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Initialize versioned migration system
	migrationSystem := NewMigrationSystem(db, "migrations")
	migrationSystem.SetLogger(DefaultLogger{})

	// Initialize migration tables
	if err := migrationSystem.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize migration system: %w", err)
	}

	// Load migrations from files
	if err := migrationSystem.LoadMigrationsFromDir(); err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Run pending migrations
	if err := migrationSystem.Up(""); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Printf("Migration system completed successfully")
	return nil
}

// GetMigrationSystem returns a configured migration system instance
func GetMigrationSystem(migrationsDir string) *MigrationSystem {
	db := GetDB()
	if db == nil {
		log.Printf("Warning: database not initialized for migration system")
		return nil
	}

	migrationSystem := NewMigrationSystem(db, migrationsDir)
	migrationSystem.SetLogger(DefaultLogger{})
	return migrationSystem
}

// getModelName extracts the model name from the given interface
func getModelName(model interface{}) string {
	if model == nil {
		return "unknown"
	}

	t := reflect.TypeOf(model)
	
	// Handle pointers
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	
	// Handle slices
	if t.Kind() == reflect.Slice {
		t = t.Elem()
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
	}
	
	return t.Name()
}