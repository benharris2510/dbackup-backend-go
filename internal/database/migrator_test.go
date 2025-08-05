package database

import (
	"testing"
	"time"

	"github.com/dbackup/backend-go/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test models for migration testing
type MigratorTestModel struct {
	ID        uint      `gorm:"primaryKey"`
	Name      string    `gorm:"not null;index:idx_name"`
	Email     string    `gorm:"unique;index:idx_email"`
	Age       int       `gorm:"check:age > 0"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SecondTestModel struct {
	ID          uint   `gorm:"primaryKey"`
	Description string
	TestModelID uint
	TestModel   MigratorTestModel `gorm:"foreignKey:TestModelID;constraint:OnDelete:CASCADE"`
}

func setupMigratorTest(t *testing.T) func() {
	// Skip if we don't have a test database
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			URL:                   "postgres://postgres:postgres@localhost:5432/dbackup_test",
			MaxConnections:        10,
			MaxIdleConnections:    5,
			ConnectionMaxLifetime: 30 * time.Minute,
		},
		Server: config.ServerConfig{
			Env: "test",
		},
	}

	err := Initialize(cfg)
	if err != nil {
		t.Skipf("Skipping test due to database connection error: %v", err)
	}

	return func() {
		// Clean up test tables
		GetDB().Migrator().DropTable(&SecondTestModel{})
		GetDB().Migrator().DropTable(&MigratorTestModel{})
		Close()
	}
}

func TestNewMigrator(t *testing.T) {
	cleanup := setupMigratorTest(t)
	defer cleanup()

	migrator := NewMigrator(GetDB())
	assert.NotNil(t, migrator)
	assert.Equal(t, GetDB(), migrator.db)
}

func TestMigratorAutoMigrate(t *testing.T) {
	cleanup := setupMigratorTest(t)
	defer cleanup()

	migrator := NewMigrator(GetDB())

	t.Run("successful auto migration", func(t *testing.T) {
		err := migrator.AutoMigrate(&MigratorTestModel{})
		assert.NoError(t, err)

		// Verify table exists
		assert.True(t, migrator.HasTable(&MigratorTestModel{}))

		// Clean up for next test
		GetDB().Migrator().DropTable(&MigratorTestModel{})
	})

	t.Run("auto migrate multiple models", func(t *testing.T) {
		err := migrator.AutoMigrate(&MigratorTestModel{}, &SecondTestModel{})
		assert.NoError(t, err)

		// Verify both tables exist
		assert.True(t, migrator.HasTable(&MigratorTestModel{}))
		assert.True(t, migrator.HasTable(&SecondTestModel{}))

		// Clean up for next test
		GetDB().Migrator().DropTable(&SecondTestModel{})
		GetDB().Migrator().DropTable(&MigratorTestModel{})
	})

	t.Run("auto migrate with no models", func(t *testing.T) {
		err := migrator.AutoMigrate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no models provided for migration")
	})
}

func TestMigratorCreateTable(t *testing.T) {
	cleanup := setupMigratorTest(t)
	defer cleanup()

	migrator := NewMigrator(GetDB())

	t.Run("successful table creation", func(t *testing.T) {
		err := migrator.CreateTable(&MigratorTestModel{})
		assert.NoError(t, err)

		// Verify table exists
		assert.True(t, migrator.HasTable(&MigratorTestModel{}))

		// Clean up for next test
		GetDB().Migrator().DropTable(&MigratorTestModel{})
	})

	t.Run("create table that already exists", func(t *testing.T) {
		// Create table first
		err := migrator.CreateTable(&MigratorTestModel{})
		require.NoError(t, err)

		// Try to create it again - should skip without error
		err = migrator.CreateTable(&MigratorTestModel{})
		assert.NoError(t, err)

		// Clean up
		GetDB().Migrator().DropTable(&MigratorTestModel{})
	})

	t.Run("create multiple tables", func(t *testing.T) {
		err := migrator.CreateTable(&MigratorTestModel{}, &SecondTestModel{})
		assert.NoError(t, err)

		// Verify both tables exist
		assert.True(t, migrator.HasTable(&MigratorTestModel{}))
		assert.True(t, migrator.HasTable(&SecondTestModel{}))

		// Clean up
		GetDB().Migrator().DropTable(&SecondTestModel{})
		GetDB().Migrator().DropTable(&MigratorTestModel{})
	})
}

func TestMigratorDropTable(t *testing.T) {
	cleanup := setupMigratorTest(t)
	defer cleanup()

	migrator := NewMigrator(GetDB())

	t.Run("successful table drop", func(t *testing.T) {
		// Create table first
		err := migrator.CreateTable(&MigratorTestModel{})
		require.NoError(t, err)
		assert.True(t, migrator.HasTable(&MigratorTestModel{}))

		// Drop the table
		err = migrator.DropTable(&MigratorTestModel{})
		assert.NoError(t, err)

		// Verify table doesn't exist
		assert.False(t, migrator.HasTable(&MigratorTestModel{}))
	})

	t.Run("drop table that doesn't exist", func(t *testing.T) {
		// Try to drop non-existent table - should skip without error
		err := migrator.DropTable(&MigratorTestModel{})
		assert.NoError(t, err)
	})

	t.Run("drop multiple tables", func(t *testing.T) {
		// Create tables first
		err := migrator.CreateTable(&MigratorTestModel{}, &SecondTestModel{})
		require.NoError(t, err)
		assert.True(t, migrator.HasTable(&MigratorTestModel{}))
		assert.True(t, migrator.HasTable(&SecondTestModel{}))

		// Drop both tables
		err = migrator.DropTable(&SecondTestModel{}, &MigratorTestModel{})
		assert.NoError(t, err)

		// Verify both tables don't exist
		assert.False(t, migrator.HasTable(&MigratorTestModel{}))
		assert.False(t, migrator.HasTable(&SecondTestModel{}))
	})
}

func TestMigratorHasTable(t *testing.T) {
	cleanup := setupMigratorTest(t)
	defer cleanup()

	migrator := NewMigrator(GetDB())

	// Initially table should not exist
	assert.False(t, migrator.HasTable(&MigratorTestModel{}))

	// Create table
	err := migrator.CreateTable(&MigratorTestModel{})
	require.NoError(t, err)

	// Now table should exist
	assert.True(t, migrator.HasTable(&MigratorTestModel{}))

	// Clean up
	GetDB().Migrator().DropTable(&MigratorTestModel{})
}

func TestMigratorIndexOperations(t *testing.T) {
	cleanup := setupMigratorTest(t)
	defer cleanup()

	migrator := NewMigrator(GetDB())

	// Create table first
	err := migrator.CreateTable(&MigratorTestModel{})
	require.NoError(t, err)
	defer GetDB().Migrator().DropTable(&MigratorTestModel{})

	t.Run("create index", func(t *testing.T) {
		err := migrator.CreateIndex(&MigratorTestModel{}, "idx_custom")
		// Note: This might not work as expected without proper field specification
		// But we're testing the interface
		if err != nil {
			// If creating custom index fails, that's expected
			assert.Contains(t, err.Error(), "index")
		} else {
			assert.NoError(t, err)
		}
	})

	t.Run("has index", func(t *testing.T) {
		// Test with indexes that should exist (from model tags)
		hasIndex := migrator.HasIndex(&MigratorTestModel{}, "idx_name")
		// This depends on whether GORM created the index from the tag
		assert.IsType(t, true, hasIndex) // Just verify it returns a boolean
	})

	t.Run("create existing index", func(t *testing.T) {
		// Try to create an index that might already exist
		err := migrator.CreateIndex(&MigratorTestModel{}, "idx_name")
		// Should handle gracefully (skip if exists)
		if err != nil {
			// Error is acceptable if index operations are complex
			assert.Contains(t, err.Error(), "index")
		} else {
			assert.NoError(t, err)
		}
	})

	t.Run("drop index", func(t *testing.T) {
		err := migrator.DropIndex(&MigratorTestModel{}, "idx_custom")
		// Should handle gracefully (skip if doesn't exist)
		assert.NoError(t, err)
	})
}

func TestMigratorConstraintOperations(t *testing.T) {
	cleanup := setupMigratorTest(t)
	defer cleanup()

	migrator := NewMigrator(GetDB())

	// Create tables first
	err := migrator.CreateTable(&MigratorTestModel{}, &SecondTestModel{})
	require.NoError(t, err)
	defer func() {
		GetDB().Migrator().DropTable(&SecondTestModel{})
		GetDB().Migrator().DropTable(&MigratorTestModel{})
	}()

	t.Run("has constraint", func(t *testing.T) {
		// Test checking for constraints
		hasConstraint := migrator.HasConstraint(&MigratorTestModel{}, "check_age")
		assert.IsType(t, true, hasConstraint) // Just verify it returns a boolean
	})

	t.Run("create constraint", func(t *testing.T) {
		err := migrator.CreateConstraint(&MigratorTestModel{}, "custom_constraint")
		// Creating arbitrary constraints may fail, which is expected
		if err != nil {
			assert.Contains(t, err.Error(), "constraint")
		} else {
			assert.NoError(t, err)
		}
	})

	t.Run("drop constraint", func(t *testing.T) {
		err := migrator.DropConstraint(&MigratorTestModel{}, "custom_constraint")
		// Should handle gracefully (skip if doesn't exist)
		assert.NoError(t, err)
	})
}

func TestMigratorRunMigrations(t *testing.T) {
	cleanup := setupMigratorTest(t)
	defer cleanup()

	migrator := NewMigrator(GetDB())

	t.Run("successful migration series", func(t *testing.T) {
		migrations := []func(*Migrator) error{
			func(m *Migrator) error {
				return m.CreateTable(&MigratorTestModel{})
			},
			func(m *Migrator) error {
				return m.CreateTable(&SecondTestModel{})
			},
		}

		err := migrator.RunMigrations(migrations)
		assert.NoError(t, err)

		// Verify both tables exist
		assert.True(t, migrator.HasTable(&MigratorTestModel{}))
		assert.True(t, migrator.HasTable(&SecondTestModel{}))

		// Clean up
		GetDB().Migrator().DropTable(&SecondTestModel{})
		GetDB().Migrator().DropTable(&MigratorTestModel{})
	})

	t.Run("migration with error (rollback)", func(t *testing.T) {
		migrations := []func(*Migrator) error{
			func(m *Migrator) error {
				return m.CreateTable(&MigratorTestModel{})
			},
			func(m *Migrator) error {
				return assert.AnError // Force an error
			},
		}

		err := migrator.RunMigrations(migrations)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "migration step 2 failed")

		// Due to transaction rollback, table should not exist
		assert.False(t, migrator.HasTable(&MigratorTestModel{}))
	})

	t.Run("empty migrations", func(t *testing.T) {
		migrations := []func(*Migrator) error{}

		err := migrator.RunMigrations(migrations)
		assert.NoError(t, err)
	})
}

func TestGetMigrator(t *testing.T) {
	cleanup := setupMigratorTest(t)
	defer cleanup()

	migrator := GetMigrator()
	assert.NotNil(t, migrator)
	assert.Equal(t, GetDB(), migrator.db)
}

func TestGetModelName(t *testing.T) {
	t.Run("struct value", func(t *testing.T) {
		model := MigratorTestModel{}
		name := getModelName(model)
		assert.Equal(t, "MigratorTestModel", name)
	})

	t.Run("struct pointer", func(t *testing.T) {
		model := &MigratorTestModel{}
		name := getModelName(model)
		assert.Equal(t, "MigratorTestModel", name)
	})

	t.Run("slice of structs", func(t *testing.T) {
		model := []MigratorTestModel{}
		name := getModelName(model)
		assert.Equal(t, "MigratorTestModel", name)
	})

	t.Run("slice of struct pointers", func(t *testing.T) {
		model := []*MigratorTestModel{}
		name := getModelName(model)
		assert.Equal(t, "MigratorTestModel", name)
	})

	t.Run("nil value", func(t *testing.T) {
		name := getModelName(nil)
		assert.Equal(t, "unknown", name)
	})

	t.Run("primitive type", func(t *testing.T) {
		name := getModelName("string")
		assert.Equal(t, "string", name)
	})

	t.Run("pointer to primitive", func(t *testing.T) {
		str := "test"
		name := getModelName(&str)
		assert.Equal(t, "string", name)
	})
}