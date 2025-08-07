package database

import (
	"crypto/md5"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/dbackup/backend-go/internal/models"
)

// MigrationFunc represents a migration function
type MigrationFunc func(*gorm.DB) error

// MigrationDefinition represents a single migration
type MigrationDefinition struct {
	Version     string
	Name        string
	Description string
	Up          MigrationFunc
	Down        MigrationFunc
	FilePath    string
}

// MigrationSystem handles database migrations with versioning support
type MigrationSystem struct {
	db           *gorm.DB
	migrationsDir string
	migrations   []*MigrationDefinition
	logger       Logger
}

// Logger interface for migration logging
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
}

// DefaultLogger is a simple logger implementation
type DefaultLogger struct{}

func (l DefaultLogger) Info(msg string, args ...interface{}) {
	fmt.Printf("[INFO] "+msg+"\n", args...)
}

func (l DefaultLogger) Error(msg string, args ...interface{}) {
	fmt.Printf("[ERROR] "+msg+"\n", args...)
}

func (l DefaultLogger) Warn(msg string, args ...interface{}) {
	fmt.Printf("[WARN] "+msg+"\n", args...)
}

// NewMigrationSystem creates a new migration system instance
func NewMigrationSystem(db *gorm.DB, migrationsDir string) *MigrationSystem {
	return &MigrationSystem{
		db:            db,
		migrationsDir: migrationsDir,
		migrations:    make([]*MigrationDefinition, 0),
		logger:        DefaultLogger{},
	}
}

// SetLogger sets a custom logger for the migration system
func (ms *MigrationSystem) SetLogger(logger Logger) {
	ms.logger = logger
}

// Initialize initializes the migration system by creating necessary tables
func (ms *MigrationSystem) Initialize() error {
	ms.logger.Info("Initializing migration system...")

	// Create migration tables
	if err := ms.db.AutoMigrate(&models.Migration{}, &models.MigrationBatch{}); err != nil {
		return fmt.Errorf("failed to create migration tables: %w", err)
	}

	ms.logger.Info("Migration system initialized successfully")
	return nil
}

// RegisterMigration registers a migration definition
func (ms *MigrationSystem) RegisterMigration(migration *MigrationDefinition) error {
	if migration.Version == "" {
		return fmt.Errorf("migration version cannot be empty")
	}
	if migration.Name == "" {
		return fmt.Errorf("migration name cannot be empty")
	}
	if migration.Up == nil {
		return fmt.Errorf("migration up function cannot be nil")
	}

	// Check for duplicate versions
	for _, existing := range ms.migrations {
		if existing.Version == migration.Version {
			return fmt.Errorf("migration with version %s already exists", migration.Version)
		}
	}

	ms.migrations = append(ms.migrations, migration)
	return nil
}

// LoadMigrationsFromDir loads migration files from the specified directory
func (ms *MigrationSystem) LoadMigrationsFromDir() error {
	if ms.migrationsDir == "" {
		return fmt.Errorf("migrations directory not set")
	}

	if _, err := os.Stat(ms.migrationsDir); os.IsNotExist(err) {
		ms.logger.Warn("Migrations directory does not exist: %s", ms.migrationsDir)
		return nil
	}

	ms.logger.Info("Loading migrations from directory: %s", ms.migrationsDir)

	err := filepath.WalkDir(ms.migrationsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".sql") {
			return nil
		}

		migration, err := ms.parseMigrationFile(path)
		if err != nil {
			ms.logger.Error("Failed to parse migration file %s: %v", path, err)
			return err
		}

		if migration != nil {
			if err := ms.RegisterMigration(migration); err != nil {
				ms.logger.Error("Failed to register migration %s: %v", path, err)
				return err
			}
			ms.logger.Info("Loaded migration: %s - %s", migration.Version, migration.Name)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Sort migrations by version
	ms.sortMigrations()

	ms.logger.Info("Loaded %d migrations from directory", len(ms.migrations))
	return nil
}

// parseMigrationFile parses a SQL migration file
func (ms *MigrationSystem) parseMigrationFile(filePath string) (*MigrationDefinition, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read migration file: %w", err)
	}

	filename := filepath.Base(filePath)
	version, name, err := ms.parseMigrationFilename(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to parse migration filename: %w", err)
	}

	migration := &MigrationDefinition{
		Version:  version,
		Name:     name,
		FilePath: filePath,
		Up:       ms.createSQLMigrationFunc(string(content), true),
		Down:     ms.createSQLMigrationFunc(string(content), false),
	}

	return migration, nil
}

// parseMigrationFilename parses migration filename to extract version and name
func (ms *MigrationSystem) parseMigrationFilename(filename string) (version, name string, err error) {
	// Expected format: YYYYMMDDHHMMSS_migration_name.sql
	re := regexp.MustCompile(`^(\d{14})_(.+)\.sql$`)
	matches := re.FindStringSubmatch(filename)
	
	if len(matches) != 3 {
		return "", "", fmt.Errorf("invalid migration filename format: %s", filename)
	}

	version = matches[1]
	name = strings.ReplaceAll(matches[2], "_", " ")
	
	return version, name, nil
}

// createSQLMigrationFunc creates a migration function from SQL content
func (ms *MigrationSystem) createSQLMigrationFunc(content string, isUp bool) MigrationFunc {
	return func(db *gorm.DB) error {
		var sql string
		
		if isUp {
			sql = ms.extractUpSQL(content)
		} else {
			sql = ms.extractDownSQL(content)
		}
		
		if sql == "" {
			if isUp {
				return fmt.Errorf("no UP SQL found in migration")
			}
			// Down migrations are optional
			return nil
		}

		// Split SQL into statements and execute each one
		statements := ms.splitSQLStatements(sql)
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			
			if err := db.Exec(stmt).Error; err != nil {
				return fmt.Errorf("failed to execute SQL statement: %w", err)
			}
		}

		return nil
	}
}

// extractUpSQL extracts the UP migration SQL from file content
func (ms *MigrationSystem) extractUpSQL(content string) string {
	lines := strings.Split(content, "\n")
	var upSQL []string
	inUp := false
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if strings.HasPrefix(line, "-- +migrate Up") {
			inUp = true
			continue
		}
		
		if strings.HasPrefix(line, "-- +migrate Down") {
			break
		}
		
		if inUp {
			upSQL = append(upSQL, line)
		}
	}
	
	return strings.Join(upSQL, "\n")
}

// extractDownSQL extracts the DOWN migration SQL from file content
func (ms *MigrationSystem) extractDownSQL(content string) string {
	lines := strings.Split(content, "\n")
	var downSQL []string
	inDown := false
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if strings.HasPrefix(line, "-- +migrate Down") {
			inDown = true
			continue
		}
		
		if inDown {
			downSQL = append(downSQL, line)
		}
	}
	
	return strings.Join(downSQL, "\n")
}

// splitSQLStatements splits SQL content into individual statements
func (ms *MigrationSystem) splitSQLStatements(sql string) []string {
	// Simple statement splitter - can be enhanced for more complex cases
	statements := strings.Split(sql, ";")
	var result []string
	
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt != "" {
			result = append(result, stmt)
		}
	}
	
	return result
}

// sortMigrations sorts migrations by version in ascending order
func (ms *MigrationSystem) sortMigrations() {
	sort.Slice(ms.migrations, func(i, j int) bool {
		return ms.migrations[i].Version < ms.migrations[j].Version
	})
}

// Up runs pending migrations
func (ms *MigrationSystem) Up(targetVersion string) error {
	ms.logger.Info("Starting migration up to version: %s", targetVersion)

	// Get pending migrations
	pendingMigrations, err := ms.getPendingMigrations(targetVersion)
	if err != nil {
		return fmt.Errorf("failed to get pending migrations: %w", err)
	}

	if len(pendingMigrations) == 0 {
		ms.logger.Info("No pending migrations to run")
		return nil
	}

	// Create migration batch
	batch, err := ms.createMigrationBatch(len(pendingMigrations))
	if err != nil {
		return fmt.Errorf("failed to create migration batch: %w", err)
	}

	ms.logger.Info("Running %d pending migrations in batch %d", len(pendingMigrations), batch.BatchNumber)

	// Run migrations
	successCount := 0
	for _, migration := range pendingMigrations {
		if err := ms.runMigration(migration, true); err != nil {
			ms.logger.Error("Migration %s failed: %v", migration.Version, err)
			batch.FailureCount++
		} else {
			ms.logger.Info("Migration %s completed successfully", migration.Version)
			batch.SuccessCount++
			successCount++
		}
		batch.TotalCount++
	}

	// Update batch status
	batch.MarkCompleted()
	if err := ms.db.Save(batch).Error; err != nil {
		ms.logger.Error("Failed to update migration batch: %v", err)
	}

	if batch.FailureCount > 0 {
		return fmt.Errorf("migration batch completed with %d failures out of %d migrations", 
			batch.FailureCount, batch.TotalCount)
	}

	ms.logger.Info("Migration batch completed successfully: %d/%d migrations applied", 
		successCount, len(pendingMigrations))
	return nil
}

// Down rolls back migrations
func (ms *MigrationSystem) Down(targetVersion string) error {
	ms.logger.Info("Starting migration down to version: %s", targetVersion)

	// Get applied migrations to rollback
	migrationsToRollback, err := ms.getMigrationsToRollback(targetVersion)
	if err != nil {
		return fmt.Errorf("failed to get migrations to rollback: %w", err)
	}

	if len(migrationsToRollback) == 0 {
		ms.logger.Info("No migrations to rollback")
		return nil
	}

	ms.logger.Info("Rolling back %d migrations", len(migrationsToRollback))

	// Rollback migrations in reverse order
	for i := len(migrationsToRollback) - 1; i >= 0; i-- {
		migration := migrationsToRollback[i]
		if err := ms.runMigration(migration, false); err != nil {
			ms.logger.Error("Rollback of migration %s failed: %v", migration.Version, err)
			return fmt.Errorf("rollback failed at migration %s: %w", migration.Version, err)
		}
		ms.logger.Info("Migration %s rolled back successfully", migration.Version)
	}

	ms.logger.Info("Migration rollback completed successfully")
	return nil
}

// Status returns the current migration status
func (ms *MigrationSystem) Status() ([]*models.Migration, error) {
	var migrations []*models.Migration
	err := ms.db.Order("version ASC").Find(&migrations).Error
	return migrations, err
}

// getPendingMigrations returns migrations that need to be applied
func (ms *MigrationSystem) getPendingMigrations(targetVersion string) ([]*MigrationDefinition, error) {
	// Get applied migrations
	var appliedMigrations []models.Migration
	err := ms.db.Where("status = ?", models.MigrationStatusApplied).Find(&appliedMigrations).Error
	if err != nil {
		return nil, err
	}

	// Create map for quick lookup
	appliedMap := make(map[string]bool)
	for _, m := range appliedMigrations {
		appliedMap[m.Version] = true
	}

	// Find pending migrations
	var pending []*MigrationDefinition
	for _, migration := range ms.migrations {
		if targetVersion != "" && migration.Version > targetVersion {
			break
		}
		if !appliedMap[migration.Version] {
			pending = append(pending, migration)
		}
	}

	return pending, nil
}

// getMigrationsToRollback returns migrations that need to be rolled back
func (ms *MigrationSystem) getMigrationsToRollback(targetVersion string) ([]*MigrationDefinition, error) {
	// Get applied migrations after target version
	var appliedMigrations []models.Migration
	query := ms.db.Where("status = ?", models.MigrationStatusApplied)
	if targetVersion != "" {
		query = query.Where("version > ?", targetVersion)
	}
	err := query.Order("version DESC").Find(&appliedMigrations).Error
	if err != nil {
		return nil, err
	}

	// Find migration definitions
	var toRollback []*MigrationDefinition
	migrationMap := make(map[string]*MigrationDefinition)
	for _, m := range ms.migrations {
		migrationMap[m.Version] = m
	}

	for _, applied := range appliedMigrations {
		if migration, exists := migrationMap[applied.Version]; exists {
			toRollback = append(toRollback, migration)
		}
	}

	return toRollback, nil
}

// runMigration executes a single migration
func (ms *MigrationSystem) runMigration(migration *MigrationDefinition, isUp bool) error {
	startTime := time.Now()

	// Get or create migration record
	var migrationRecord models.Migration
	err := ms.db.Where("version = ?", migration.Version).First(&migrationRecord).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			migrationRecord = models.Migration{
				Version:     migration.Version,
				Name:        migration.Name,
				Description: migration.Description,
				Status:      models.MigrationStatusPending,
				Checksum:    ms.calculateChecksum(migration),
			}
			if err := ms.db.Create(&migrationRecord).Error; err != nil {
				return fmt.Errorf("failed to create migration record: %w", err)
			}
		} else {
			return fmt.Errorf("failed to get migration record: %w", err)
		}
	}

	// Execute migration in transaction
	err = ms.db.Transaction(func(tx *gorm.DB) error {
		var migrationFunc MigrationFunc
		if isUp {
			migrationFunc = migration.Up
		} else {
			migrationFunc = migration.Down
		}

		if migrationFunc == nil {
			if !isUp {
				// Down migration is optional
				return nil
			}
			return fmt.Errorf("migration function not found")
		}

		return migrationFunc(tx)
	})

	executionTime := time.Since(startTime)

	// Update migration record
	if err != nil {
		migrationRecord.MarkFailed(err.Error())
	} else {
		if isUp {
			migrationRecord.MarkApplied(executionTime)
		} else {
			migrationRecord.MarkRollback()
		}
	}

	if saveErr := ms.db.Save(&migrationRecord).Error; saveErr != nil {
		ms.logger.Error("Failed to update migration record: %v", saveErr)
	}

	return err
}

// createMigrationBatch creates a new migration batch
func (ms *MigrationSystem) createMigrationBatch(count int) (*models.MigrationBatch, error) {
	// Get the next batch number
	var lastBatch models.MigrationBatch
	ms.db.Order("batch_number DESC").First(&lastBatch)

	batch := &models.MigrationBatch{
		BatchNumber:  lastBatch.BatchNumber + 1,
		Status:       "running",
		TotalCount:   count,
		SuccessCount: 0,
		FailureCount: 0,
	}

	if err := ms.db.Create(batch).Error; err != nil {
		return nil, err
	}

	return batch, nil
}

// calculateChecksum calculates a checksum for migration content
func (ms *MigrationSystem) calculateChecksum(migration *MigrationDefinition) string {
	content := fmt.Sprintf("%s:%s:%s", migration.Version, migration.Name, migration.Description)
	if migration.FilePath != "" {
		if fileContent, err := os.ReadFile(migration.FilePath); err == nil {
			content += string(fileContent)
		}
	}
	return fmt.Sprintf("%x", md5.Sum([]byte(content)))
}

// CreateMigration creates a new migration file
func (ms *MigrationSystem) CreateMigration(name string) (string, error) {
	if ms.migrationsDir == "" {
		return "", fmt.Errorf("migrations directory not set")
	}

	// Create migrations directory if it doesn't exist
	if err := os.MkdirAll(ms.migrationsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Generate version timestamp
	version := time.Now().Format("20060102150405")
	
	// Clean migration name
	cleanName := strings.ReplaceAll(strings.ToLower(name), " ", "_")
	cleanName = regexp.MustCompile(`[^a-z0-9_]`).ReplaceAllString(cleanName, "")

	// Create filename
	filename := fmt.Sprintf("%s_%s.sql", version, cleanName)
	filepath := filepath.Join(ms.migrationsDir, filename)

	// Create migration file template
	template := fmt.Sprintf(`-- +migrate Up
-- %s
-- Created: %s

-- Add your UP migration SQL here


-- +migrate Down
-- Rollback for: %s

-- Add your DOWN migration SQL here (optional)

`, name, time.Now().Format("2006-01-02 15:04:05"), name)

	if err := os.WriteFile(filepath, []byte(template), 0644); err != nil {
		return "", fmt.Errorf("failed to create migration file: %w", err)
	}

	ms.logger.Info("Created migration file: %s", filename)
	return filepath, nil
}

// Reset resets the entire database by running all down migrations
func (ms *MigrationSystem) Reset() error {
	ms.logger.Info("Resetting database...")

	// Get all applied migrations in reverse order
	var appliedMigrations []models.Migration
	err := ms.db.Where("status = ?", models.MigrationStatusApplied).
		Order("version DESC").Find(&appliedMigrations).Error
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	if len(appliedMigrations) == 0 {
		ms.logger.Info("No migrations to reset")
		return nil
	}

	// Rollback all migrations
	return ms.Down("")
}

// Refresh resets the database and runs all migrations again
func (ms *MigrationSystem) Refresh() error {
	ms.logger.Info("Refreshing database...")

	if err := ms.Reset(); err != nil {
		return fmt.Errorf("failed to reset database: %w", err)
	}

	return ms.Up("")
}

// GetVersion returns the current migration version
func (ms *MigrationSystem) GetVersion() (string, error) {
	var migration models.Migration
	err := ms.db.Where("status = ?", models.MigrationStatusApplied).
		Order("version DESC").First(&migration).Error
	
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", err
	}

	return migration.Version, nil
}