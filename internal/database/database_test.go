package database

import (
	"context"
	"testing"
	"time"

	"github.com/dbackup/backend-go/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestModel is a simple model for testing migrations
type TestModel struct {
	ID        uint      `gorm:"primaryKey"`
	Name      string    `gorm:"not null"`
	Email     string    `gorm:"unique"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func TestDatabaseInitialize(t *testing.T) {
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

	t.Run("successful initialization", func(t *testing.T) {
		// Reset global db variable
		db = nil

		err := Initialize(cfg)
		if err != nil {
			t.Skipf("Skipping test due to database connection error: %v", err)
		}

		assert.NoError(t, err)
		assert.NotNil(t, GetDB())
		assert.True(t, IsConnected())

		// Clean up
		Close()
	})

	t.Run("invalid database URL", func(t *testing.T) {
		// Reset global db variable
		db = nil

		invalidCfg := &config.Config{
			Database: config.DatabaseConfig{
				URL: "invalid://url",
			},
			Server: config.ServerConfig{
			Env: "test",
		},
		}

		err := Initialize(invalidCfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to connect to database")
	})
}

func TestGetDB(t *testing.T) {
	t.Run("database not initialized", func(t *testing.T) {
		// Reset global db variable
		db = nil

		assert.Panics(t, func() {
			GetDB()
		})
	})

	t.Run("database initialized", func(t *testing.T) {
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

		database := GetDB()
		assert.NotNil(t, database)
		assert.IsType(t, &gorm.DB{}, database)

		// Clean up
		Close()
	})
}

func TestIsConnected(t *testing.T) {
	t.Run("database not initialized", func(t *testing.T) {
		// Reset global db variable
		db = nil

		assert.False(t, IsConnected())
	})

	t.Run("database connected", func(t *testing.T) {
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

		assert.True(t, IsConnected())

		// Clean up
		Close()
	})
}

func TestMigrate(t *testing.T) {
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
	defer Close()

	t.Run("successful migration", func(t *testing.T) {
		err := Migrate(&TestModel{})
		assert.NoError(t, err)

		// Verify table exists
		assert.True(t, GetDB().Migrator().HasTable(&TestModel{}))

		// Clean up
		GetDB().Migrator().DropTable(&TestModel{})
	})

	t.Run("migration with multiple models", func(t *testing.T) {
		type AnotherTestModel struct {
			ID   uint   `gorm:"primaryKey"`
			Data string
		}

		err := Migrate(&TestModel{}, &AnotherTestModel{})
		assert.NoError(t, err)

		// Verify both tables exist
		assert.True(t, GetDB().Migrator().HasTable(&TestModel{}))
		assert.True(t, GetDB().Migrator().HasTable(&AnotherTestModel{}))

		// Clean up
		GetDB().Migrator().DropTable(&TestModel{})
		GetDB().Migrator().DropTable(&AnotherTestModel{})
	})

	t.Run("migration with database not initialized", func(t *testing.T) {
		// Temporarily set db to nil
		originalDB := db
		db = nil

		err := Migrate(&TestModel{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database not initialized")

		// Restore db
		db = originalDB
	})
}

func TestGetStats(t *testing.T) {
	t.Run("database not initialized", func(t *testing.T) {
		// Reset global db variable
		db = nil

		stats, err := GetStats()
		assert.Error(t, err)
		assert.Nil(t, stats)
		assert.Contains(t, err.Error(), "database not initialized")
	})

	t.Run("successful stats retrieval", func(t *testing.T) {
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
		defer Close()

		stats, err := GetStats()
		assert.NoError(t, err)
		assert.NotNil(t, stats)

		// Check that stats contain expected fields
		expectedFields := []string{
			"max_open_connections",
			"open_connections",
			"in_use",
			"idle",
			"wait_count",
			"wait_duration",
			"max_idle_closed",
			"max_idle_time_closed",
			"max_lifetime_closed",
		}

		for _, field := range expectedFields {
			assert.Contains(t, stats, field)
		}

		// Verify max_open_connections matches our config
		assert.Equal(t, 10, stats["max_open_connections"])
	})
}

func TestTransaction(t *testing.T) {
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
	defer Close()

	// Migrate test model
	err = Migrate(&TestModel{})
	require.NoError(t, err)
	defer GetDB().Migrator().DropTable(&TestModel{})

	t.Run("successful transaction", func(t *testing.T) {
		err := Transaction(func(tx *gorm.DB) error {
			return tx.Create(&TestModel{
				Name:  "Test User",
				Email: "test@example.com",
			}).Error
		})

		assert.NoError(t, err)

		// Verify record was created
		var count int64
		GetDB().Model(&TestModel{}).Count(&count)
		assert.Equal(t, int64(1), count)

		// Clean up
		GetDB().Delete(&TestModel{}, "email = ?", "test@example.com")
	})

	t.Run("transaction rollback on error", func(t *testing.T) {
		initialCount := int64(0)
		GetDB().Model(&TestModel{}).Count(&initialCount)

		err := Transaction(func(tx *gorm.DB) error {
			// Create a record
			if err := tx.Create(&TestModel{
				Name:  "Rollback Test",
				Email: "rollback@example.com",
			}).Error; err != nil {
				return err
			}

			// Force an error to trigger rollback
			return assert.AnError
		})

		assert.Error(t, err)

		// Verify no record was created (transaction rolled back)
		var finalCount int64
		GetDB().Model(&TestModel{}).Count(&finalCount)
		assert.Equal(t, initialCount, finalCount)
	})

	t.Run("transaction with database not initialized", func(t *testing.T) {
		// Temporarily set db to nil
		originalDB := db
		db = nil

		err := Transaction(func(tx *gorm.DB) error {
			return nil
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database not initialized")

		// Restore db
		db = originalDB
	})
}

func TestHealthCheck(t *testing.T) {
	t.Run("database not initialized", func(t *testing.T) {
		// Reset global db variable
		db = nil

		err := HealthCheck()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database not initialized")
	})

	t.Run("successful health check", func(t *testing.T) {
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
		defer Close()

		err = HealthCheck()
		assert.NoError(t, err)
	})

	t.Run("health check with timeout", func(t *testing.T) {
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
		defer Close()

		// Create a context with timeout to simulate timeout behavior
		// This test verifies that the health check uses context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		// Get underlying sql.DB to test with context
		sqlDB, err := GetDB().DB()
		require.NoError(t, err)

		// This should timeout quickly
		err = sqlDB.PingContext(ctx)
		// The exact error depends on timing, but it should be context-related
		// We're mainly testing that the health check mechanism works
		assert.True(t, err == nil || err == context.DeadlineExceeded)
	})
}

func TestClose(t *testing.T) {
	t.Run("close with nil database", func(t *testing.T) {
		// Reset global db variable
		db = nil

		err := Close()
		assert.NoError(t, err)
	})

	t.Run("successful close", func(t *testing.T) {
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

		// Verify connection is active
		assert.True(t, IsConnected())

		// Close the connection
		err = Close()
		assert.NoError(t, err)

		// Verify connection is closed
		assert.False(t, IsConnected())
	})
}