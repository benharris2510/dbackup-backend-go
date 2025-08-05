package services

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/dbackup/backend-go/internal/encryption"
	"github.com/dbackup/backend-go/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	
	// Auto-migrate models for testing
	err = db.AutoMigrate(&models.DatabaseConnection{}, &models.DatabaseTable{})
	require.NoError(t, err)
	
	return db
}

func TestNewDatabaseService(t *testing.T) {
	db := setupTestDB(t)
	encService := encryption.NewService("test-key-for-testing")
	
	service := NewDatabaseService(db, encService)
	
	assert.NotNil(t, service)
	assert.Equal(t, db, service.db)
	assert.Equal(t, encService, service.encryptionService)
}

func TestDatabaseService_TestConnection_UnsupportedType(t *testing.T) {
	db := setupTestDB(t)
	encService := encryption.NewService("test-key-for-testing")
	service := NewDatabaseService(db, encService)
	
	conn := &models.DatabaseConnection{
		Type: "unsupported",
	}
	
	result, err := service.TestConnection(context.Background(), conn)
	
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "Unsupported database type")
	assert.Equal(t, "Database type not supported", result.Message)
	assert.Greater(t, result.ResponseTime, time.Duration(0))
}

func TestDatabaseService_TestConnection_DecryptionError(t *testing.T) {
	db := setupTestDB(t)
	encService := encryption.NewService("test-key-for-testing")
	service := NewDatabaseService(db, encService)
	
	conn := &models.DatabaseConnection{
		Type:     models.DatabaseTypePostgreSQL,
		Password: "invalid-encrypted-data",
	}
	
	result, err := service.TestConnection(context.Background(), conn)
	
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "Failed to decrypt credentials")
	assert.Equal(t, "Invalid connection credentials", result.Message)
}

func TestDatabaseService_BuildPostgreSQLConnectionString(t *testing.T) {
	db := setupTestDB(t)
	encService := encryption.NewService("test-key-for-testing")
	service := NewDatabaseService(db, encService)
	
	tests := []struct {
		name     string
		conn     *models.DatabaseConnection
		password string
		expected string
	}{
		{
			name: "basic connection",
			conn: &models.DatabaseConnection{
				Host:              "localhost",
				Port:              5432,
				Username:          "testuser",
				Database:          "testdb",
				SSLEnabled:        false,
				ConnectionTimeout: 30 * time.Second,
			},
			password: "testpass",
			expected: "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=disable connect_timeout=30",
		},
		{
			name: "SSL enabled with mode",
			conn: &models.DatabaseConnection{
				Host:              "localhost",
				Port:              5432,
				Username:          "testuser",
				Database:          "testdb",
				SSLEnabled:        true,
				SSLMode:           stringPtr("require"),
				ConnectionTimeout: 30 * time.Second,
			},
			password: "testpass",
			expected: "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=require connect_timeout=30",
		},
		{
			name: "SSL enabled without mode",
			conn: &models.DatabaseConnection{
				Host:              "localhost",
				Port:              5432,
				Username:          "testuser",
				Database:          "testdb",
				SSLEnabled:        true,
				ConnectionTimeout: 30 * time.Second,
			},
			password: "testpass",
			expected: "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=require connect_timeout=30",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.buildPostgreSQLConnectionString(tt.conn, tt.password)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDatabaseService_GetPostgreSQLInfo(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	
	gormDB := setupTestDB(t)
	encService := encryption.NewService("test-key-for-testing")
	service := NewDatabaseService(gormDB, encService)
	
	// Mock version query
	mock.ExpectQuery(regexp.QuoteMeta("SELECT version()")).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).
			AddRow("PostgreSQL 14.2 on x86_64-linux-gnu"))
	
	// Mock database size query
	mock.ExpectQuery(regexp.QuoteMeta("SELECT pg_database_size($1)")).
		WithArgs("testdb").
		WillReturnRows(sqlmock.NewRows([]string{"pg_database_size"}).
			AddRow(1048576)) // 1MB
	
	// Mock encoding query
	mock.ExpectQuery(regexp.QuoteMeta("SELECT pg_encoding_to_char(encoding) FROM pg_database WHERE datname = $1")).
		WithArgs("testdb").
		WillReturnRows(sqlmock.NewRows([]string{"pg_encoding_to_char"}).
			AddRow("UTF8"))
	
	// Mock collation query
	mock.ExpectQuery(regexp.QuoteMeta("SELECT datcollate FROM pg_database WHERE datname = $1")).
		WithArgs("testdb").
		WillReturnRows(sqlmock.NewRows([]string{"datcollate"}).
			AddRow("en_US.UTF-8"))
	
	info, err := service.getPostgreSQLInfo(context.Background(), db, "testdb")
	
	require.NoError(t, err)
	assert.Equal(t, "14.2", info.Version)
	assert.Equal(t, "1.0 MB", info.Size)
	assert.Equal(t, "UTF8", info.Charset)
	assert.Equal(t, "en_US.UTF-8", info.Collation)
	
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDatabaseService_GetPostgreSQLInfo_VersionError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	
	gormDB := setupTestDB(t)
	encService := encryption.NewService("test-key-for-testing")
	service := NewDatabaseService(gormDB, encService)
	
	// Mock version query error
	mock.ExpectQuery(regexp.QuoteMeta("SELECT version()")).
		WillReturnError(sql.ErrConnDone)
	
	info, err := service.getPostgreSQLInfo(context.Background(), db, "testdb")
	
	assert.Error(t, err)
	assert.Nil(t, info)
	assert.Contains(t, err.Error(), "failed to get version")
	
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDatabaseService_DiscoverTables_UnsupportedType(t *testing.T) {
	db := setupTestDB(t)
	encService := encryption.NewService("test-key-for-testing")
	service := NewDatabaseService(db, encService)
	
	conn := &models.DatabaseConnection{
		Type: "unsupported",
	}
	
	tables, err := service.DiscoverTables(context.Background(), conn, &models.TableDiscoveryRequest{})
	
	assert.Error(t, err)
	assert.Nil(t, tables)
	assert.Contains(t, err.Error(), "unsupported database type")
}

func TestDatabaseService_DiscoverTables_DecryptionError(t *testing.T) {
	db := setupTestDB(t)
	encService := encryption.NewService("test-key-for-testing")
	service := NewDatabaseService(db, encService)
	
	conn := &models.DatabaseConnection{
		Type:     models.DatabaseTypePostgreSQL,
		Password: "invalid-encrypted-data",
	}
	
	tables, err := service.DiscoverTables(context.Background(), conn, &models.TableDiscoveryRequest{})
	
	assert.Error(t, err)
	assert.Nil(t, tables)
	assert.Contains(t, err.Error(), "failed to decrypt credentials")
}

func TestDatabaseService_BuildPostgreSQLTablesQuery(t *testing.T) {
	db := setupTestDB(t)
	encService := encryption.NewService("test-key-for-testing")
	service := NewDatabaseService(db, encService)
	
	baseQuery := `
		SELECT 
			t.table_name,
			t.table_schema,
			CASE 
				WHEN t.table_type = 'BASE TABLE' THEN 'table'
				WHEN t.table_type = 'VIEW' THEN 'view'
				ELSE LOWER(t.table_type)
			END as table_type,
			obj_description(c.oid) as comment,
			s.n_tup_ins + s.n_tup_upd + s.n_tup_del as row_count,
			pg_total_relation_size(c.oid) as data_length,
			pg_indexes_size(c.oid) as index_length
		FROM information_schema.tables t
		LEFT JOIN pg_class c ON c.relname = t.table_name
		LEFT JOIN pg_namespace n ON n.oid = c.relnamespace AND n.nspname = t.table_schema
		LEFT JOIN pg_stat_user_tables s ON s.relname = t.table_name AND s.schemaname = t.table_schema
		WHERE t.table_catalog = current_database()`
	
	tests := []struct {
		name     string
		req      *models.TableDiscoveryRequest
		expected string
	}{
		{
			name: "default query",
			req:  &models.TableDiscoveryRequest{},
			expected: baseQuery + " AND t.table_schema NOT IN ('information_schema', 'pg_catalog', 'pg_toast', 'pg_temp_1') AND t.table_type = 'BASE TABLE' ORDER BY t.table_schema, t.table_name",
		},
		{
			name: "include views",
			req:  &models.TableDiscoveryRequest{IncludeViews: true},
			expected: baseQuery + " AND t.table_schema NOT IN ('information_schema', 'pg_catalog', 'pg_toast', 'pg_temp_1') ORDER BY t.table_schema, t.table_name",
		},
		{
			name: "include system",
			req:  &models.TableDiscoveryRequest{IncludeSystem: true},
			expected: baseQuery + " AND t.table_type = 'BASE TABLE' ORDER BY t.table_schema, t.table_name",
		},
		{
			name: "schema pattern",
			req:  &models.TableDiscoveryRequest{SchemaPattern: stringPtr("public")},
			expected: baseQuery + " AND t.table_schema LIKE $1 AND t.table_type = 'BASE TABLE' ORDER BY t.table_schema, t.table_name",
		},
		{
			name: "table pattern",
			req:  &models.TableDiscoveryRequest{TablePattern: stringPtr("user%")},
			expected: baseQuery + " AND t.table_schema NOT IN ('information_schema', 'pg_catalog', 'pg_toast', 'pg_temp_1') AND t.table_name LIKE $1 AND t.table_type = 'BASE TABLE' ORDER BY t.table_schema, t.table_name",
		},
		{
			name: "schema and table pattern",
			req: &models.TableDiscoveryRequest{
				SchemaPattern: stringPtr("public"),
				TablePattern:  stringPtr("user%"),
			},
			expected: baseQuery + " AND t.table_schema LIKE $1 AND t.table_name LIKE $2 AND t.table_type = 'BASE TABLE' ORDER BY t.table_schema, t.table_name",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.buildPostgreSQLTablesQuery(tt.req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDatabaseService_BuildPostgreSQLTablesArgs(t *testing.T) {
	db := setupTestDB(t)
	encService := encryption.NewService("test-key-for-testing")
	service := NewDatabaseService(db, encService)
	
	tests := []struct {
		name     string
		req      *models.TableDiscoveryRequest
		expected []interface{}
	}{
		{
			name:     "no patterns",
			req:      &models.TableDiscoveryRequest{},
			expected: []interface{}{},
		},
		{
			name:     "schema pattern only",
			req:      &models.TableDiscoveryRequest{SchemaPattern: stringPtr("public")},
			expected: []interface{}{"public"},
		},
		{
			name:     "table pattern only",
			req:      &models.TableDiscoveryRequest{TablePattern: stringPtr("user%")},
			expected: []interface{}{"user%"},
		},
		{
			name: "both patterns",
			req: &models.TableDiscoveryRequest{
				SchemaPattern: stringPtr("public"),
				TablePattern:  stringPtr("user%"),
			},
			expected: []interface{}{"public", "user%"},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.buildPostgreSQLTablesArgs(tt.req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDatabaseService_UpdateConnectionTestResult(t *testing.T) {
	db := setupTestDB(t)
	encService := encryption.NewService("test-key-for-testing")
	service := NewDatabaseService(db, encService)
	
	// Create a test connection
	conn := &models.DatabaseConnection{
		Name:     "Test Connection",
		Type:     models.DatabaseTypePostgreSQL,
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		Username: "testuser",
		Password: "testpass",
	}
	err := db.Create(conn).Error
	require.NoError(t, err)
	
	t.Run("successful test result", func(t *testing.T) {
		result := &models.TestConnectionResult{
			Success: true,
			Message: "Connection successful",
		}
		
		err := service.UpdateConnectionTestResult(context.Background(), conn.ID, result)
		require.NoError(t, err)
		
		// Verify the update
		var updated models.DatabaseConnection
		err = db.First(&updated, conn.ID).Error
		require.NoError(t, err)
		
		assert.NotNil(t, updated.LastTestedAt)
		assert.Nil(t, updated.LastTestError)
	})
	
	t.Run("failed test result", func(t *testing.T) {
		result := &models.TestConnectionResult{
			Success: false,
			Message: "Connection failed",
			Error:   "timeout",
		}
		
		err := service.UpdateConnectionTestResult(context.Background(), conn.ID, result)
		require.NoError(t, err)
		
		// Verify the update
		var updated models.DatabaseConnection
		err = db.First(&updated, conn.ID).Error
		require.NoError(t, err)
		
		assert.NotNil(t, updated.LastTestedAt)
		assert.NotNil(t, updated.LastTestError)
		assert.Equal(t, "Connection failed: timeout", *updated.LastTestError)
	})
}

func TestDatabaseService_SaveDiscoveredTables(t *testing.T) {
	db := setupTestDB(t)
	encService := encryption.NewService("test-key-for-testing")
	service := NewDatabaseService(db, encService)
	
	// Create a test connection
	conn := &models.DatabaseConnection{
		Name:     "Test Connection",
		Type:     models.DatabaseTypePostgreSQL,
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		Username: "testuser",
		Password: "testpass",
	}
	err := db.Create(conn).Error
	require.NoError(t, err)
	
	t.Run("save new tables", func(t *testing.T) {
		tables := []models.DatabaseTable{
			{
				Name:                 "users",
				Schema:               "public",
				Type:                 models.TableTypeTable,
				DatabaseConnectionID: conn.ID,
				LastDiscoveredAt:     time.Now(),
			},
			{
				Name:                 "orders",
				Schema:               "public",
				Type:                 models.TableTypeTable,
				DatabaseConnectionID: conn.ID,
				LastDiscoveredAt:     time.Now(),
			},
		}
		
		err := service.SaveDiscoveredTables(context.Background(), tables)
		require.NoError(t, err)
		
		// Verify tables were created
		var count int64
		err = db.Model(&models.DatabaseTable{}).
			Where("database_connection_id = ?", conn.ID).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})
	
	t.Run("update existing tables", func(t *testing.T) {
		// Update the existing users table
		rowCount := int64(100)
		tables := []models.DatabaseTable{
			{
				Name:                 "users",
				Schema:               "public",
				Type:                 models.TableTypeTable,
				DatabaseConnectionID: conn.ID,
				RowCount:             &rowCount,
				LastDiscoveredAt:     time.Now(),
			},
		}
		
		err := service.SaveDiscoveredTables(context.Background(), tables)
		require.NoError(t, err)
		
		// Verify table was updated
		var updated models.DatabaseTable
		err = db.Where("database_connection_id = ? AND schema = ? AND name = ?", 
			conn.ID, "public", "users").First(&updated).Error
		require.NoError(t, err)
		assert.NotNil(t, updated.RowCount)
		assert.Equal(t, int64(100), *updated.RowCount)
	})
	
	t.Run("empty tables slice", func(t *testing.T) {
		err := service.SaveDiscoveredTables(context.Background(), []models.DatabaseTable{})
		assert.NoError(t, err)
	})
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDatabaseService_BuildMySQLConnectionString(t *testing.T) {
	db := setupTestDB(t)
	encService := encryption.NewService("test-key-for-testing")
	service := NewDatabaseService(db, encService)
	
	tests := []struct {
		name     string
		conn     *models.DatabaseConnection
		password string
		expected string
	}{
		{
			name: "basic connection",
			conn: &models.DatabaseConnection{
				Host:              "localhost",
				Port:              3306,
				Username:          "testuser",
				Database:          "testdb",
				SSLEnabled:        false,
				ConnectionTimeout: 30 * time.Second,
			},
			password: "testpass",
			expected: "testuser:testpass@tcp(localhost:3306)/testdb?tls=false&timeout=30s&parseTime=true&charset=utf8mb4",
		},
		{
			name: "SSL enabled",
			conn: &models.DatabaseConnection{
				Host:              "localhost",
				Port:              3306,
				Username:          "testuser",
				Database:          "testdb",
				SSLEnabled:        true,
				ConnectionTimeout: 30 * time.Second,
			},
			password: "testpass",
			expected: "testuser:testpass@tcp(localhost:3306)/testdb?tls=true&timeout=30s&parseTime=true&charset=utf8mb4",
		},
		{
			name: "no timeout",
			conn: &models.DatabaseConnection{
				Host:       "localhost",
				Port:       3306,
				Username:   "testuser",
				Database:   "testdb",
				SSLEnabled: false,
			},
			password: "testpass",
			expected: "testuser:testpass@tcp(localhost:3306)/testdb?tls=false&parseTime=true&charset=utf8mb4",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.buildMySQLConnectionString(tt.conn, tt.password)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDatabaseService_GetMySQLInfo(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	
	gormDB := setupTestDB(t)
	encService := encryption.NewService("test-key-for-testing")
	service := NewDatabaseService(gormDB, encService)
	
	// Mock version query
	mock.ExpectQuery(regexp.QuoteMeta("SELECT VERSION()")).
		WillReturnRows(sqlmock.NewRows([]string{"VERSION()"}).
			AddRow("8.0.28-0ubuntu0.20.04.3"))
	
	// Mock database size query
	mock.ExpectQuery(regexp.QuoteMeta("SELECT ROUND(SUM(data_length + index_length)) FROM information_schema.tables WHERE table_schema = ?")).
		WithArgs("testdb").
		WillReturnRows(sqlmock.NewRows([]string{"ROUND(SUM(data_length + index_length))"}).
			AddRow(2097152)) // 2MB
	
	// Mock charset query
	mock.ExpectQuery(regexp.QuoteMeta("SELECT default_character_set_name FROM information_schema.schemata WHERE schema_name = ?")).
		WithArgs("testdb").
		WillReturnRows(sqlmock.NewRows([]string{"default_character_set_name"}).
			AddRow("utf8mb4"))
	
	// Mock collation query
	mock.ExpectQuery(regexp.QuoteMeta("SELECT default_collation_name FROM information_schema.schemata WHERE schema_name = ?")).
		WithArgs("testdb").
		WillReturnRows(sqlmock.NewRows([]string{"default_collation_name"}).
			AddRow("utf8mb4_0900_ai_ci"))
	
	info, err := service.getMySQLInfo(context.Background(), db, "testdb")
	
	require.NoError(t, err)
	assert.Equal(t, "8.0.28", info.Version)
	assert.Equal(t, "2.0 MB", info.Size)
	assert.Equal(t, "utf8mb4", info.Charset)
	assert.Equal(t, "utf8mb4_0900_ai_ci", info.Collation)
	
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDatabaseService_GetMySQLInfo_VersionError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	
	gormDB := setupTestDB(t)
	encService := encryption.NewService("test-key-for-testing")
	service := NewDatabaseService(gormDB, encService)
	
	// Mock version query error
	mock.ExpectQuery(regexp.QuoteMeta("SELECT VERSION()")).
		WillReturnError(sql.ErrConnDone)
	
	info, err := service.getMySQLInfo(context.Background(), db, "testdb")
	
	assert.Error(t, err)
	assert.Nil(t, info)
	assert.Contains(t, err.Error(), "failed to get version")
	
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDatabaseService_BuildMySQLTablesQuery(t *testing.T) {
	db := setupTestDB(t)
	encService := encryption.NewService("test-key-for-testing")
	service := NewDatabaseService(db, encService)
	
	baseQuery := `
		SELECT 
			t.table_name,
			t.table_schema,
			CASE 
				WHEN t.table_type = 'BASE TABLE' THEN 'table'
				WHEN t.table_type = 'VIEW' THEN 'view'
				ELSE LOWER(t.table_type)
			END as table_type,
			t.table_comment,
			t.engine,
			t.table_collation,
			t.table_rows,
			t.data_length,
			t.index_length
		FROM information_schema.tables t
		WHERE t.table_schema = ?`
	
	tests := []struct {
		name     string
		req      *models.TableDiscoveryRequest
		expected string
	}{
		{
			name: "default query",
			req:  &models.TableDiscoveryRequest{},
			expected: baseQuery + " AND t.table_type = 'BASE TABLE' AND t.table_schema NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys') ORDER BY t.table_schema, t.table_name",
		},
		{
			name: "include views",
			req:  &models.TableDiscoveryRequest{IncludeViews: true},
			expected: baseQuery + " AND t.table_schema NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys') ORDER BY t.table_schema, t.table_name",
		},
		{
			name: "include system",
			req:  &models.TableDiscoveryRequest{IncludeSystem: true},
			expected: baseQuery + " AND t.table_type = 'BASE TABLE' ORDER BY t.table_schema, t.table_name",
		},
		{
			name: "schema pattern",
			req:  &models.TableDiscoveryRequest{SchemaPattern: stringPtr("test%")},
			expected: baseQuery + " AND t.table_schema LIKE ? AND t.table_type = 'BASE TABLE' AND t.table_schema NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys') ORDER BY t.table_schema, t.table_name",
		},
		{
			name: "table pattern",
			req:  &models.TableDiscoveryRequest{TablePattern: stringPtr("user%")},
			expected: baseQuery + " AND t.table_name LIKE ? AND t.table_type = 'BASE TABLE' AND t.table_schema NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys') ORDER BY t.table_schema, t.table_name",
		},
		{
			name: "schema and table pattern",
			req: &models.TableDiscoveryRequest{
				SchemaPattern: stringPtr("test%"),
				TablePattern:  stringPtr("user%"),
			},
			expected: baseQuery + " AND t.table_schema LIKE ? AND t.table_name LIKE ? AND t.table_type = 'BASE TABLE' AND t.table_schema NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys') ORDER BY t.table_schema, t.table_name",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.buildMySQLTablesQuery(tt.req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDatabaseService_BuildMySQLTablesArgs(t *testing.T) {
	db := setupTestDB(t)
	encService := encryption.NewService("test-key-for-testing")
	service := NewDatabaseService(db, encService)
	
	tests := []struct {
		name     string
		dbName   string
		req      *models.TableDiscoveryRequest
		expected []interface{}
	}{
		{
			name:     "no patterns",
			dbName:   "testdb",
			req:      &models.TableDiscoveryRequest{},
			expected: []interface{}{"testdb"},
		},
		{
			name:     "schema pattern only",
			dbName:   "testdb",
			req:      &models.TableDiscoveryRequest{SchemaPattern: stringPtr("test%")},
			expected: []interface{}{"testdb", "test%"},
		},
		{
			name:     "table pattern only",
			dbName:   "testdb",
			req:      &models.TableDiscoveryRequest{TablePattern: stringPtr("user%")},
			expected: []interface{}{"testdb", "user%"},
		},
		{
			name:   "both patterns",
			dbName: "testdb",
			req: &models.TableDiscoveryRequest{
				SchemaPattern: stringPtr("test%"),
				TablePattern:  stringPtr("user%"),
			},
			expected: []interface{}{"testdb", "test%", "user%"},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.buildMySQLTablesArgs(tt.dbName, tt.req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDatabaseService_DiscoverTables_MySQL(t *testing.T) {
	db := setupTestDB(t)
	encService := encryption.NewService("test-key-for-testing")
	service := NewDatabaseService(db, encService)
	
	// Create encrypted password
	password := "testpass"
	encryptedPassword, err := encService.Encrypt(password)
	require.NoError(t, err)
	
	conn := &models.DatabaseConnection{
		Type:              models.DatabaseTypeMySQL,
		Host:              "localhost",
		Port:              3306,
		Database:          "testdb",
		Username:          "testuser",
		Password:          encryptedPassword,
		ConnectionTimeout: 30 * time.Second,
		QueryTimeout:      5 * time.Minute,
	}
	
	tables, err := service.DiscoverTables(context.Background(), conn, &models.TableDiscoveryRequest{})
	
	// This will fail because we don't have a real MySQL connection, but it tests the flow
	assert.Error(t, err)
	assert.Nil(t, tables)
	assert.Contains(t, err.Error(), "connection test failed")
}

func TestDatabaseService_TestConnection_MySQL(t *testing.T) {
	db := setupTestDB(t)
	encService := encryption.NewService("test-key-for-testing")
	service := NewDatabaseService(db, encService)
	
	// Create encrypted password
	password := "testpass"
	encryptedPassword, err := encService.Encrypt(password)
	require.NoError(t, err)
	
	conn := &models.DatabaseConnection{
		Type:              models.DatabaseTypeMySQL,
		Host:              "localhost",
		Port:              3306,
		Database:          "testdb",
		Username:          "testuser",
		Password:          encryptedPassword,
		ConnectionTimeout: 30 * time.Second,
		QueryTimeout:      5 * time.Minute,
	}
	
	result, err := service.TestConnection(context.Background(), conn)
	
	// This will fail because we don't have a real MySQL connection, but it tests the flow
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "Connection test failed")
	assert.Equal(t, "Connection failed", result.Message)
	assert.Greater(t, result.ResponseTime, time.Duration(0))
}

// Helper function for tests
func stringPtr(s string) *string {
	return &s
}