package models

import (
	"testing"
	"time"

	"github.com/dbackup/backend-go/internal/encryption"
	"github.com/dbackup/backend-go/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabaseType_GetDefaultPort(t *testing.T) {
	tests := []struct {
		dbType   models.DatabaseType
		expected int
	}{
		{models.DatabaseTypePostgreSQL, 5432},
		{models.DatabaseTypeMySQL, 3306},
		{models.DatabaseTypeMongoDB, 27017},
		{models.DatabaseTypeRedis, 6379},
		{models.DatabaseTypeSQLServer, 1433},
		{models.DatabaseTypeOracle, 1521},
		{models.DatabaseTypeSQLite, 3306}, // Default fallback
	}

	for _, tt := range tests {
		t.Run(string(tt.dbType), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.dbType.GetDefaultPort())
		})
	}
}

func TestDatabaseType_IsValid(t *testing.T) {
	tests := []struct {
		dbType   models.DatabaseType
		expected bool
	}{
		{models.DatabaseTypePostgreSQL, true},
		{models.DatabaseTypeMySQL, true},
		{models.DatabaseTypeMongoDB, true},
		{models.DatabaseTypeRedis, true},
		{models.DatabaseTypeSQLServer, true},
		{models.DatabaseTypeOracle, true},
		{models.DatabaseTypeSQLite, true},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.dbType), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.dbType.IsValid())
		})
	}
}

func TestDatabaseType_GetDisplayName(t *testing.T) {
	tests := []struct {
		dbType   models.DatabaseType
		expected string
	}{
		{models.DatabaseTypePostgreSQL, "PostgreSQL"},
		{models.DatabaseTypeMySQL, "MySQL"},
		{models.DatabaseTypeMongoDB, "MongoDB"},
		{models.DatabaseTypeRedis, "Redis"},
		{models.DatabaseTypeSQLServer, "SQL Server"},
		{models.DatabaseTypeOracle, "Oracle"},
		{models.DatabaseTypeSQLite, "SQLite"},
		{"custom", "custom"},
	}

	for _, tt := range tests {
		t.Run(string(tt.dbType), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.dbType.GetDisplayName())
		})
	}
}

func TestDatabaseConnection_GetConnectionString(t *testing.T) {
	tests := []struct {
		name       string
		connection *models.DatabaseConnection
		expected   string
	}{
		{
			name: "PostgreSQL without SSL",
			connection: &models.DatabaseConnection{
				Type:     models.DatabaseTypePostgreSQL,
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
				Username: "testuser",
			},
			expected: "postgres://testuser:***@localhost:5432/testdb?sslmode=disable",
		},
		{
			name: "PostgreSQL with SSL",
			connection: &models.DatabaseConnection{
				Type:       models.DatabaseTypePostgreSQL,
				Host:       "localhost",
				Port:       5432,
				Database:   "testdb",
				Username:   "testuser",
				SSLEnabled: true,
				SSLMode:    stringPtr("require"),
			},
			expected: "postgres://testuser:***@localhost:5432/testdb?sslmode=require",
		},
		{
			name: "MySQL without SSL",
			connection: &models.DatabaseConnection{
				Type:     models.DatabaseTypeMySQL,
				Host:     "localhost",
				Port:     3306,
				Database: "testdb",
				Username: "testuser",
			},
			expected: "mysql://testuser:***@localhost:3306/testdb?tls=false",
		},
		{
			name: "MySQL with SSL",
			connection: &models.DatabaseConnection{
				Type:       models.DatabaseTypeMySQL,
				Host:       "localhost",
				Port:       3306,
				Database:   "testdb",
				Username:   "testuser",
				SSLEnabled: true,
			},
			expected: "mysql://testuser:***@localhost:3306/testdb?tls=true",
		},
		{
			name: "MongoDB",
			connection: &models.DatabaseConnection{
				Type:     models.DatabaseTypeMongoDB,
				Host:     "localhost",
				Port:     27017,
				Database: "testdb",
				Username: "testuser",
			},
			expected: "mongodb://testuser:***@localhost:27017/testdb?ssl=false",
		},
		{
			name: "Custom database type",
			connection: &models.DatabaseConnection{
				Type:     "custom",
				Host:     "localhost",
				Port:     1234,
				Database: "testdb",
				Username: "testuser",
			},
			expected: "custom://testuser:***@localhost:1234/testdb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.connection.GetConnectionString()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDatabaseConnection_GetFullConnectionString(t *testing.T) {
	connection := &models.DatabaseConnection{
		Type:     models.DatabaseTypePostgreSQL,
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		Username: "testuser",
	}

	password := "secret123"
	expected := "postgres://testuser:secret123@localhost:5432/testdb?sslmode=disable"
	
	result := connection.GetFullConnectionString(password)
	assert.Equal(t, expected, result)
}

func TestDatabaseConnection_IsHealthy(t *testing.T) {
	tests := []struct {
		name       string
		connection *models.DatabaseConnection
		expected   bool
	}{
		{
			name: "healthy connection",
			connection: &models.DatabaseConnection{
				IsActive:      true,
				LastTestError: nil,
			},
			expected: true,
		},
		{
			name: "inactive connection",
			connection: &models.DatabaseConnection{
				IsActive:      false,
				LastTestError: nil,
			},
			expected: false,
		},
		{
			name: "connection with error",
			connection: &models.DatabaseConnection{
				IsActive:      true,
				LastTestError: stringPtr("connection timeout"),
			},
			expected: false,
		},
		{
			name: "inactive connection with error",
			connection: &models.DatabaseConnection{
				IsActive:      false,
				LastTestError: stringPtr("connection timeout"),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.connection.IsHealthy())
		})
	}
}

func TestDatabaseConnection_NeedsRetesting(t *testing.T) {
	tests := []struct {
		name         string
		lastTestedAt *time.Time
		expected     bool
	}{
		{
			name:         "never tested",
			lastTestedAt: nil,
			expected:     true,
		},
		{
			name:         "tested recently",
			lastTestedAt: timePtr(time.Now().Add(-30 * time.Minute)),
			expected:     false,
		},
		{
			name:         "tested over an hour ago",
			lastTestedAt: timePtr(time.Now().Add(-2 * time.Hour)),
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connection := &models.DatabaseConnection{
				LastTestedAt: tt.lastTestedAt,
			}
			assert.Equal(t, tt.expected, connection.NeedsRetesting())
		})
	}
}

func TestDatabaseConnection_SetTestResult(t *testing.T) {
	connection := &models.DatabaseConnection{}

	t.Run("successful test", func(t *testing.T) {
		connection.SetTestResult(true, "")
		
		assert.NotNil(t, connection.LastTestedAt)
		assert.Nil(t, connection.LastTestError)
		assert.WithinDuration(t, time.Now(), *connection.LastTestedAt, time.Second)
	})

	t.Run("failed test", func(t *testing.T) {
		errorMsg := "connection timeout"
		connection.SetTestResult(false, errorMsg)
		
		assert.NotNil(t, connection.LastTestedAt)
		assert.NotNil(t, connection.LastTestError)
		assert.Equal(t, errorMsg, *connection.LastTestError)
		assert.WithinDuration(t, time.Now(), *connection.LastTestedAt, time.Second)
	})
}

func TestDatabaseConnection_GetTableCount(t *testing.T) {
	connection := &models.DatabaseConnection{}
	assert.Equal(t, 0, connection.GetTableCount())

	// Simulate having tables (can't actually create them without full model setup)
	// This tests the basic functionality
}

func TestDatabaseConnection_GetActiveBackupJobsCount(t *testing.T) {
	connection := &models.DatabaseConnection{}
	assert.Equal(t, 0, connection.GetActiveBackupJobsCount())

	// Simulate having backup jobs (can't actually create them without full model setup)
	// This tests the basic functionality
}

func TestDatabaseConnection_CanCreateBackup(t *testing.T) {
	tests := []struct {
		name       string
		connection *models.DatabaseConnection
		expected   bool
	}{
		{
			name: "can create backup",
			connection: &models.DatabaseConnection{
				IsActive:      true,
				LastTestError: nil,
			},
			expected: true,
		},
		{
			name: "inactive connection",
			connection: &models.DatabaseConnection{
				IsActive:      false,
				LastTestError: nil,
			},
			expected: false,
		},
		{
			name: "unhealthy connection",
			connection: &models.DatabaseConnection{
				IsActive:      true,
				LastTestError: stringPtr("error"),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.connection.CanCreateBackup())
		})
	}
}

func TestDatabaseConnection_EncryptCredentials(t *testing.T) {
	encService := encryption.NewService("test-encryption-key-for-testing")

	t.Run("encrypt password", func(t *testing.T) {
		connection := &models.DatabaseConnection{
			Password: "plain-password",
		}

		err := connection.EncryptCredentials(encService)
		require.NoError(t, err)

		assert.NotEqual(t, "plain-password", connection.Password)
		assert.NotEmpty(t, connection.Password)

		// Verify we can decrypt it back
		decrypted, err := encService.Decrypt(connection.Password)
		require.NoError(t, err)
		assert.Equal(t, "plain-password", decrypted)
	})

	t.Run("encrypt SSL certificates", func(t *testing.T) {
		sslCert := "test-ssl-cert-data"
		sslKey := "test-ssl-key-data"
		sslRootCert := "test-ssl-root-cert-data"

		// Make copies for the connection to avoid modifying our comparison values
		sslCertCopy := sslCert
		sslKeyCopy := sslKey
		sslRootCertCopy := sslRootCert

		connection := &models.DatabaseConnection{
			Password:    "plain-password",
			SSLCert:     &sslCertCopy,
			SSLKey:      &sslKeyCopy,
			SSLRootCert: &sslRootCertCopy,
		}

		err := connection.EncryptCredentials(encService)
		require.NoError(t, err)

		// All fields should be encrypted
		assert.NotEqual(t, "plain-password", connection.Password)
		assert.NotEqual(t, sslCert, *connection.SSLCert)
		assert.NotEqual(t, sslKey, *connection.SSLKey)
		assert.NotEqual(t, sslRootCert, *connection.SSLRootCert)

		// Verify we can decrypt them back to original values
		decryptedPassword, err := encService.Decrypt(connection.Password)
		require.NoError(t, err)
		assert.Equal(t, "plain-password", decryptedPassword)

		decryptedCert, err := encService.Decrypt(*connection.SSLCert)
		require.NoError(t, err)
		assert.Equal(t, sslCert, decryptedCert)

		decryptedKey, err := encService.Decrypt(*connection.SSLKey)
		require.NoError(t, err)
		assert.Equal(t, sslKey, decryptedKey)

		decryptedRootCert, err := encService.Decrypt(*connection.SSLRootCert)
		require.NoError(t, err)
		assert.Equal(t, sslRootCert, decryptedRootCert)
	})

	t.Run("skip empty fields", func(t *testing.T) {
		connection := &models.DatabaseConnection{
			Password: "",
			SSLCert:  stringPtr(""),
		}

		err := connection.EncryptCredentials(encService)
		require.NoError(t, err)

		assert.Equal(t, "", connection.Password)
		assert.Equal(t, "", *connection.SSLCert)
	})

	t.Run("nil encryption service", func(t *testing.T) {
		connection := &models.DatabaseConnection{
			Password: "test",
		}

		err := connection.EncryptCredentials(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "encryption service is required")
	})
}

func TestDatabaseConnection_DecryptCredentials(t *testing.T) {
	encService := encryption.NewService("test-encryption-key-for-testing")

	t.Run("decrypt password", func(t *testing.T) {
		originalPassword := "plain-password"
		encrypted, err := encService.Encrypt(originalPassword)
		require.NoError(t, err)

		connection := &models.DatabaseConnection{
			Password: encrypted,
		}

		err = connection.DecryptCredentials(encService)
		require.NoError(t, err)
		assert.Equal(t, originalPassword, connection.Password)
	})

	t.Run("decrypt SSL certificates", func(t *testing.T) {
		originalCert := "test-ssl-cert-data"
		originalKey := "test-ssl-key-data"
		originalRootCert := "test-ssl-root-cert-data"

		encryptedCert, err := encService.Encrypt(originalCert)
		require.NoError(t, err)
		encryptedKey, err := encService.Encrypt(originalKey)
		require.NoError(t, err)
		encryptedRootCert, err := encService.Encrypt(originalRootCert)
		require.NoError(t, err)

		connection := &models.DatabaseConnection{
			SSLCert:     &encryptedCert,
			SSLKey:      &encryptedKey,
			SSLRootCert: &encryptedRootCert,
		}

		err = connection.DecryptCredentials(encService)
		require.NoError(t, err)

		assert.Equal(t, originalCert, *connection.SSLCert)
		assert.Equal(t, originalKey, *connection.SSLKey)
		assert.Equal(t, originalRootCert, *connection.SSLRootCert)
	})

	t.Run("skip empty fields", func(t *testing.T) {
		connection := &models.DatabaseConnection{
			Password: "",
			SSLCert:  stringPtr(""),
		}

		err := connection.DecryptCredentials(encService)
		require.NoError(t, err)

		assert.Equal(t, "", connection.Password)
		assert.Equal(t, "", *connection.SSLCert)
	})

	t.Run("nil encryption service", func(t *testing.T) {
		connection := &models.DatabaseConnection{
			Password: "encrypted",
		}

		err := connection.DecryptCredentials(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "encryption service is required")
	})

	t.Run("invalid encrypted data", func(t *testing.T) {
		connection := &models.DatabaseConnection{
			Password: "invalid-encrypted-data",
		}

		err := connection.DecryptCredentials(encService)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decrypt password")
	})
}

func TestDatabaseConnection_GetDecryptedPassword(t *testing.T) {
	encService := encryption.NewService("test-encryption-key-for-testing")

	t.Run("get decrypted password", func(t *testing.T) {
		originalPassword := "plain-password"
		encrypted, err := encService.Encrypt(originalPassword)
		require.NoError(t, err)

		connection := &models.DatabaseConnection{
			Password: encrypted,
		}

		decrypted, err := connection.GetDecryptedPassword(encService)
		require.NoError(t, err)
		assert.Equal(t, originalPassword, decrypted)
	})

	t.Run("empty password", func(t *testing.T) {
		connection := &models.DatabaseConnection{
			Password: "",
		}

		decrypted, err := connection.GetDecryptedPassword(encService)
		require.NoError(t, err)
		assert.Equal(t, "", decrypted)
	})

	t.Run("nil encryption service", func(t *testing.T) {
		connection := &models.DatabaseConnection{
			Password: "encrypted",
		}

		_, err := connection.GetDecryptedPassword(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "encryption service is required")
	})
}

func TestDatabaseConnection_ValidateConnection(t *testing.T) {
	tests := []struct {
		name       string
		connection *models.DatabaseConnection
		wantErr    bool
		errMsg     string
	}{
		{
			name: "valid PostgreSQL connection",
			connection: &models.DatabaseConnection{
				Name:     "test-connection",
				Type:     models.DatabaseTypePostgreSQL,
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
				Username: "testuser",
				Password: "testpass",
			},
			wantErr: false,
		},
		{
			name: "valid SQLite connection",
			connection: &models.DatabaseConnection{
				Name:     "test-sqlite",
				Type:     models.DatabaseTypeSQLite,
				Database: "/path/to/db.sqlite",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			connection: &models.DatabaseConnection{
				Type:     models.DatabaseTypePostgreSQL,
				Host:     "localhost",
				Database: "testdb",
			},
			wantErr: true,
			errMsg:  "connection name is required",
		},
		{
			name: "invalid database type",
			connection: &models.DatabaseConnection{
				Name: "test",
				Type: "invalid",
				Host: "localhost",
			},
			wantErr: true,
			errMsg:  "invalid database type",
		},
		{
			name: "missing host for non-SQLite",
			connection: &models.DatabaseConnection{
				Name:     "test",
				Type:     models.DatabaseTypePostgreSQL,
				Database: "testdb",
			},
			wantErr: true,
			errMsg:  "host is required for non-SQLite databases",
		},
		{
			name: "missing database",
			connection: &models.DatabaseConnection{
				Name: "test",
				Type: models.DatabaseTypePostgreSQL,
				Host: "localhost",
			},
			wantErr: true,
			errMsg:  "database name is required",
		},
		{
			name: "missing username for non-SQLite",
			connection: &models.DatabaseConnection{
				Name:     "test",
				Type:     models.DatabaseTypePostgreSQL,
				Host:     "localhost",
				Database: "testdb",
			},
			wantErr: true,
			errMsg:  "username is required for non-SQLite databases",
		},
		{
			name: "missing password for non-SQLite",
			connection: &models.DatabaseConnection{
				Name:     "test",
				Type:     models.DatabaseTypePostgreSQL,
				Host:     "localhost",
				Database: "testdb",
				Username: "testuser",
			},
			wantErr: true,
			errMsg:  "password is required for non-SQLite databases",
		},
		{
			name: "invalid port - too low",
			connection: &models.DatabaseConnection{
				Name:     "test",
				Type:     models.DatabaseTypePostgreSQL,
				Host:     "localhost",
				Port:     0,
				Database: "testdb",
				Username: "testuser",
				Password: "testpass",
			},
			wantErr: true,
			errMsg:  "port must be between 1 and 65535",
		},
		{
			name: "invalid port - too high",
			connection: &models.DatabaseConnection{
				Name:     "test",
				Type:     models.DatabaseTypePostgreSQL,
				Host:     "localhost",
				Port:     70000,
				Database: "testdb",
				Username: "testuser",
				Password: "testpass",
			},
			wantErr: true,
			errMsg:  "port must be between 1 and 65535",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.connection.ValidateConnection()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDatabaseConnection_SetDefaultValues(t *testing.T) {
	connection := &models.DatabaseConnection{
		Type: models.DatabaseTypePostgreSQL,
	}

	connection.SetDefaultValues()

	assert.Equal(t, 5432, connection.Port)
	assert.Equal(t, 10, connection.MaxConnections)
	assert.Equal(t, 30*time.Second, connection.ConnectionTimeout)
	assert.Equal(t, 5*time.Minute, connection.QueryTimeout)
}

func TestDatabaseConnection_ToPublic(t *testing.T) {
	now := time.Now()
	description := "Test connection"
	lastError := "Connection timeout"
	sslMode := "require"
	sslCert := "cert-data"
	sslKey := "key-data"
	sslRootCert := "root-cert-data"

	connection := &models.DatabaseConnection{
		ID:                1,
		UID:               "test-uid",
		Name:              "Test Connection",
		Description:       &description,
		Type:              models.DatabaseTypePostgreSQL,
		Host:              "localhost",
		Port:              5432,
		Database:          "testdb",
		Username:          "testuser",
		SSLEnabled:        true,
		SSLMode:           &sslMode,
		SSLCert:           &sslCert,
		SSLKey:            &sslKey,
		SSLRootCert:       &sslRootCert,
		MaxConnections:    20,
		ConnectionTimeout: 30 * time.Second,
		QueryTimeout:      5 * time.Minute,
		IsActive:          true,
		LastTestedAt:      &now,
		LastTestError:     &lastError,
		Tags: []models.DatabaseTag{
			{ID: 1, Name: "production", Color: "#ff0000"},
			{ID: 2, Name: "primary", Color: "#00ff00"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	public := connection.ToPublic()

	assert.Equal(t, connection.ID, public.ID)
	assert.Equal(t, connection.UID, public.UID)
	assert.Equal(t, connection.Name, public.Name)
	assert.Equal(t, description, public.Description)
	assert.Equal(t, connection.Type, public.Type)
	assert.Equal(t, connection.Host, public.Host)
	assert.Equal(t, connection.Port, public.Port)
	assert.Equal(t, connection.Database, public.Database)
	assert.Equal(t, connection.Username, public.Username)
	assert.Equal(t, connection.SSLEnabled, public.SSLEnabled)
	assert.Equal(t, sslMode, public.SSLMode)
	assert.True(t, public.HasSSLCert)
	assert.True(t, public.HasSSLKey)
	assert.True(t, public.HasSSLRootCert)
	assert.Equal(t, connection.MaxConnections, public.MaxConnections)
	assert.Equal(t, connection.ConnectionTimeout, public.ConnectionTimeout)
	assert.Equal(t, connection.QueryTimeout, public.QueryTimeout)
	assert.Equal(t, connection.IsActive, public.IsActive)
	assert.Equal(t, connection.LastTestedAt, public.LastTestedAt)
	assert.Equal(t, lastError, public.LastTestError)
	assert.Equal(t, 0, public.TableCount)
	assert.Equal(t, 0, public.BackupJobsCount)
	assert.NotEmpty(t, public.ConnectionString)
	assert.Equal(t, connection.IsHealthy(), public.IsHealthy)
	assert.Equal(t, connection.NeedsRetesting(), public.NeedsRetesting)
	assert.Equal(t, connection.CreatedAt, public.CreatedAt)
	assert.Equal(t, connection.UpdatedAt, public.UpdatedAt)

	// Check tags conversion
	assert.Len(t, public.Tags, 2)
	assert.Equal(t, uint(1), public.Tags[0].ID)
	assert.Equal(t, "production", public.Tags[0].Name)
	assert.Equal(t, "#ff0000", public.Tags[0].Color)
	assert.Equal(t, uint(2), public.Tags[1].ID)
	assert.Equal(t, "primary", public.Tags[1].Name)
	assert.Equal(t, "#00ff00", public.Tags[1].Color)

	// Verify sensitive data is not exposed
	// The actual password should never be in the public representation
}

func TestConnectionRequest_ToModel(t *testing.T) {
	request := &models.ConnectionRequest{
		Name:              "Test Connection",
		Description:       "Test description",
		Type:              models.DatabaseTypePostgreSQL,
		Host:              "localhost",
		Port:              5432,
		Database:          "testdb",
		Username:          "testuser",
		Password:          "testpass",
		SSLEnabled:        true,
		SSLMode:           "require",
		SSLCert:           "cert-data",
		SSLKey:            "key-data",
		SSLRootCert:       "root-cert-data",
		MaxConnections:    20,
		ConnectionTimeout: 30,
		QueryTimeout:      300,
		TagIDs:            []uint{1, 2},
	}

	model := request.ToModel()

	assert.Equal(t, request.Name, model.Name)
	assert.Equal(t, request.Description, *model.Description)
	assert.Equal(t, request.Type, model.Type)
	assert.Equal(t, request.Host, model.Host)
	assert.Equal(t, request.Port, model.Port)
	assert.Equal(t, request.Database, model.Database)
	assert.Equal(t, request.Username, model.Username)
	assert.Equal(t, request.Password, model.Password)
	assert.Equal(t, request.SSLEnabled, model.SSLEnabled)
	assert.Equal(t, request.SSLMode, *model.SSLMode)
	assert.Equal(t, request.SSLCert, *model.SSLCert)
	assert.Equal(t, request.SSLKey, *model.SSLKey)
	assert.Equal(t, request.SSLRootCert, *model.SSLRootCert)
	assert.Equal(t, request.MaxConnections, model.MaxConnections)
	assert.Equal(t, 30*time.Second, model.ConnectionTimeout)
	assert.Equal(t, 5*time.Minute, model.QueryTimeout)
	assert.True(t, model.IsActive)

	t.Run("empty optional fields", func(t *testing.T) {
		minimalRequest := &models.ConnectionRequest{
			Name:     "Minimal",
			Type:     models.DatabaseTypeMySQL,
			Database: "db",
		}

		minimalModel := minimalRequest.ToModel()
		assert.Nil(t, minimalModel.Description)
		assert.Nil(t, minimalModel.SSLMode)
		assert.Nil(t, minimalModel.SSLCert)
		assert.Nil(t, minimalModel.SSLKey)
		assert.Nil(t, minimalModel.SSLRootCert)
	})
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}