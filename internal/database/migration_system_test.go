package database

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"

	"github.com/dbackup/backend-go/internal/models"
)

type MigrationSystemTestSuite struct {
	suite.Suite
	db              *gorm.DB
	migrationSystem *MigrationSystem
	tempDir         string
}

func TestMigrationSystemSuite(t *testing.T) {
	suite.Run(t, new(MigrationSystemTestSuite))
}

func (suite *MigrationSystemTestSuite) SetupSuite() {
	// Create temporary directory for test migrations
	tempDir, err := os.MkdirTemp("", "test_migrations")
	require.NoError(suite.T(), err)
	suite.tempDir = tempDir

	// Setup test database
	db, err := SetupTestDB("migration_system_test")
	require.NoError(suite.T(), err)
	suite.db = db

	// Initialize migration system
	suite.migrationSystem = NewMigrationSystem(suite.db, suite.tempDir)
	err = suite.migrationSystem.Initialize()
	require.NoError(suite.T(), err)
}

func (suite *MigrationSystemTestSuite) TearDownSuite() {
	// Clean up test database
	CleanupTestDB(suite.db, "migration_system_test")

	// Remove temporary directory
	os.RemoveAll(suite.tempDir)
}

func (suite *MigrationSystemTestSuite) SetupTest() {
	// Clean migration data before each test
	suite.db.Exec("DELETE FROM migration_batches")
	suite.db.Exec("DELETE FROM schema_migrations")
	
	// Reset migration system
	suite.migrationSystem.migrations = make([]*MigrationDefinition, 0)
}

func (suite *MigrationSystemTestSuite) TestInitialize() {
	t := suite.T()

	// Test that migration tables are created
	assert.True(t, suite.db.Migrator().HasTable(&models.Migration{}))
	assert.True(t, suite.db.Migrator().HasTable(&models.MigrationBatch{}))
}

func (suite *MigrationSystemTestSuite) TestRegisterMigration() {
	t := suite.T()

	migration := &MigrationDefinition{
		Version:     "20240101000001",
		Name:        "test migration",
		Description: "A test migration",
		Up: func(db *gorm.DB) error {
			return nil
		},
		Down: func(db *gorm.DB) error {
			return nil
		},
	}

	// Test successful registration
	err := suite.migrationSystem.RegisterMigration(migration)
	assert.NoError(t, err)
	assert.Len(t, suite.migrationSystem.migrations, 1)

	// Test duplicate version error
	duplicate := &MigrationDefinition{
		Version: "20240101000001",
		Name:    "duplicate",
		Up:      func(db *gorm.DB) error { return nil },
	}
	err = suite.migrationSystem.RegisterMigration(duplicate)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// Test validation errors
	testCases := []struct {
		name      string
		migration *MigrationDefinition
		errorMsg  string
	}{
		{
			name: "empty version",
			migration: &MigrationDefinition{
				Name: "test",
				Up:   func(db *gorm.DB) error { return nil },
			},
			errorMsg: "version cannot be empty",
		},
		{
			name: "empty name",
			migration: &MigrationDefinition{
				Version: "20240101000002",
				Up:      func(db *gorm.DB) error { return nil },
			},
			errorMsg: "name cannot be empty",
		},
		{
			name: "nil up function",
			migration: &MigrationDefinition{
				Version: "20240101000003",
				Name:    "test",
			},
			errorMsg: "up function cannot be nil",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := suite.migrationSystem.RegisterMigration(tc.migration)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.errorMsg)
		})
	}
}

func (suite *MigrationSystemTestSuite) TestParseMigrationFilename() {
	t := suite.T()

	testCases := []struct {
		filename        string
		expectedVersion string
		expectedName    string
		expectError     bool
	}{
		{
			filename:        "20240101123456_create_users_table.sql",
			expectedVersion: "20240101123456",
			expectedName:    "create users table",
			expectError:     false,
		},
		{
			filename:        "20240202090000_add_indexes.sql",
			expectedVersion: "20240202090000",
			expectedName:    "add indexes",
			expectError:     false,
		},
		{
			filename:    "invalid_filename.sql",
			expectError: true,
		},
		{
			filename:    "20240101_missing_seconds.sql",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			version, name, err := suite.migrationSystem.parseMigrationFilename(tc.filename)
			
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedVersion, version)
				assert.Equal(t, tc.expectedName, name)
			}
		})
	}
}

func (suite *MigrationSystemTestSuite) TestCreateMigrationFile() {
	t := suite.T()

	migrationName := "create test table"
	filePath, err := suite.migrationSystem.CreateMigration(migrationName)
	
	require.NoError(t, err)
	assert.Contains(t, filePath, "create_test_table.sql")
	
	// Verify file was created
	_, err = os.Stat(filePath)
	assert.NoError(t, err)
	
	// Verify file content
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	
	contentStr := string(content)
	assert.Contains(t, contentStr, "-- +migrate Up")
	assert.Contains(t, contentStr, "-- +migrate Down")
	assert.Contains(t, contentStr, "create test table")
}

func (suite *MigrationSystemTestSuite) TestLoadMigrationsFromDir() {
	t := suite.T()

	// Create test migration files
	migrations := []struct {
		filename string
		content  string
	}{
		{
			filename: "20240101000001_first_migration.sql",
			content: `-- +migrate Up
CREATE TABLE test_table1 (id SERIAL PRIMARY KEY);

-- +migrate Down
DROP TABLE test_table1;`,
		},
		{
			filename: "20240101000002_second_migration.sql",
			content: `-- +migrate Up
CREATE TABLE test_table2 (id SERIAL PRIMARY KEY);

-- +migrate Down
DROP TABLE test_table2;`,
		},
	}

	for _, migration := range migrations {
		filePath := filepath.Join(suite.tempDir, migration.filename)
		err := os.WriteFile(filePath, []byte(migration.content), 0644)
		require.NoError(t, err)
	}

	// Load migrations
	err := suite.migrationSystem.LoadMigrationsFromDir()
	require.NoError(t, err)

	// Verify migrations were loaded and sorted
	assert.Len(t, suite.migrationSystem.migrations, 2)
	assert.Equal(t, "20240101000001", suite.migrationSystem.migrations[0].Version)
	assert.Equal(t, "20240101000002", suite.migrationSystem.migrations[1].Version)
}

func (suite *MigrationSystemTestSuite) TestExtractSQL() {
	t := suite.T()

	content := `-- +migrate Up
CREATE TABLE users (id SERIAL PRIMARY KEY);
CREATE INDEX idx_users_id ON users(id);

-- +migrate Down
DROP INDEX idx_users_id;
DROP TABLE users;`

	// Test UP SQL extraction
	upSQL := suite.migrationSystem.extractUpSQL(content)
	assert.Contains(t, upSQL, "CREATE TABLE users")
	assert.Contains(t, upSQL, "CREATE INDEX idx_users_id")
	assert.NotContains(t, upSQL, "DROP TABLE")

	// Test DOWN SQL extraction
	downSQL := suite.migrationSystem.extractDownSQL(content)
	assert.Contains(t, downSQL, "DROP INDEX idx_users_id")
	assert.Contains(t, downSQL, "DROP TABLE users")
	assert.NotContains(t, downSQL, "CREATE TABLE")
}

func (suite *MigrationSystemTestSuite) TestRunMigrationUp() {
	t := suite.T()

	// Register test migration
	migration := &MigrationDefinition{
		Version:     "20240101000001",
		Name:        "create test table",
		Description: "Test migration",
		Up: func(db *gorm.DB) error {
			return db.Exec("CREATE TABLE migration_test (id SERIAL PRIMARY KEY, name VARCHAR(255))").Error
		},
		Down: func(db *gorm.DB) error {
			return db.Exec("DROP TABLE migration_test").Error
		},
	}

	err := suite.migrationSystem.RegisterMigration(migration)
	require.NoError(t, err)

	// Run migration up
	err = suite.migrationSystem.Up("")
	assert.NoError(t, err)

	// Verify table was created
	assert.True(t, suite.db.Migrator().HasTable("migration_test"))

	// Verify migration record was created
	var migrationRecord models.Migration
	err = suite.db.Where("version = ?", "20240101000001").First(&migrationRecord).Error
	assert.NoError(t, err)
	assert.Equal(t, models.MigrationStatusApplied, migrationRecord.Status)
	assert.NotNil(t, migrationRecord.AppliedAt)
	assert.Greater(t, migrationRecord.ExecutionTime, int64(0))
}

func (suite *MigrationSystemTestSuite) TestRunMigrationDown() {
	t := suite.T()

	// Register test migration
	migration := &MigrationDefinition{
		Version:     "20240101000001",
		Name:        "create test table",
		Description: "Test migration",
		Up: func(db *gorm.DB) error {
			return db.Exec("CREATE TABLE migration_test_rollback (id SERIAL PRIMARY KEY)").Error
		},
		Down: func(db *gorm.DB) error {
			return db.Exec("DROP TABLE migration_test_rollback").Error
		},
	}

	err := suite.migrationSystem.RegisterMigration(migration)
	require.NoError(t, err)

	// Run migration up first
	err = suite.migrationSystem.Up("")
	require.NoError(t, err)
	assert.True(t, suite.db.Migrator().HasTable("migration_test_rollback"))

	// Run migration down
	err = suite.migrationSystem.Down("")
	assert.NoError(t, err)

	// Verify table was dropped
	assert.False(t, suite.db.Migrator().HasTable("migration_test_rollback"))

	// Verify migration record was updated
	var migrationRecord models.Migration
	err = suite.db.Where("version = ?", "20240101000001").First(&migrationRecord).Error
	assert.NoError(t, err)
	assert.Equal(t, models.MigrationStatusRollback, migrationRecord.Status)
	assert.NotNil(t, migrationRecord.RolledAt)
}

func (suite *MigrationSystemTestSuite) TestMigrationFailure() {
	t := suite.T()

	// Register migration that will fail
	migration := &MigrationDefinition{
		Version:     "20240101000001",
		Name:        "failing migration",
		Description: "Migration that will fail",
		Up: func(db *gorm.DB) error {
			return fmt.Errorf("intentional migration failure")
		},
	}

	err := suite.migrationSystem.RegisterMigration(migration)
	require.NoError(t, err)

	// Run migration up (should fail)
	err = suite.migrationSystem.Up("")
	assert.Error(t, err)

	// Verify migration record shows failure
	var migrationRecord models.Migration
	err = suite.db.Where("version = ?", "20240101000001").First(&migrationRecord).Error
	assert.NoError(t, err)
	assert.Equal(t, models.MigrationStatusFailed, migrationRecord.Status)
	assert.Contains(t, migrationRecord.ErrorMessage, "intentional migration failure")
	assert.Nil(t, migrationRecord.AppliedAt)
}

func (suite *MigrationSystemTestSuite) TestMigrationBatch() {
	t := suite.T()

	// Register multiple migrations
	migrations := []*MigrationDefinition{
		{
			Version: "20240101000001",
			Name:    "migration 1",
			Up: func(db *gorm.DB) error {
				return db.Exec("CREATE TABLE batch_test1 (id SERIAL)").Error
			},
		},
		{
			Version: "20240101000002",
			Name:    "migration 2",
			Up: func(db *gorm.DB) error {
				return db.Exec("CREATE TABLE batch_test2 (id SERIAL)").Error
			},
		},
	}

	for _, migration := range migrations {
		err := suite.migrationSystem.RegisterMigration(migration)
		require.NoError(t, err)
	}

	// Run migrations
	err := suite.migrationSystem.Up("")
	assert.NoError(t, err)

	// Verify batch was created
	var batch models.MigrationBatch
	err = suite.db.Order("batch_number DESC").First(&batch).Error
	assert.NoError(t, err)
	assert.Equal(t, 2, batch.TotalCount)
	assert.Equal(t, 2, batch.SuccessCount)
	assert.Equal(t, 0, batch.FailureCount)
	assert.Equal(t, "completed", batch.Status)
	assert.NotNil(t, batch.CompletedAt)
}

func (suite *MigrationSystemTestSuite) TestGetPendingMigrations() {
	t := suite.T()

	// Register test migrations
	migrations := []*MigrationDefinition{
		{
			Version: "20240101000001",
			Name:    "migration 1",
			Up:      func(db *gorm.DB) error { return nil },
		},
		{
			Version: "20240101000002",
			Name:    "migration 2",
			Up:      func(db *gorm.DB) error { return nil },
		},
		{
			Version: "20240101000003",
			Name:    "migration 3",
			Up:      func(db *gorm.DB) error { return nil },
		},
	}

	for _, migration := range migrations {
		err := suite.migrationSystem.RegisterMigration(migration)
		require.NoError(t, err)
	}

	// Apply first migration manually
	migrationRecord := models.Migration{
		Version:   "20240101000001",
		Name:      "migration 1",
		Status:    models.MigrationStatusApplied,
		AppliedAt: &time.Time{},
		Checksum:  "test",
	}
	now := time.Now()
	migrationRecord.AppliedAt = &now
	err := suite.db.Create(&migrationRecord).Error
	require.NoError(t, err)

	// Get pending migrations
	pending, err := suite.migrationSystem.getPendingMigrations("")
	assert.NoError(t, err)
	assert.Len(t, pending, 2)
	assert.Equal(t, "20240101000002", pending[0].Version)
	assert.Equal(t, "20240101000003", pending[1].Version)

	// Test with target version
	pending, err = suite.migrationSystem.getPendingMigrations("20240101000002")
	assert.NoError(t, err)
	assert.Len(t, pending, 1)
	assert.Equal(t, "20240101000002", pending[0].Version)
}

func (suite *MigrationSystemTestSuite) TestStatus() {
	t := suite.T()

	// Create test migration records
	migrations := []models.Migration{
		{
			Version:   "20240101000001",
			Name:      "migration 1",
			Status:    models.MigrationStatusApplied,
			AppliedAt: &time.Time{},
			Checksum:  "test1",
		},
		{
			Version:      "20240101000002",
			Name:         "migration 2",
			Status:       models.MigrationStatusFailed,
			ErrorMessage: "test error",
			Checksum:     "test2",
		},
	}

	for i := range migrations {
		now := time.Now()
		if migrations[i].AppliedAt != nil {
			migrations[i].AppliedAt = &now
		}
		err := suite.db.Create(&migrations[i]).Error
		require.NoError(t, err)
	}

	// Get status
	status, err := suite.migrationSystem.Status()
	assert.NoError(t, err)
	assert.Len(t, status, 2)
	assert.Equal(t, "20240101000001", status[0].Version)
	assert.Equal(t, models.MigrationStatusApplied, status[0].Status)
	assert.Equal(t, "20240101000002", status[1].Version)
	assert.Equal(t, models.MigrationStatusFailed, status[1].Status)
}

func (suite *MigrationSystemTestSuite) TestGetVersion() {
	t := suite.T()

	// Test with no migrations
	version, err := suite.migrationSystem.GetVersion()
	assert.NoError(t, err)
	assert.Empty(t, version)

	// Create applied migration
	migration := models.Migration{
		Version:   "20240101000001",
		Name:      "test migration",
		Status:    models.MigrationStatusApplied,
		AppliedAt: &time.Time{},
		Checksum:  "test",
	}
	now := time.Now()
	migration.AppliedAt = &now
	err = suite.db.Create(&migration).Error
	require.NoError(t, err)

	// Get version
	version, err = suite.migrationSystem.GetVersion()
	assert.NoError(t, err)
	assert.Equal(t, "20240101000001", version)
}

func (suite *MigrationSystemTestSuite) TestTargetVersion() {
	t := suite.T()

	// Register multiple migrations
	migrations := []*MigrationDefinition{
		{
			Version: "20240101000001",
			Name:    "migration 1",
			Up: func(db *gorm.DB) error {
				return db.Exec("CREATE TABLE target_test1 (id SERIAL)").Error
			},
			Down: func(db *gorm.DB) error {
				return db.Exec("DROP TABLE target_test1").Error
			},
		},
		{
			Version: "20240101000002",
			Name:    "migration 2",
			Up: func(db *gorm.DB) error {
				return db.Exec("CREATE TABLE target_test2 (id SERIAL)").Error
			},
			Down: func(db *gorm.DB) error {
				return db.Exec("DROP TABLE target_test2").Error
			},
		},
		{
			Version: "20240101000003",
			Name:    "migration 3",
			Up: func(db *gorm.DB) error {
				return db.Exec("CREATE TABLE target_test3 (id SERIAL)").Error
			},
			Down: func(db *gorm.DB) error {
				return db.Exec("DROP TABLE target_test3").Error
			},
		},
	}

	for _, migration := range migrations {
		err := suite.migrationSystem.RegisterMigration(migration)
		require.NoError(t, err)
	}

	// Migrate up to version 2
	err := suite.migrationSystem.Up("20240101000002")
	assert.NoError(t, err)

	// Verify only first 2 tables exist
	assert.True(t, suite.db.Migrator().HasTable("target_test1"))
	assert.True(t, suite.db.Migrator().HasTable("target_test2"))
	assert.False(t, suite.db.Migrator().HasTable("target_test3"))

	// Migrate down to version 1
	err = suite.migrationSystem.Down("20240101000001")
	assert.NoError(t, err)

	// Verify only first table exists
	assert.True(t, suite.db.Migrator().HasTable("target_test1"))
	assert.False(t, suite.db.Migrator().HasTable("target_test2"))
	assert.False(t, suite.db.Migrator().HasTable("target_test3"))
}

func (suite *MigrationSystemTestSuite) TestSQLStatementSplitting() {
	t := suite.T()

	sql := `CREATE TABLE test1 (id SERIAL);
	CREATE INDEX idx_test1 ON test1(id);
	CREATE TABLE test2 (id SERIAL);`

	statements := suite.migrationSystem.splitSQLStatements(sql)
	
	assert.Len(t, statements, 3)
	assert.Contains(t, statements[0], "CREATE TABLE test1")
	assert.Contains(t, statements[1], "CREATE INDEX idx_test1")
	assert.Contains(t, statements[2], "CREATE TABLE test2")
}

func (suite *MigrationSystemTestSuite) TestChecksumCalculation() {
	t := suite.T()

	migration := &MigrationDefinition{
		Version:     "20240101000001",
		Name:        "test migration",
		Description: "Test checksum",
	}

	checksum1 := suite.migrationSystem.calculateChecksum(migration)
	assert.NotEmpty(t, checksum1)

	// Same migration should have same checksum
	checksum2 := suite.migrationSystem.calculateChecksum(migration)
	assert.Equal(t, checksum1, checksum2)

	// Different migration should have different checksum
	migration.Description = "Different description"
	checksum3 := suite.migrationSystem.calculateChecksum(migration)
	assert.NotEqual(t, checksum1, checksum3)
}

// Benchmark tests for migration system performance
func BenchmarkMigrationUp(b *testing.B) {
	db, err := SetupTestDB("migration_benchmark")
	if err != nil {
		b.Fatal(err)
	}
	defer CleanupTestDB(db, "migration_benchmark")

	tempDir, err := os.MkdirTemp("", "bench_migrations")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	migrationSystem := NewMigrationSystem(db, tempDir)
	if err := migrationSystem.Initialize(); err != nil {
		b.Fatal(err)
	}

	// Register a simple migration
	migration := &MigrationDefinition{
		Version: "20240101000001",
		Name:    "benchmark migration",
		Up: func(db *gorm.DB) error {
			return db.Exec("CREATE TABLE IF NOT EXISTS benchmark_test (id SERIAL PRIMARY KEY)").Error
		},
		Down: func(db *gorm.DB) error {
			return db.Exec("DROP TABLE IF EXISTS benchmark_test").Error
		},
	}

	if err := migrationSystem.RegisterMigration(migration); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clean up before each iteration
		db.Exec("DELETE FROM schema_migrations")
		db.Exec("DROP TABLE IF EXISTS benchmark_test")

		if err := migrationSystem.Up(""); err != nil {
			b.Fatal(err)
		}
	}
}

// Helper functions for testing
func SetupTestDB(dbName string) (*gorm.DB, error) {
	// This would typically set up a test database
	// For now, return a mock or use the existing database setup
	return GetDB(), nil
}

func CleanupTestDB(db *gorm.DB, dbName string) {
	// Clean up test database
	if db != nil {
		// Clean up tables created during tests
		db.Exec("DROP TABLE IF EXISTS migration_test")
		db.Exec("DROP TABLE IF EXISTS migration_test_rollback")
		db.Exec("DROP TABLE IF EXISTS batch_test1")
		db.Exec("DROP TABLE IF EXISTS batch_test2")
		db.Exec("DROP TABLE IF EXISTS target_test1")
		db.Exec("DROP TABLE IF EXISTS target_test2")
		db.Exec("DROP TABLE IF EXISTS target_test3")
		db.Exec("DROP TABLE IF EXISTS benchmark_test")
	}
}